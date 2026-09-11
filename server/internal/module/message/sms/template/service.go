package template

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Service struct {
	repository repository
	runtime    RuntimeCoordinator
}

func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) {
	s.runtime = runtime
}

// mutate runs one write and, when a runtime coordinator is wired, invalidates
// the runtime snapshot only after the database change succeeds.
func (s *Service) mutate(ctx context.Context, change func(context.Context) error) error {
	if s.runtime == nil {
		return change(ctx)
	}
	if err := s.runtime.Mutate(ctx, change); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

// List returns the fixed four templates. A missing or duplicated scene means the
// migrated data no longer matches the frozen catalog, so it fails closed.
func (s *Service) List(ctx context.Context) ([]Safe, error) {
	rows, err := s.repository.List(ctx)
	if err != nil {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("sms template repository: %w", err))
	}
	byScene := make(map[string]Model, len(rows))
	for _, row := range rows {
		byScene[row.Scene] = row
	}
	catalog := FixedCatalog()
	if len(byScene) != len(catalog) {
		return nil, apperror.DependencyUnavailable(fmt.Errorf("sms template catalog is incomplete"))
	}
	safes := make([]Safe, 0, len(catalog))
	for _, fixed := range catalog {
		row, found := byScene[fixed.Scene]
		if !found {
			return nil, apperror.DependencyUnavailable(fmt.Errorf("sms template scene %q is missing", fixed.Scene))
		}
		safe, err := safeOf(row)
		if err != nil {
			return nil, apperror.DependencyUnavailable(err)
		}
		safes = append(safes, safe)
	}
	return safes, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (Safe, error) {
	current, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Safe{}, apperror.NotFound(err)
	}
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("sms template repository: %w", err))
	}
	if input.Scene != current.Scene {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template scene cannot be changed"))
	}

	name := strings.TrimSpace(input.Name)
	templateID := strings.TrimSpace(input.TencentTemplateID)
	if !validText(name, maxTemplateNameLength) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template name is invalid"))
	}
	if !validParameterKeys(input.ParameterKeys) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template parameter keys must match the fixed catalog"))
	}
	if templateID != "" && !numericTemplatePattern.MatchString(templateID) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template id must be numeric"))
	}
	if err := validateExampleVariables(input.ExampleVariables); err != nil {
		return Safe{}, err
	}
	if current.IsEnabled == yesno.Yes && templateID == "" {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("an enabled sms template requires a provider template id"))
	}

	current.Name = name
	current.TencentTemplateID = templateID
	current.ParameterKeys = jsonOf(input.ParameterKeys)
	current.ExampleVariables = jsonOf(input.ExampleVariables)

	now := time.Now().UTC()
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, &current, now)
	}); err != nil {
		return Safe{}, apperror.DependencyUnavailable(fmt.Errorf("update sms template: %w", err))
	}
	current.UpdatedAt = now
	safe, err := safeOf(current)
	if err != nil {
		return Safe{}, apperror.DependencyUnavailable(err)
	}
	return safe, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, status yesno.Value) error {
	if !yesno.IsValid(status) {
		return apperror.InvalidRequest(fmt.Errorf("isEnabled must be 0 or 1"))
	}
	current, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	if err != nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms template repository: %w", err))
	}
	if status == yesno.Yes {
		if !numericTemplatePattern.MatchString(strings.TrimSpace(current.TencentTemplateID)) {
			return apperror.InvalidRequest(fmt.Errorf("an enabled sms template requires a numeric provider template id"))
		}
		var variables map[string]string
		if err := decodeJSON(current.ExampleVariables, &variables); err != nil {
			return apperror.DependencyUnavailable(err)
		}
		if err := validateExampleVariables(variables); err != nil {
			return apperror.InvalidRequest(fmt.Errorf("an enabled sms template requires every example variable"))
		}
	}
	if err := s.mutate(ctx, func(writeContext context.Context) error {
		return s.repository.UpdateStatus(writeContext, id, int16(status), time.Now().UTC())
	}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func validText(value string, maxRunes int) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxRunes
}

func validParameterKeys(keys []string) bool {
	fixed := FixedCatalog()[0].ParameterKeys
	if len(keys) != len(fixed) {
		return false
	}
	for index, key := range fixed {
		if keys[index] != key {
			return false
		}
	}
	return true
}

func validateExampleVariables(variables map[string]string) error {
	fixed := FixedCatalog()[0].ParameterKeys
	if len(variables) != len(fixed) {
		return apperror.InvalidRequest(fmt.Errorf("sms template example variables must match the fixed catalog"))
	}
	for _, key := range fixed {
		value, found := variables[key]
		if !found {
			return apperror.InvalidRequest(fmt.Errorf("sms template example variable %q is missing", key))
		}
		if !validText(strings.TrimSpace(value), maxVariableLength) {
			return apperror.InvalidRequest(fmt.Errorf("sms template example variable %q is invalid", key))
		}
	}
	return nil
}
