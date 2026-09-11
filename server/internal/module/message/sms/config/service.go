package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

const (
	maxSDKAppIDLength = 64
	maxSignNameLength = 128
	maxRegionLength   = 64
	maxEndpointLength = 255
	maxSecretLength   = 256
	minTTLMinutes     = 1
	maxTTLMinutes     = 60
)

type Service struct {
	repository repository
	keys       *secretkey.KeyRing
	runtime    RuntimeCoordinator
}

func NewService(repository repository, keys *secretkey.KeyRing, runtime RuntimeCoordinator) *Service {
	return &Service{repository: repository, keys: keys, runtime: runtime}
}

func (s *Service) Load(ctx context.Context) (Safe, error) {
	value, err := s.repository.FindActive(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Safe{}, nil
	}
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms config repository: %w", err))
	}
	return safeOf(value), nil
}

func (s *Service) Update(ctx context.Context, input Input) (Safe, error) {
	normalized, err := s.normalize(input)
	if err != nil {
		return Safe{}, err
	}

	current, findErr := s.repository.FindActive(ctx)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms config repository: %w", findErr))
	}
	existing := findErr == nil

	if !existing && (normalized.SecretID == "" || normalized.SecretKey == "") {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms credentials are required"))
	}
	if existing && (normalized.SecretID == "") != (normalized.SecretKey == "") {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms credentials must be replaced together"))
	}
	if s.keys == nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms encryption key is unavailable"))
	}

	model := Model{
		ID:            current.ID,
		SDKAppID:      normalized.SDKAppID,
		SignName:      normalized.SignName,
		Region:        normalized.Region,
		TTLMinutes:    int16(normalized.TTLMinutes),
		IsEnabled:     normalized.IsEnabled,
		LastTestAt:    current.LastTestAt,
		LastTestError: current.LastTestError,
	}
	if normalized.Endpoint != "" {
		endpoint := normalized.Endpoint
		model.Endpoint = &endpoint
	}
	if existing {
		model.SecretIDCiphertext = current.SecretIDCiphertext
		model.SecretKeyCiphertext = current.SecretKeyCiphertext
		model.SecretIDHint = current.SecretIDHint
		model.SecretKeyHint = current.SecretKeyHint
	}
	if normalized.SecretID != "" {
		secretIDCiphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), normalized.SecretID)
		if err != nil {
			return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("encrypt sms secret id: %w", err))
		}
		secretKeyCiphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), normalized.SecretKey)
		if err != nil {
			return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("encrypt sms secret key: %w", err))
		}
		model.SecretIDCiphertext = secretIDCiphertext
		model.SecretKeyCiphertext = secretKeyCiphertext
		model.SecretIDHint = hint(normalized.SecretID)
		model.SecretKeyHint = hint(normalized.SecretKey)
	}

	now := time.Now().UTC()
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		if !existing {
			model.CreatedAt = now
			model.UpdatedAt = now
			return s.repository.Create(writeContext, &model)
		}
		return s.repository.Update(writeContext, &model, now)
	}); err != nil {
		return Safe{}, err
	}
	model.UpdatedAt = now
	return safeOf(model), nil
}

func (s *Service) Delete(ctx context.Context) error {
	current, err := s.repository.FindActive(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms config repository: %w", err))
	}
	return s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Delete(writeContext, current.ID)
	})
}

