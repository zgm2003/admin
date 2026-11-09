package provider

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
)

type repository interface {
	Create(context.Context, Model) (int64, error)
	FindByID(context.Context, int64) (Model, error)
	List(context.Context, ListQuery) ([]Model, int64, error)
	Update(context.Context, int64, UpdateValues) error
	UpdateStatus(context.Context, int64, yesno.Value, time.Time) error
	Delete(context.Context, int64) error
}

type Service struct {
	repo repository
	keys *secretkey.KeyRing
	now  func() time.Time
}

func NewService(repo repository, keys *secretkey.KeyRing) *Service {
	return &Service{repo: repo, keys: keys, now: time.Now}
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if s == nil || s.repo == nil {
		return pagination.Result[SafeValue]{}, dependency(fmt.Errorf("provider repository missing"))
	}
	rows, total, err := s.repo.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	items := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		item, err := safeValue(ctx, row)
		if err != nil {
			return pagination.Result[SafeValue]{}, err
		}
		items = append(items, item)
	}
	return pagination.Result[SafeValue]{List: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if s == nil || s.repo == nil {
		return SafeValue{}, dependency(fmt.Errorf("provider repository missing"))
	}
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	return safeValue(ctx, row)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if s == nil || s.repo == nil || s.keys == nil {
		return 0, dependency(fmt.Errorf("provider service is not configured"))
	}
	name, err := validateName(input.Name)
	if err != nil {
		return 0, invalid(fmt.Errorf("name: %w", err))
	}
	if !input.Vendor.Valid() || !input.Protocol.Valid() || !yesno.IsValid(input.Status) {
		return 0, invalid(fmt.Errorf("vendor, protocol or status is invalid"))
	}
	baseURL, err := validateBaseURL(input.BaseURL)
	if err != nil {
		return 0, invalid(fmt.Errorf("baseUrl: %w", err))
	}
	apiKey, err := validateAPIKey(input.APIKey, true)
	if err != nil {
		return 0, invalid(fmt.Errorf("apiKey: %w", err))
	}
	if input.TimeoutMS < 1000 || input.TimeoutMS > maxTimeoutMS ||
		input.SortOrder < 0 || input.SortOrder > maxSortOrder {
		return 0, invalid(fmt.Errorf("timeoutMs or sortOrder is invalid"))
	}
	ciphertext, encryptionKeyID, err := secretkey.EncryptProviderValue(s.keys.ProviderEncryptionKey(), apiKey)
	if err != nil {
		return 0, dependency(fmt.Errorf("encrypt provider API key: %w", err))
	}
	now := s.now().UTC()
	id, err := s.repo.Create(ctx, Model{
		Name: name, Vendor: input.Vendor, Protocol: input.Protocol,
		BaseURL: baseURL, APIKeyCiphertext: ciphertext, EncryptionKeyID: encryptionKeyID,
		TimeoutMS: input.TimeoutMS, SortOrder: input.SortOrder, Status: input.Status,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return 0, mapError(err)
	}
	return id, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if s == nil || s.repo == nil || s.keys == nil {
		return dependency(fmt.Errorf("provider service is not configured"))
	}
	name, err := validateName(input.Name)
	if err != nil {
		return invalid(fmt.Errorf("name: %w", err))
	}
	if !input.Vendor.Valid() || !input.Protocol.Valid() || !yesno.IsValid(input.Status) {
		return invalid(fmt.Errorf("vendor, protocol or status is invalid"))
	}
	baseURL, err := validateBaseURL(input.BaseURL)
	if err != nil {
		return invalid(fmt.Errorf("baseUrl: %w", err))
	}
	if input.TimeoutMS < 1000 || input.TimeoutMS > maxTimeoutMS ||
		input.SortOrder < 0 || input.SortOrder > maxSortOrder {
		return invalid(fmt.Errorf("timeoutMs or sortOrder is invalid"))
	}
	var ciphertext, encryptionKeyID *string
	if input.APIKey != nil {
		apiKey, err := validateAPIKey(*input.APIKey, true)
		if err != nil {
			return invalid(fmt.Errorf("apiKey: %w", err))
		}
		value, keyID, err := secretkey.EncryptProviderValue(s.keys.ProviderEncryptionKey(), apiKey)
		if err != nil {
			return dependency(fmt.Errorf("encrypt provider API key: %w", err))
		}
		ciphertext, encryptionKeyID = &value, &keyID
	}
	if err = s.repo.Update(ctx, id, UpdateValues{
		Name: name, Vendor: input.Vendor, Protocol: input.Protocol, BaseURL: baseURL,
		APIKeyCiphertext: ciphertext, EncryptionKeyID: encryptionKeyID,
		TimeoutMS: input.TimeoutMS, SortOrder: input.SortOrder, Status: input.Status,
		UpdatedAt: s.now().UTC(),
	}); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, input StatusInput) error {
	if s == nil || s.repo == nil {
		return dependency(fmt.Errorf("provider repository missing"))
	}
	if !yesno.IsValid(input.Status) {
		return invalid(fmt.Errorf("status is invalid"))
	}
	if err := s.repo.UpdateStatus(ctx, id, input.Status, s.now().UTC()); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return dependency(fmt.Errorf("provider repository missing"))
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return mapError(err)
	}
	return nil
}
