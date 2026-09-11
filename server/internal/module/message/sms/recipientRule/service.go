package recipientRule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Service struct {
	repository repository
	keys       *secretkey.KeyRing
	runtime    RuntimeCoordinator
}

func NewService(repository repository, keys *secretkey.KeyRing) *Service {
	return &Service{repository: repository, keys: keys}
}

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) {
	s.runtime = runtime
}

// mutate runs one write and invalidates the runtime snapshot after success when
// a coordinator is wired, mirroring the template and config services.
func (s *Service) mutate(ctx context.Context, change func(context.Context) error) error {
	if s.runtime == nil {
		return change(ctx)
	}
	if err := s.runtime.Mutate(ctx, change); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]Safe, error) {
	rows, err := s.repository.List(ctx)
	if err != nil {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
	}
	safes := make([]Safe, 0, len(rows))
	for _, row := range rows {
		safes = append(safes, safeOf(row))
	}
	return safes, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Safe, error) {
	if s.keys == nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms keys are unavailable"))
	}
	if !validAction(input.Action) || !yesno.IsValid(input.IsEnabled) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule input is invalid"))
	}
	name, remark, err := validateMeta(input.Name, input.Remark)
	if err != nil {
		return Safe{}, err
	}
	pattern, err := normalizePattern(input.Scope, input.Pattern)
	if err != nil {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule pattern is invalid"))
	}
	ciphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), pattern)
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("encrypt sms recipient rule pattern: %w", err))
	}

	now := time.Now().UTC()
	value := Model{
		Scope:             input.Scope,
		PatternCiphertext: ciphertext,
		PatternHint:       patternHint(input.Scope, pattern),
		PatternHMAC:       patternHMAC(s.keys, pattern),
		Action:            input.Action,
		Name:              name,
		Remark:            remark,
		IsEnabled:         input.IsEnabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Create(writeContext, &value)
	}); err != nil {
		return Safe{}, s.writeError(err)
	}
	return safeOf(value), nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (Safe, error) {
	current, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Safe{}, apperror.NotFound(err)
	}
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
	}
	if s.keys == nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms keys are unavailable"))
	}
	if !validAction(input.Action) || !yesno.IsValid(input.IsEnabled) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule input is invalid"))
	}
	name, remark, err := validateMeta(input.Name, input.Remark)
	if err != nil {
		return Safe{}, err
	}

	if input.Pattern == nil {
		if scope := strings.TrimSpace(input.Scope); scope != "" && scope != current.Scope {
			return Safe{}, apperror.InvalidRequest(fmt.Errorf("changing the sms recipient rule scope requires a new pattern"))
		}
	} else {
		scope := strings.TrimSpace(input.Scope)
		pattern, err := normalizePattern(scope, *input.Pattern)
		if err != nil {
			return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms recipient rule pattern is invalid"))
		}
		ciphertext, _, err := secretkey.EncryptSMSValue(s.keys.SMSEncryptionKey(), pattern)
		if err != nil {
			return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("encrypt sms recipient rule pattern: %w", err))
		}
		current.Scope = scope
		current.PatternCiphertext = ciphertext
		current.PatternHint = patternHint(scope, pattern)
		current.PatternHMAC = patternHMAC(s.keys, pattern)
	}

	current.Action = input.Action
	current.Name = name
	current.Remark = remark
	current.IsEnabled = input.IsEnabled

	now := time.Now().UTC()
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, &current, now)
	}); err != nil {
		return Safe{}, s.writeError(err)
	}
	current.UpdatedAt = now
	return safeOf(current), nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	if _, err := s.repository.FindByID(ctx, id); errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.UpdateStatus(writeContext, id, int16(status), time.Now().UTC())
	}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.repository.FindByID(ctx, id); errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	} else if err != nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Delete(writeContext, id)
	}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

// Evaluate resolves the allow/deny decision for one normalized phone number.
// Business and administrator test sends share the same rules.
func (s *Service) Evaluate(ctx context.Context, toPhone string) (Decision, error) {
	rows, err := s.repository.List(ctx)
	if err != nil {
		return Decision{}, apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
	}
	if s.keys == nil {
		return Decision{}, apperror.DependencyUnavailable(fmt.Errorf("sms keys are unavailable"))
	}
	patterns := make([]RulePattern, 0, len(rows))
	for _, row := range rows {
		if row.IsEnabled != yesno.Yes {
			continue
		}
		pattern, err := secretkey.DecryptSMSValue(s.keys.SMSEncryptionKey(), row.PatternCiphertext)
		if err != nil {
			return Decision{}, apperror.DependencyUnavailable(fmt.Errorf("decrypt sms recipient rule pattern: %w", err))
		}
		patterns = append(patterns, RulePattern{ID: row.ID, Scope: row.Scope, Action: row.Action, Pattern: pattern})
	}
	return Match(toPhone, patterns), nil
}

func (s *Service) writeError(err error) error {
	if errors.Is(err, ErrConflict) {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	return apperror.DependencyUnavailable(fmt.Errorf("sms recipient rule repository: %w", err))
}

func validateMeta(name, remark string) (string, string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" || utf8.RuneCountInString(trimmedName) > maxNameLength {
		return "", "", apperror.InvalidRequest(fmt.Errorf("sms recipient rule name is invalid"))
	}
	trimmedRemark := strings.TrimSpace(remark)
	if utf8.RuneCountInString(trimmedRemark) > maxRemarkLength {
		return "", "", apperror.InvalidRequest(fmt.Errorf("sms recipient rule remark is too long"))
	}
	return trimmedName, trimmedRemark, nil
}
