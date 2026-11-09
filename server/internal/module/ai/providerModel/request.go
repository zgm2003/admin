package providermodel

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

type ListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	ModelKind pricing.ModelKind
	Status    *yesno.Value
}

type PriceInput struct {
	TierKey        string
	TierLabel      string
	PriceCNYMicros int64
}

type UpdateInput struct {
	DisplayName     string
	ModelKind       pricing.ModelKind
	PricingMode     pricing.PricingMode
	PriceUnit       pricing.PriceUnit
	OfficialModelID *int64
	Status          yesno.Value
	Prices          []PriceInput
}

type updateRequest struct {
	DisplayName     *string              `json:"displayName"`
	ModelKind       *pricing.ModelKind   `json:"modelKind"`
	PricingMode     *pricing.PricingMode `json:"pricingMode"`
	PriceUnit       *pricing.PriceUnit   `json:"priceUnit"`
	OfficialModelID *int64               `json:"officialModelId"`
	Status          *yesno.Value         `json:"status"`
	Prices          []priceRequest       `json:"prices"`
}

type priceRequest struct {
	TierKey   string `json:"tierKey"`
	TierLabel string `json:"tierLabel"`
	PriceCNY  string `json:"priceCny"`
}

func (r updateRequest) input() (UpdateInput, error) {
	if r.DisplayName == nil || r.ModelKind == nil || r.PricingMode == nil ||
		r.PriceUnit == nil || r.Status == nil {
		return UpdateInput{}, fmt.Errorf("displayName, modelKind, pricingMode, priceUnit and status are required")
	}
	displayName := strings.TrimSpace(*r.DisplayName)
	if displayName == "" || utf8.RuneCountInString(displayName) > 200 {
		return UpdateInput{}, fmt.Errorf("displayName must be between 1 and 200 characters")
	}
	if !r.ModelKind.Valid() || !r.PricingMode.Valid() || !r.PriceUnit.Valid() || !yesno.IsValid(*r.Status) {
		return UpdateInput{}, fmt.Errorf("modelKind, pricingMode, priceUnit or status is invalid")
	}
	prices := make([]PriceInput, 0, len(r.Prices))
	for _, item := range r.Prices {
		tierKey := strings.TrimSpace(item.TierKey)
		tierLabel := strings.TrimSpace(item.TierLabel)
		if tierKey == "" || len(tierKey) > 64 || len(tierLabel) > 64 {
			return UpdateInput{}, fmt.Errorf("price tierKey and tierLabel are invalid")
		}
		amount, err := parseCNYMicros(item.PriceCNY)
		if err != nil {
			return UpdateInput{}, fmt.Errorf("priceCny: %w", err)
		}
		prices = append(prices, PriceInput{TierKey: tierKey, TierLabel: tierLabel, PriceCNYMicros: amount})
	}
	return UpdateInput{
		DisplayName: displayName, ModelKind: *r.ModelKind, PricingMode: *r.PricingMode,
		PriceUnit: *r.PriceUnit, OfficialModelID: r.OfficialModelID,
		Status: *r.Status, Prices: prices,
	}, nil
}

func parseListQuery(values url.Values) (ListQuery, error) {
	for key, entries := range values {
		if (key != "page" && key != "pageSize" && key != "keyword" && key != "modelKind" &&
			key != "status") || len(entries) != 1 {
			return ListQuery{}, fmt.Errorf("query invalid")
		}
	}
	request, err := pagination.ParseRequest(values)
	if err != nil {
		return ListQuery{}, err
	}
	query := ListQuery{
		Page: request.Page, PageSize: request.PageSize,
		Keyword: strings.TrimSpace(values.Get("keyword")),
	}
	if utf8.RuneCountInString(query.Keyword) > 200 {
		return ListQuery{}, fmt.Errorf("keyword is too long")
	}
	if raw := strings.TrimSpace(values.Get("modelKind")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return ListQuery{}, fmt.Errorf("modelKind is invalid")
		}
		query.ModelKind = pricing.ModelKind(value)
		if !query.ModelKind.Valid() {
			return ListQuery{}, fmt.Errorf("modelKind is invalid")
		}
	}
	if raw := strings.TrimSpace(values.Get("status")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		status := yesno.Value(value)
		if !yesno.IsValid(status) {
			return ListQuery{}, fmt.Errorf("status is invalid")
		}
		query.Status = &status
	}
	return query, nil
}

func parseCNYMicros(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("price is required")
	}
	if value[0] == '+' {
		value = value[1:]
	}
	if value == "" || value[0] == '-' {
		return 0, fmt.Errorf("price must be non-negative")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("price is invalid")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > 999_999_999_999 {
		return 0, fmt.Errorf("price is too large")
	}
	fraction := int64(0)
	if len(parts) == 2 {
		rawFraction := parts[1]
		if rawFraction == "" || len(rawFraction) > 6 {
			return 0, fmt.Errorf("price supports at most six decimal places")
		}
		for _, character := range rawFraction {
			if character < '0' || character > '9' {
				return 0, fmt.Errorf("price is invalid")
			}
		}
		padded := rawFraction + strings.Repeat("0", 6-len(rawFraction))
		fraction, err = strconv.ParseInt(padded, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("price is invalid")
		}
	}
	return whole*1_000_000 + fraction, nil
}

func formatCNYMicros(value int64) string {
	if value <= 0 {
		return "0.00"
	}
	whole := value / 1_000_000
	fraction := value % 1_000_000
	text := strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%06d", fraction)
	for strings.HasSuffix(text, "0") && len(text)-strings.IndexByte(text, '.')-1 > 2 {
		text = strings.TrimSuffix(text, "0")
	}
	return text
}
