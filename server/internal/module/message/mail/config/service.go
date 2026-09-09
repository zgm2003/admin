package config

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Service struct {
	repository         *Repository
	keys               *secretkey.KeyRing
	readiness          ReadinessCoordinator
	runtimeInvalidator func(context.Context) error
}

func (s *Service) SetRuntimeInvalidator(invalidator func(context.Context) error) {
	s.runtimeInvalidator = invalidator
}

func NewService(repository *Repository, keys *secretkey.KeyRing, readiness ReadinessCoordinator) *Service {
	return &Service{repository: repository, keys: keys, readiness: readiness}
}

func (s *Service) Get(ctx context.Context) (Safe, error) {
	value, err := s.repository.Find(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Safe{Configured: false, TTLMinutes: 0, IsEnabled: yesno.No}, nil
	}
	if err != nil {
		return Safe{}, wrapRepository(err)
	}
	return safe(value), nil
}

func (s *Service) Save(ctx context.Context, input Input) (Safe, error) {
	if input.TTLMinutes < 1 || input.TTLMinutes > 60 || !yesno.IsValid(input.IsEnabled) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("invalid mail config"))
	}
	fromEmail, err := normalizeAddress(input.FromEmail)
	if err != nil {
		return Safe{}, apperror.InvalidRequest(err)
	}
	input.FromEmail = fromEmail
	if strings.TrimSpace(input.Region) == "" || strings.TrimSpace(input.FromName) == "" {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("region and fromName are required"))
	}
	if s.keys == nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("mail encryption key unavailable"))
	}
	if strings.TrimSpace(input.SecretID) == "" || strings.TrimSpace(input.SecretKey) == "" {
		current, findErr := s.repository.Find(ctx)
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return Safe{}, apperror.InvalidRequest(fmt.Errorf("credentials are required for the first configuration"))
		}
		if findErr != nil {
			return Safe{}, wrapRepository(findErr)
		}
		if strings.TrimSpace(input.SecretID) == "" {
			input.SecretID, err = secretkey.DecryptMailValue(s.keys.MailEncryptionKey(), current.SecretIDCiphertext)
			if err != nil {
				return Safe{}, apperror.DependencyUnavailable(err)
			}
		}
		if strings.TrimSpace(input.SecretKey) == "" {
			input.SecretKey, err = secretkey.DecryptMailValue(s.keys.MailEncryptionKey(), current.SecretKeyCiphertext)
			if err != nil {
				return Safe{}, apperror.DependencyUnavailable(err)
			}
		}
	}
	if strings.TrimSpace(input.ReplyTo) != "" {
		input.ReplyTo, err = normalizeAddress(input.ReplyTo)
		if err != nil {
			return Safe{}, apperror.InvalidRequest(err)
		}
	}
	secretID, _, err := secretkey.EncryptMailValue(s.keys.MailEncryptionKey(), input.SecretID)
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(err)
	}
	secretKey, _, err := secretkey.EncryptMailValue(s.keys.MailEncryptionKey(), input.SecretKey)
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(err)
	}
	values := map[string]any{
		"secret_id_ciphertext": secretID, "secret_key_ciphertext": secretKey,
		"secret_id_hint": hint(input.SecretID), "secret_key_hint": hint(input.SecretKey),
		"region": strings.TrimSpace(input.Region), "endpoint": nullableText(input.Endpoint),
		"from_email": input.FromEmail, "from_name": strings.TrimSpace(input.FromName),
		"reply_to": nullableText(input.ReplyTo), "ttl_minutes": input.TTLMinutes,
		"is_enabled": input.IsEnabled, "updated_at": time.Now().UTC(),
	}
	var saved Model
	if s.readiness == nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("mail readiness coordinator unavailable"))
	}
	if err := s.readiness.Mutate(ctx, func(writeContext context.Context) error {
		var saveErr error
		saved, saveErr = s.repository.Save(writeContext, values)
		return saveErr
	}); err != nil {
		return Safe{}, mapMutationError(err)
	}
	if s.runtimeInvalidator != nil {
		if err := s.runtimeInvalidator(ctx); err != nil {
			return Safe{}, apperror.DependencyUnavailable(err)
		}
	}
	return safe(saved), nil
}

func (s *Service) Delete(ctx context.Context) error {
	if s.readiness == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail readiness coordinator unavailable"))
	}
	err := s.readiness.Mutate(ctx, s.repository.Delete)
	if mapped := mapMutationError(err); mapped != nil {
		return mapped
	}
	if s.runtimeInvalidator != nil {
		if err := s.runtimeInvalidator(ctx); err != nil {
			return apperror.DependencyUnavailable(err)
		}
	}
	return nil
}

func safe(value Model) Safe {
	return Safe{
		Configured: value.SecretIDCiphertext != "" && value.SecretKeyCiphertext != "",
		Region:     value.Region, Endpoint: stringValue(value.Endpoint), FromEmail: value.FromEmail,
		FromName: value.FromName, ReplyTo: stringValue(value.ReplyTo), TTLMinutes: int(value.TTLMinutes),
		IsEnabled: value.IsEnabled, LastTestAt: value.LastTestAt, LastTestError: value.LastTestError,
	}
}

func normalizeAddress(value string) (string, error) {
	address := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Name != "" || parsed.Address != address || len(address) > 254 {
		return "", fmt.Errorf("email address is invalid")
	}
	return address, nil
}

func nullableText(value string) any {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func hint(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "***" + value[len(value)-2:]
}

func mapMutationError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	return wrapRepository(err)
}

func wrapRepository(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	return apperror.DependencyUnavailable(fmt.Errorf("mail config repository: %w", err))
}
