package template

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
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
func (s *Service) mutate(ctx context.Context, change func(context.Context, int64) (cachegeneration.MutationResult, error)) error {
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("sms runtime coordinator is unavailable"))
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
	content := strings.TrimSpace(input.Content)
	templateID := strings.TrimSpace(input.TencentTemplateID)
	if !validText(name, maxTemplateNameLength) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template name is invalid"))
	}
	if !validText(content, maxContentLength) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template content is invalid"))
	}
	if templateID != "" && !numericTemplatePattern.MatchString(templateID) {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("sms template id must be numeric"))
	}
	if err := validateTemplate(input.VariableKeys, input.ExampleVariables, content); err != nil {
		return Safe{}, apperror.InvalidRequest(err)
	}
	if current.IsEnabled == yesno.Yes && templateID == "" {
		return Safe{}, apperror.InvalidRequest(fmt.Errorf("an enabled sms template requires a provider template id"))
	}

	current.Name = name
	current.Content = content
	current.TencentTemplateID = templateID
	current.VariableKeys = jsonOf(input.VariableKeys)
	current.ExampleVariables = jsonOf(input.ExampleVariables)

	now := time.Now().UTC()
	if err := s.mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Update(writeContext, &current, expected, now)
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
		if err := validateTemplateFromModel(current, variables); err != nil {
			return apperror.InvalidRequest(fmt.Errorf("an enabled sms template requires every example variable"))
		}
	}
	now := time.Now().UTC()
	if err := s.mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateStatus(writeContext, id, int16(status), expected, now)
	}); err != nil {
		return apperror.DependencyUnavailable(err)
	}
	return nil
}

func validText(value string, maxRunes int) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxRunes
}

var variableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
var placeholderPattern = regexp.MustCompile(`{([0-9]+)}`)

func validateTemplate(keys []string, examples map[string]string, content string) error {
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if !variableNamePattern.MatchString(key) || seen[key] {
			return fmt.Errorf("sms template variable keys are invalid")
		}
		seen[key] = true
	}
	for _, required := range []string{"code", "ttl_minutes"} {
		if !seen[required] {
			return fmt.Errorf("sms template variable %s is required", required)
		}
	}
	if len(examples) != len(keys) {
		return fmt.Errorf("sms template example variables must match variable keys")
	}
	for key := range seen {
		value, ok := examples[key]
		if !ok || !validText(strings.TrimSpace(value), maxVariableLength) {
			return fmt.Errorf("sms template example variable %s is invalid", key)
		}
	}
	ttl, err := strconv.Atoi(strings.TrimSpace(examples["ttl_minutes"]))
	if err != nil || ttl < 1 || ttl > 60 {
		return fmt.Errorf("sms variable ttl_minutes is invalid")
	}
	positions := make(map[int]bool)
	for _, match := range placeholderPattern.FindAllStringSubmatch(content, -1) {
		position, err := strconv.Atoi(match[1])
		if err != nil || position < 1 || position > len(keys) {
			return fmt.Errorf("sms template placeholder is invalid")
		}
		positions[position] = true
	}
	if remainder := placeholderPattern.ReplaceAllString(content, ""); strings.ContainsAny(remainder, "{}") {
		return fmt.Errorf("sms template placeholder is invalid")
	}
	for position := 1; position <= len(keys); position++ {
		if !positions[position] {
			return fmt.Errorf("sms template placeholder {%d} is missing", position)
		}
	}
	return nil
}

func validateTemplateFromModel(current Model, examples map[string]string) error {
	var keys []string
	if err := decodeJSON(current.VariableKeys, &keys); err != nil {
		return err
	}
	return validateTemplate(keys, examples, current.Content)
}
