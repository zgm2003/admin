package recipientrule

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"admin/server/internal/shared/apperror"
	sharedemail "admin/server/internal/shared/email"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var domainLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

type Service struct {
	repository *Repository
	runtime    RuntimeCoordinator
}

func NewService(repository *Repository) *Service { return &Service{repository: repository} }

func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) {
	s.runtime = runtime
}

func (s *Service) Evaluate(ctx context.Context, email string, _ SendMode) (Decision, error) {
	email, err := NormalizeRecipient(email)
	if err != nil {
		return Decision{}, err
	}
	rows, err := s.repository.List(ctx)
	if err != nil {
		return Decision{}, err
	}
	return EvaluateRows(rows, email)
}

func EvaluateRows(rows []Model, email string) (Decision, error) {
	var err error
	email, err = NormalizeRecipient(email)
	if err != nil {
		return Decision{}, err
	}
	var exact, domain []Model
	for _, row := range rows {
		if row.IsEnabled != yesno.Yes {
			continue
		}
		if row.Scope == ScopeEmail && row.Pattern == email {
			exact = append(exact, row)
		}
		if row.Scope == ScopeDomain && matchesDomain(email, row.Pattern) {
			domain = append(domain, row)
		}
	}
	sort.Slice(exact, func(i, j int) bool { return exact[i].ID < exact[j].ID })
	sort.Slice(domain, func(i, j int) bool { return domain[i].ID < domain[j].ID })
	return choose(exact, domain), nil
}

func (s *Service) List(ctx context.Context) ([]Model, error) {
	values, err := s.repository.List(ctx)
	if err != nil {
		return nil, wrapRepository(err)
	}
	return values, nil
}

func (s *Service) Create(ctx context.Context, input Input) (int64, error) {
	if err := validateInput(input); err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	pattern, err := NormalizeRule(input.Scope, input.Pattern)
	if err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	now := time.Now().UTC()
	value := &Model{
		Scope: input.Scope, Pattern: pattern, Action: input.Action, Name: input.Name,
		Remark: input.Remark, IsEnabled: input.IsEnabled, CreatedAt: now, UpdatedAt: now,
	}
	if s.runtime == nil {
		return 0, apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	if err := s.runtime.Mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Create(writeContext, value)
	}); err != nil {
		return 0, wrapRepository(err)
	}
	return value.ID, nil
}

func (s *Service) Update(ctx context.Context, id int64, input Input) error {
	if err := validateInput(input); err != nil {
		return apperror.InvalidRequest(err)
	}
	pattern, err := NormalizeRule(input.Scope, input.Pattern)
	if err != nil {
		return apperror.InvalidRequest(err)
	}
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	err = wrapRepository(s.runtime.Mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, id, map[string]any{
			"scope": input.Scope, "pattern": pattern, "action": input.Action, "name": input.Name,
			"remark": input.Remark, "is_enabled": input.IsEnabled, "updated_at": time.Now().UTC(),
		})
	}))
	return err
}

func (s *Service) SetStatus(ctx context.Context, id int64, enabled yesno.Value) error {
	if !yesno.IsValid(enabled) {
		return apperror.InvalidRequest(fmt.Errorf("status invalid"))
	}
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	err := wrapRepository(s.runtime.Mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Update(writeContext, id, map[string]any{
			"is_enabled": enabled, "updated_at": time.Now().UTC(),
		})
	}))
	return err
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if s.runtime == nil {
		return apperror.DependencyUnavailable(fmt.Errorf("mail runtime coordinator unavailable"))
	}
	return wrapRepository(s.runtime.Mutate(ctx, func(writeContext context.Context) error {
		return s.repository.Delete(writeContext, id)
	}))
}

func NormalizeRecipient(value string) (string, error) {
	return sharedemail.Normalize(value)
}

func NormalizeRule(scope, pattern string) (string, error) {
	pattern = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(pattern, ".")))
	if scope == ScopeEmail {
		return NormalizeRecipient(pattern)
	}
	if scope != ScopeDomain || len(pattern) > 253 {
		return "", fmt.Errorf("invalid recipient rule scope")
	}
	labels := strings.Split(pattern, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("invalid domain")
	}
	for _, label := range labels {
		if !domainLabel.MatchString(label) {
			return "", fmt.Errorf("invalid domain")
		}
	}
	return pattern, nil
}

func validateInput(input Input) error {
	if (input.Action != ActionAllow && input.Action != ActionDeny) ||
		!yesno.IsValid(input.IsEnabled) || strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("invalid recipient rule")
	}
	return nil
}

func choose(groups ...[]Model) Decision {
	for _, group := range groups {
		for _, action := range []string{ActionDeny, ActionAllow} {
			for _, rule := range group {
				if rule.Action == action {
					return Decision{Allowed: action == ActionAllow, RuleID: rule.ID, Reason: action}
				}
			}
		}
	}
	return Decision{Allowed: true, Reason: "default_allow"}
}

func matchesDomain(email, pattern string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	host := strings.ToLower(parts[1])
	return host == pattern || strings.HasSuffix(host, "."+pattern)
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
	return apperror.DependencyUnavailable(fmt.Errorf("mail recipient rule repository: %w", err))
}
