package providermodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type repository interface {
	List(context.Context, int64, ListQuery) ([]Model, int64, error)
	FindByID(context.Context, int64, int64) (Model, error)
	PricesByModelIDs(context.Context, []int64) (map[int64][]Price, error)
	OfficialByIDs(context.Context, []int64) (map[int64]Official, error)
	OfficialByVendor(context.Context, int16) ([]Official, error)
	Sync(context.Context, int64, []SyncItem, time.Time) (SyncCounts, error)
	UpdateConfig(context.Context, int64, int64, UpdateValues) error
}

type providerSource interface {
	FindByID(context.Context, int64) (aiprovider.Model, error)
}

type Service struct {
	repo           repository
	providerSource providerSource
	keys           *secretkey.KeyRing
	httpClient     *http.Client
	now            func() time.Time
}

func NewService(repo *Repository, providerSource providerSource, keys *secretkey.KeyRing) *Service {
	return &Service{
		repo: repo, providerSource: providerSource, keys: keys,
		httpClient: &http.Client{Timeout: 20 * time.Second}, now: time.Now,
	}
}

func (s *Service) List(ctx context.Context, providerID int64, query ListQuery) (pagination.Result[SafeValue], error) {
	if s == nil || s.repo == nil {
		return pagination.Result[SafeValue]{}, dependency(fmt.Errorf("provider model repository missing"))
	}
	if providerID < 1 {
		return pagination.Result[SafeValue]{}, invalid(fmt.Errorf("provider id is invalid"))
	}
	rows, total, err := s.repo.List(ctx, providerID, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	items, err := s.safeValues(ctx, rows)
	if err != nil {
		return pagination.Result[SafeValue]{}, err
	}
	return pagination.Result[SafeValue]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, providerID, id int64) (SafeValue, error) {
	if s == nil || s.repo == nil {
		return SafeValue{}, dependency(fmt.Errorf("provider model repository missing"))
	}
	if providerID < 1 || id < 1 {
		return SafeValue{}, invalid(fmt.Errorf("provider or model id is invalid"))
	}
	row, err := s.repo.FindByID(ctx, providerID, id)
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	items, err := s.safeValues(ctx, []Model{row})
	if err != nil {
		return SafeValue{}, err
	}
	return items[0], nil
}

func (s *Service) Update(ctx context.Context, providerID, id int64, input UpdateInput) error {
	if s == nil || s.repo == nil || s.providerSource == nil {
		return dependency(fmt.Errorf("provider model service is not configured"))
	}
	if providerID < 1 || id < 1 {
		return invalid(fmt.Errorf("provider or model id is invalid"))
	}
	provider, err := s.providerSource.FindByID(ctx, providerID)
	if err != nil {
		return mapError(err)
	}
	if _, err := s.repo.FindByID(ctx, providerID, id); err != nil {
		return mapError(err)
	}
	if _, err := s.validateConfiguration(ctx, int16(provider.Vendor), input); err != nil {
		return err
	}
	if err := s.validatePrices(input); err != nil {
		return err
	}
	prices := make([]Price, 0, len(input.Prices))
	for _, item := range input.Prices {
		prices = append(prices, Price{
			TierKey: item.TierKey, TierLabel: item.TierLabel, PriceCNYMicros: item.PriceCNYMicros,
		})
	}
	if err := s.repo.UpdateConfig(ctx, providerID, id, UpdateValues{
		DisplayName: input.DisplayName, ModelKind: input.ModelKind, PricingMode: input.PricingMode,
		PriceUnit: input.PriceUnit, OfficialModelID: input.OfficialModelID,
		Status: input.Status, Prices: prices, UpdatedAt: s.now().UTC(),
	}); err != nil {
		return mapError(err)
	}
	return nil
}

type SyncResult struct {
	Total       int   `json:"total"`
	Inserted    int64 `json:"inserted"`
	Updated     int64 `json:"updated"`
	Deactivated int64 `json:"deactivated"`
	Unmatched   int   `json:"unmatched"`
}

type upstreamModel struct {
	ID          string
	DisplayName string
}

func (s *Service) Sync(ctx context.Context, providerID int64) (SyncResult, error) {
	if s == nil || s.repo == nil || s.providerSource == nil || s.keys == nil {
		return SyncResult{}, dependency(fmt.Errorf("provider model sync service is not configured"))
	}
	provider, err := s.providerSource.FindByID(ctx, providerID)
	if err != nil {
		return SyncResult{}, mapError(err)
	}
	key, err := secretkey.DecryptProviderValue(s.keys.ProviderEncryptionKey(), provider.APIKeyCiphertext)
	if err != nil {
		return SyncResult{}, dependency(fmt.Errorf("decrypt provider API key: %w", err))
	}
	upstream, err := s.listUpstreamModels(ctx, provider, key)
	if err != nil {
		return SyncResult{}, err
	}
	officials, err := s.repo.OfficialByVendor(ctx, int16(provider.Vendor))
	if err != nil {
		return SyncResult{}, mapError(err)
	}
	byKey := make(map[string]Official, len(officials))
	for _, official := range officials {
		byKey[official.ModelKey] = official
	}
	seenAt := s.now().UTC()
	items := make([]SyncItem, 0, len(upstream))
	unmatched := 0
	for _, model := range upstream {
		item := SyncItem{
			UpstreamModel: model.ID, ModelKey: model.ID, DisplayName: model.DisplayName,
			ModelKind: pricing.KindChat, PricingMode: pricing.ModeProviderUnit,
			PriceUnit: pricing.UnitRequest, Status: yesno.No,
		}
		if official, ok := byKey[model.ID]; ok {
			id := official.ID
			item.OfficialModelID = &id
			item.ModelKind = official.ModelKind
			item.PricingMode = pricing.ModeOfficialToken
			item.PriceUnit = pricing.UnitMillionTokens
		} else if official, ok := byKey[strings.ToLower(model.ID)]; ok {
			id := official.ID
			item.OfficialModelID = &id
			item.ModelKind = official.ModelKind
			item.PricingMode = pricing.ModeOfficialToken
			item.PriceUnit = pricing.UnitMillionTokens
		} else {
			unmatched++
		}
		items = append(items, item)
	}
	counts, err := s.repo.Sync(ctx, providerID, items, seenAt)
	if err != nil {
		return SyncResult{}, mapError(err)
	}
	return SyncResult{
		Total: len(upstream), Inserted: counts.Inserted, Updated: counts.Updated,
		Deactivated: counts.Deactivated, Unmatched: unmatched,
	}, nil
}

func (s *Service) safeValues(ctx context.Context, rows []Model) ([]SafeValue, error) {
	ids := make([]int64, 0, len(rows))
	officialIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.OfficialModelID != nil {
			officialIDs = append(officialIDs, *row.OfficialModelID)
		}
	}
	prices, err := s.repo.PricesByModelIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	officials, err := s.repo.OfficialByIDs(ctx, officialIDs)
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		var official *Official
		if row.OfficialModelID != nil {
			if value, ok := officials[*row.OfficialModelID]; ok {
				official = &value
			}
		}
		item, err := safeValue(ctx, row, prices[row.ID], official)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) validateConfiguration(ctx context.Context, vendor int16, input UpdateInput) (*Official, error) {
	if !input.ModelKind.Valid() || !input.PricingMode.Valid() || !input.PriceUnit.Valid() || !yesno.IsValid(input.Status) {
		return nil, invalid(fmt.Errorf("modelKind, pricingMode, priceUnit or status is invalid"))
	}
	if input.ModelKind == pricing.KindChat && input.PricingMode != pricing.ModeOfficialToken {
		return nil, invalid(fmt.Errorf("chat models must use official token pricing"))
	}
	if input.PricingMode == pricing.ModeOfficialToken && input.PriceUnit != pricing.UnitMillionTokens {
		return nil, invalid(fmt.Errorf("token pricing must use million tokens"))
	}
	if input.PricingMode == pricing.ModeProviderUnit && input.PriceUnit == pricing.UnitMillionTokens {
		return nil, invalid(fmt.Errorf("provider unit pricing cannot use million tokens"))
	}
	switch input.ModelKind {
	case pricing.KindImage:
		if input.PricingMode == pricing.ModeProviderUnit && input.PriceUnit != pricing.UnitImage {
			return nil, invalid(fmt.Errorf("image models must use image price units"))
		}
	case pricing.KindVideo:
		if input.PricingMode == pricing.ModeProviderUnit && input.PriceUnit != pricing.UnitSecond {
			return nil, invalid(fmt.Errorf("video models must use second price units"))
		}
	case pricing.KindAudio:
		if input.PricingMode == pricing.ModeProviderUnit &&
			input.PriceUnit != pricing.UnitMinute && input.PriceUnit != pricing.UnitMillionCharacters &&
			input.PriceUnit != pricing.UnitRequest {
			return nil, invalid(fmt.Errorf("audio models must use minute, million characters or request price units"))
		}
	}
	var official *Official
	if input.OfficialModelID != nil {
		officials, err := s.repo.OfficialByIDs(ctx, []int64{*input.OfficialModelID})
		if err != nil {
			return nil, mapError(err)
		}
		value, ok := officials[*input.OfficialModelID]
		if !ok {
			return nil, invalid(fmt.Errorf("official model does not exist"))
		}
		if value.Vendor != vendor {
			return nil, invalid(fmt.Errorf("official model vendor does not match provider"))
		}
		if value.ModelKind != input.ModelKind {
			return nil, invalid(fmt.Errorf("official model kind does not match"))
		}
		official = &value
	}
	if input.PricingMode == pricing.ModeOfficialToken && official == nil {
		return nil, invalid(fmt.Errorf("token pricing requires an official model"))
	}
	if input.Status == yesno.Yes && input.PricingMode == pricing.ModeOfficialToken && official == nil {
		return nil, invalid(fmt.Errorf("enabled token model requires an official model"))
	}
	return official, nil
}

