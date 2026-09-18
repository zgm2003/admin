package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Service struct {
	repository *Repository
	runtime    RuntimeCoordinator
}

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) {
	s.runtime = runtime
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]Safe, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, wrapRepository(err)
	}
	result := make([]Safe, 0, len(values))
	for _, value := range values {
		safe, safeErr := safeOf(value)
		if safeErr != nil {
			return nil, apperror.DependencyUnavailable(safeErr)
		}
		result = append(result, safe)
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	_, found := FindFixed(input.Scene)
	if !found {
		return apperror.InvalidRequest(fmt.Errorf("template scene is invalid"))
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Subject) == "" || strings.TrimSpace(input.Content) == "" {
		return apperror.InvalidRequest(fmt.Errorf("template name and subject are required"))
	}
	if input.TencentTemplateID != nil && *input.TencentTemplateID < 1 {
		return apperror.InvalidRequest(fmt.Errorf("template id is invalid"))
	}
	if err := validateTemplate(input.VariableKeys, input.ExampleVariables, input.Content, true); err != nil {
		return apperror.InvalidRequest(err)
	}
	current, err := s.repository.Find(ctx, id)
	if err != nil {
		return wrapRepository(err)
	}
	if current.Scene != input.Scene {
		return apperror.InvalidRequest(fmt.Errorf("template scene cannot be changed"))
	}
	if current.IsEnabled == yesno.Yes && input.TencentTemplateID == nil {
		return apperror.InvalidRequest(fmt.Errorf("enabled template requires a provider template id"))
	}
	variableKeys, err := json.Marshal(input.VariableKeys)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	examples, err := json.Marshal(input.ExampleVariables)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	err = s.runtime.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.Update(writeContext, id, input.Scene, map[string]any{
			"name": strings.TrimSpace(input.Name), "subject": strings.TrimSpace(input.Subject), "content": strings.TrimSpace(input.Content),
			"tencent_template_id": input.TencentTemplateID, "variable_keys": variableKeys, "example_variables": examples,
		}, expected, time.Now().UTC())
	})
	err = wrapRepository(err)
	return err
}

func (s *Service) SetStatus(ctx context.Context, id int64, enabled yesno.Value) error {
	if !yesno.IsValid(enabled) {
		return apperror.InvalidRequest(fmt.Errorf("status invalid"))
	}
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	current, err := s.repository.Find(ctx, id)
	if err != nil {
		return wrapRepository(err)
	}
	if enabled == yesno.Yes {
		if current.TencentTemplateID == nil || *current.TencentTemplateID < 1 {
			return apperror.InvalidRequest(fmt.Errorf("enabled template requires a provider template id"))
		}
		if err := validateTemplateFromModel(current); err != nil {
			return apperror.InvalidRequest(err)
		}
	}
	err = s.runtime.Mutate(ctx, func(writeContext context.Context, expected int64) (cachegeneration.MutationResult, error) {
		return s.repository.UpdateStatus(writeContext, id, enabled, expected, time.Now().UTC())
	})
	if mapped := mapMutationError(err); mapped != nil {
		return mapped
	}
	return nil
}

func validateVariables(values map[string]string, requireValues bool) error {
	for _, key := range []string{"code", "ttl_minutes"} {
		value, ok := values[key]
		if !ok || (requireValues && strings.TrimSpace(value) == "") {
			return fmt.Errorf("mail variable %s is required", key)
		}
	}
	ttl, err := strconv.Atoi(strings.TrimSpace(values["ttl_minutes"]))
	if err != nil || ttl < 1 || ttl > 60 {
		return fmt.Errorf("mail variable ttl_minutes is invalid")
	}
	return nil
}

func ValidateVariables(values map[string]string, requireValues bool) error {
	return validateVariables(values, requireValues)
}

var variableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func validateTemplate(keys []string, examples map[string]string, content string, requireValues bool) error {
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if !variableNamePattern.MatchString(key) || seen[key] {
			return fmt.Errorf("mail template variable keys are invalid")
		}
		seen[key] = true
		if !strings.Contains(content, "{{"+key+"}}") {
			return fmt.Errorf("mail template variable %s is missing from content", key)
		}
	}
	for _, required := range []string{"code", "ttl_minutes"} {
		if !seen[required] {
			return fmt.Errorf("mail template variable %s is required", required)
		}
	}
	if len(examples) != len(keys) {
		return fmt.Errorf("mail template example variables must match variable keys")
	}
	for key := range seen {
		if value, ok := examples[key]; !ok || (requireValues && strings.TrimSpace(value) == "") {
			return fmt.Errorf("mail template example variable %s is invalid", key)
		}
	}
	if ttl, err := strconv.Atoi(strings.TrimSpace(examples["ttl_minutes"])); err != nil || ttl < 1 || ttl > 60 {
		return fmt.Errorf("mail variable ttl_minutes is invalid")
	}
	for _, tag := range []string{"script", "iframe", "object", "embed", "form"} {
		if strings.Contains(strings.ToLower(content), "<"+tag) {
			return fmt.Errorf("mail template contains forbidden HTML")
		}
	}
	if strings.Contains(strings.ToLower(content), "javascript:") || regexp.MustCompile(`(?i)\bon[a-z]+\s*=`).MatchString(content) {
		return fmt.Errorf("mail template contains forbidden HTML")
	}
	return nil
}

func validateTemplateFromModel(value Model) error {
	var keys []string
	var examples map[string]string
	if err := json.Unmarshal(value.VariableKeys, &keys); err != nil {
		return err
	}
	if err := json.Unmarshal(value.ExampleVariables, &examples); err != nil {
		return err
	}
	return validateTemplate(keys, examples, value.Content, true)
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
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	return apperror.DependencyUnavailable(fmt.Errorf("mail template repository: %w", err))
}
