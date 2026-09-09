package cosconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"admin/server/internal/storage/cos"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
	keys       *secretkey.KeyRing
	tester     cos.ConnectionTester
}

func NewService(repository *Repository, keys *secretkey.KeyRing, tester cos.ConnectionTester) *Service {
	return &Service{repository: repository, keys: keys, tester: tester}
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return pagination.Result[SafeValue]{}, invalid(fmt.Errorf("invalid pagination"))
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	total, err := s.repository.Count(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, dependency(err)
	}
	rows, err := s.repository.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, dependency(err)
	}
	values := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		values = append(values, safeValue(row))
	}
	return pagination.Result[SafeValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if id < 1 {
		return SafeValue{}, invalid(fmt.Errorf("id is invalid"))
	}
	model, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SafeValue{}, notFound(err)
	}
	if err != nil {
		return SafeValue{}, dependency(err)
	}
	return safeValue(model), nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (int64, error) {
	if err := validateCreate(input); err != nil {
		return 0, invalid(err)
	}
	key, err := s.storageEncryptionKey()
	if err != nil {
		return 0, err
	}
	secretIDCiphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretID))
	if err != nil {
		return 0, dependency(err)
	}
	secretKeyCiphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretKey))
	if err != nil {
		return 0, dependency(err)
	}
	now := time.Now().UTC()
	model := &Model{
		Name:                strings.TrimSpace(input.Name),
		AppID:               strings.TrimSpace(input.AppID),
		SecretIDCiphertext:  secretIDCiphertext,
		SecretKeyCiphertext: secretKeyCiphertext,
		Bucket:              strings.TrimSpace(input.Bucket),
		Region:              strings.TrimSpace(input.Region),
		Endpoint:            normalizedPointer(input.Endpoint),
		BucketDomain:        normalizedPointer(input.BucketDomain),
		IsEnabled:           input.IsEnabled,
		Remark:              strings.TrimSpace(input.Remark),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.repository.Create(ctx, model); err != nil {
		if errors.Is(err, ErrNameConflict) {
			return 0, conflict(err)
		}
		return 0, dependency(err)
	}
	return model.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if err := validateUpdate(input); err != nil {
		return invalid(err)
	}
	return s.transaction(ctx, func(repository *Repository) error {
		if _, err := repository.LockByID(ctx, id); err != nil {
			return mapReadError(err)
		}
		values := map[string]any{
			"name":          strings.TrimSpace(input.Name),
			"app_id":        strings.TrimSpace(input.AppID),
			"bucket":        strings.TrimSpace(input.Bucket),
			"region":        strings.TrimSpace(input.Region),
			"endpoint":      normalizedPointer(input.Endpoint),
			"bucket_domain": normalizedPointer(input.BucketDomain),
			"remark":        strings.TrimSpace(input.Remark),
			"updated_at":    time.Now().UTC(),
		}
		if input.SecretID.Present || input.SecretKey.Present {
			key, err := s.storageEncryptionKey()
			if err != nil {
				return err
			}
			if input.SecretID.Present {
				ciphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretID.Value))
				if err != nil {
					return dependency(err)
				}
				values["secret_id_ciphertext"] = ciphertext
			}
			if input.SecretKey.Present {
				ciphertext, err := encryptCredential(key, strings.TrimSpace(input.SecretKey.Value))
				if err != nil {
					return dependency(err)
				}
				values["secret_key_ciphertext"] = ciphertext
			}
		}
		if err := repository.Update(ctx, id, values); err != nil {
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				return notFound(err)
			case errors.Is(err, ErrNameConflict):
				return conflict(err)
			default:
				return dependency(err)
			}
		}
		return nil
	})
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	if !yesno.IsValid(value) {
		return invalid(fmt.Errorf("isEnabled is invalid"))
	}
	return s.transaction(ctx, func(repository *Repository) error {
		model, err := repository.LockByID(ctx, id)
		if err != nil {
			return mapReadError(err)
		}
		if value == yesno.No {
			count, err := repository.CountEnabledRules(ctx, model.ID)
			if err != nil {
				return dependency(err)
			}
			if count > 0 {
				return conflict(fmt.Errorf("config has enabled rules"))
			}
		}
		if err := repository.Update(ctx, id, map[string]any{"is_enabled": value, "updated_at": time.Now().UTC()}); err != nil {
			return mapWriteError(err)
		}
		return nil
	})
}

func (s *Service) TestConnection(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	model, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return mapReadError(err)
	}
	if model.IsEnabled != yesno.Yes {
		return conflict(fmt.Errorf("COS config is disabled"))
	}
	if s.tester == nil {
		return dependency(fmt.Errorf("COS connection tester unavailable"))
	}
	key, err := s.storageEncryptionKey()
	if err != nil {
		return err
	}
	secretID, err := decryptCredential(key, model.SecretIDCiphertext)
	if err != nil {
		return dependency(err)
	}
	secretKey, err := decryptCredential(key, model.SecretKeyCiphertext)
	if err != nil {
		return dependency(err)
	}
	endpoint := ""
	if model.Endpoint != nil {
		endpoint = *model.Endpoint
	}
	if err := s.tester.TestConnection(ctx, cos.Credentials{AppID: model.AppID, SecretID: secretID, SecretKey: secretKey, Bucket: model.Bucket, Region: model.Region, Endpoint: endpoint}); err != nil {
		return dependency(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id is invalid"))
	}
	return s.transaction(ctx, func(repository *Repository) error {
		model, err := repository.LockByID(ctx, id)
		if err != nil {
			return mapReadError(err)
		}
		count, err := repository.CountEnabledRules(ctx, model.ID)
		if err != nil {
			return dependency(err)
		}
		if count > 0 {
			return conflict(fmt.Errorf("config has enabled rules"))
		}
		if err := repository.MarkDeleted(ctx, id, time.Now().UTC()); err != nil {
			return mapWriteError(err)
		}
		return nil
	})
}

func (s *Service) storageEncryptionKey() ([]byte, error) {
	if s.keys == nil {
		return nil, dependency(fmt.Errorf("storage encryption key unavailable"))
	}
	return s.keys.StorageEncryptionKey(), nil
}

func (s *Service) transaction(ctx context.Context, fn func(*Repository) error) error {
	err := s.repository.Transaction(ctx, fn)
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return dependency(err)
}

func mapReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	return dependency(err)
}

func mapWriteError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound(err)
	}
	return dependency(err)
}

func normalizedPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	return &trimmed
}