func (s *Service) validatePrices(input UpdateInput) error {
	if input.PricingMode == pricing.ModeOfficialToken {
		if len(input.Prices) != 0 {
			return invalid(fmt.Errorf("token pricing does not accept provider price tiers"))
		}
		return nil
	}
	if len(input.Prices) == 0 {
		if input.Status == yesno.Yes {
			return invalid(fmt.Errorf("enabled provider-priced model requires at least one price"))
		}
		return nil
	}
	seen := make(map[string]struct{}, len(input.Prices))
	for _, price := range input.Prices {
		if _, exists := seen[price.TierKey]; exists {
			return invalid(fmt.Errorf("duplicate price tier %q", price.TierKey))
		}
		seen[price.TierKey] = struct{}{}
	}
	return nil
}

func (s *Service) listUpstreamModels(ctx context.Context, provider aiprovider.Model, apiKey string) ([]upstreamModel, error) {
	requestCtx, cancel := context.WithTimeout(ctx, syncTimeout(provider.TimeoutMS))
	defer cancel()
	endpoint := strings.TrimRight(provider.BaseURL, "/") + "/models"
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, dependency(fmt.Errorf("build provider model request: %w", err))
	}
	switch provider.Protocol {
	case aiprovider.ProtocolOpenAI, aiprovider.ProtocolOpenAIResponses:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	case aiprovider.ProtocolAnthropic:
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	case aiprovider.ProtocolGemini:
		request.Header.Set("x-goog-api-key", apiKey)
	default:
		return nil, invalid(fmt.Errorf("unsupported provider protocol for model sync"))
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, dependency(fmt.Errorf("request provider models: %w", err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, dependency(fmt.Errorf("read provider models: %w", err))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, dependency(fmt.Errorf("provider models returned HTTP %d", response.StatusCode))
	}
	return parseUpstreamModels(provider.Protocol, body)
}

func syncTimeout(milliseconds int32) time.Duration {
	if milliseconds < 1000 || milliseconds > 300000 {
		return 20 * time.Second
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func parseUpstreamModels(protocol aiprovider.Protocol, body []byte) ([]upstreamModel, error) {
	var models []upstreamModel
	switch protocol {
	case aiprovider.ProtocolOpenAI, aiprovider.ProtocolOpenAIResponses, aiprovider.ProtocolAnthropic:
		var payload struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, dependency(fmt.Errorf("decode provider models: %w", err))
		}
		for _, item := range payload.Data {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			displayName := strings.TrimSpace(item.DisplayName)
			if displayName == "" {
				displayName = item.ID
			}
			models = append(models, upstreamModel{ID: item.ID, DisplayName: displayName})
		}
	case aiprovider.ProtocolGemini:
		var payload struct {
			Models []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, dependency(fmt.Errorf("decode provider models: %w", err))
		}
		for _, item := range payload.Models {
			name := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
			if name == "" {
				continue
			}
			displayName := strings.TrimSpace(item.DisplayName)
			if displayName == "" {
				displayName = name
			}
			models = append(models, upstreamModel{ID: name, DisplayName: displayName})
		}
	default:
		return nil, invalid(fmt.Errorf("unsupported provider protocol"))
	}
	return dedupeModels(models), nil
}

func dedupeModels(models []upstreamModel) []upstreamModel {
	seen := make(map[string]struct{}, len(models))
	result := make([]upstreamModel, 0, len(models))
	for _, model := range models {
		if _, ok := seen[model.ID]; ok {
			continue
		}
		seen[model.ID] = struct{}{}
		result = append(result, model)
	}
	return result
}

func invalid(err error) error {
	var public *apperror.Error
	if errors.As(err, &public) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return dependency(err)
	}
	return apperror.InvalidRequest(err)
}

func dependency(err error) error { return apperror.DependencyUnavailable(err) }

func mapError(err error) error {
	var public *apperror.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &public):
		return err
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	default:
		return dependency(err)
	}
}
