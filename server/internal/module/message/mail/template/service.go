package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Service struct {
	repository         *Repository
	readiness          ReadinessCoordinator
	runtimeInvalidator func(context.Context) error
}

func (s *Service) SetRuntimeInvalidator(invalidator func(context.Context) error) {
	s.runtimeInvalidator = invalidator
}

func NewService(repository *Repository, readiness ReadinessCoordinator) *Service {
	return &Service{repository: repository, readiness: readiness}
}

func (s *Service) List(ctx context.Context) ([]Model, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, wrapRepository(err)
	}
	return values, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) error {
	_, found := FindFixed(input.Scene)
	if !found || input.TencentTemplateID < 1 {
		return apperror.InvalidRequest(fmt.Errorf("template scene or id is invalid"))
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Subject) == "" {
		return apperror.InvalidRequest(fmt.Errorf("template name and subject are required"))
	}
	if err := validateVariables(input.Variables, true); err != nil {
		return apperror.InvalidRequest(err)
	}
	if err := validateVariables(input.ExampleVariables, true); err != nil {
		return apperror.InvalidRequest(err)
	}
	current, err := s.repository.Find(ctx, id)
	if err != nil {
		return wrapRepository(err)
	}
	if current.Scene != input.Scene {
		return apperror.InvalidRequest(fmt.Errorf("template scene cannot be changed"))
	}
	variables, err := json.Marshal(input.Variables)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	examples, err := json.Marshal(input.ExampleVariables)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	err = wrapRepository(s.repository.Update(ctx, id, map[string]any{
		"name": input.Name, "subject": input.Subject, "tencent_template_id": input.TencentTemplateID,
		"variables": variables, "example_variables": examples, "updated_at": time.Now().UTC(),
	}))
	if err != nil || s.runtimeInvalidator == nil {
		return err
	}
	return s.runtimeInvalidator(ctx)
}

func (s *Service) SetStatus(ctx context.Context, id int64, enabled yesno.Value) error {
	if !yesno.IsValid(enabled) {
		return apperror.InvalidRequest(fmt.Errorf("status invalid"))
	}
	if s.readiness == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail readiness coordinator unavailable"))
	}
	err := s.readiness.Mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, id, map[string]any{
			"is_enabled": enabled, "updated_at": time.Now().UTC(),
		})
	})
	if mapped := mapMutationError(err); mapped != nil {
		return mapped
	}
	if s.runtimeInvalidator != nil {
		return s.runtimeInvalidator(ctx)
	}
	return nil
}

func validateVariables(values map[string]string, requireValues bool) error {
	if len(values) != 2 {
		return fmt.Errorf("mail variables must contain code and ttl_minutes")
	}
	for _, key := range []string{"code", "ttl_minutes"} {
		value, found := values[key]
		if !found || (requireValues && strings.TrimSpace(value) == "") {
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