// Credentials decrypts the stored credentials for the sending path. It is never
// reachable from an HTTP response.
func (s *Service) Credentials(ctx context.Context) (Credentials, error) {
	current, err := s.repository.FindActive(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Credentials{}, apperror.NotFound(err)
	}
	if err != nil {
		return Credentials{}, apperror.DependencyUnavailable(fmt.Errorf("sms config repository: %w", err))
	}
	if s.keys == nil {
		return Credentials{}, apperror.DependencyUnavailable(fmt.Errorf("sms encryption key is unavailable"))
	}
	secretID, err := secretkey.DecryptSMSValue(s.keys.SMSEncryptionKey(), current.SecretIDCiphertext)
	if err != nil {
		return Credentials{}, apperror.DependencyUnavailable(fmt.Errorf("decrypt sms secret id: %w", err))
	}
	secretKey, err := secretkey.DecryptSMSValue(s.keys.SMSEncryptionKey(), current.SecretKeyCiphertext)
	if err != nil {
		return Credentials{}, apperror.DependencyUnavailable(fmt.Errorf("decrypt sms secret key: %w", err))
	}
	credentials := Credentials{
		SecretID: secretID, SecretKey: secretKey, SDKAppID: current.SDKAppID,
		SignName: current.SignName, Region: current.Region,
		TTLMinutes: int(current.TTLMinutes),
	}
	if current.Endpoint != nil {
		credentials.Endpoint = *current.Endpoint
	}
	return credentials, nil
}

// MarkTestResult records the outcome of an administrator test send.
func (s *Service) MarkTestResult(ctx context.Context, id int64, at time.Time, message string) error {
	if len(message) > 512 {
		message = message[:512]
	}
	if err := s.repository.UpdateTestResult(ctx, id, at, message); err != nil {
		return apperror.DependencyUnavailable(fmt.Errorf("update sms config test result: %w", err))
	}
	return nil
}

func (s *Service) mutate(ctx context.Context, change func(context.Context) error) error {
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms runtime coordinator is unavailable"))
	}
	if err := s.runtime.Mutate(ctx, change); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) normalize(input Input) (Input, error) {
	normalized := Input{
		SecretID:   strings.TrimSpace(input.SecretID),
		SecretKey:  strings.TrimSpace(input.SecretKey),
		SDKAppID:   strings.TrimSpace(input.SDKAppID),
		SignName:   strings.TrimSpace(input.SignName),
		Region:     strings.TrimSpace(input.Region),
		Endpoint:   strings.TrimSpace(input.Endpoint),
		TTLMinutes: input.TTLMinutes,
		IsEnabled:  input.IsEnabled,
	}
	if !utf8.ValidString(normalized.SecretID) || !utf8.ValidString(normalized.SecretKey) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms credentials are invalid"))
	}
	if utf8.RuneCountInString(normalized.SDKAppID) == 0 || utf8.RuneCountInString(normalized.SDKAppID) > maxSDKAppIDLength {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms app id is invalid"))
	}
	if utf8.RuneCountInString(normalized.SignName) == 0 || utf8.RuneCountInString(normalized.SignName) > maxSignNameLength {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms signature name is invalid"))
	}
	if !regionPattern.MatchString(normalized.Region) || utf8.RuneCountInString(normalized.Region) > maxRegionLength {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms region is invalid"))
	}
	if normalized.Endpoint != "" && (!endpointPattern.MatchString(normalized.Endpoint) || utf8.RuneCountInString(normalized.Endpoint) > maxEndpointLength) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms endpoint is invalid"))
	}
	if utf8.RuneCountInString(normalized.SecretID) > maxSecretLength || utf8.RuneCountInString(normalized.SecretKey) > maxSecretLength {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms credentials are too long"))
	}
	if normalized.TTLMinutes < minTTLMinutes || normalized.TTLMinutes > maxTTLMinutes {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms ttl minutes is invalid"))
	}
	if !yesno.IsValid(normalized.IsEnabled) {
		return Input{}, apperror.InvalidRequest(fmt.Errorf("sms isEnabled must be 0 or 1"))
	}
	return normalized, nil
}

func safeOf(value Model) Safe {
	safe := Safe{
		Configured:    true,
		SDKAppID:      value.SDKAppID,
		SignName:      value.SignName,
		Region:        value.Region,
		TTLMinutes:    int(value.TTLMinutes),
		IsEnabled:     value.IsEnabled,
		LastTestError: value.LastTestError,
	}
	if value.Endpoint != nil {
		safe.Endpoint = *value.Endpoint
	}
	if value.LastTestAt != nil {
		formatted := value.LastTestAt.UTC().Format(time.RFC3339Nano)
		safe.LastTestAt = &formatted
	}
	return safe
}

func hint(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
