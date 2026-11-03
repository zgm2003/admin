package agent

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/configCache"
	"admin/server/internal/shared/i18n"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (configcache.MutationResult, error)) error
}
type Service struct {
	repository *Repository
	validator  BindingValidator
	runtime    RuntimeCoordinator
}

func NewService(r *Repository, v BindingValidator, runtime ...RuntimeCoordinator) *Service {
	var c RuntimeCoordinator
	if len(runtime) > 0 {
		c = runtime[0]
	}
	return &Service{repository: r, validator: v, runtime: c}
}
func (s *Service) SetRuntimeCoordinator(c RuntimeCoordinator) { s.runtime = c }
func (s *Service) transaction(ctx context.Context, fn func(context.Context, *Repository) error) error {
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, expected int64) (configcache.MutationResult, error) {
		return s.repository.Mutate(ctx, expected, func(r *Repository) error { return fn(ctx, r) })
	})
}
func serviceErr(e error) error {
	if e == nil {
		return nil
	}
	var app *apperror.Error
	if errors.As(e, &app) {
		return app
	}
	if errors.Is(e, ErrConfiguration) {
		return apperror.InvalidRequest(e)
	}
	if errors.Is(e, ErrReferenced) {
		return apperror.Conflict(i18n.KeyConflict, nil, e)
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return apperror.Conflict(i18n.KeyConflict, nil, e)
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return apperror.NotFound(e)
	}
	return apperror.DependencyUnavailable(e)
}
func (s *Service) requireValidator() error {
	if s.validator == nil {
		return errors.New("AI binding validator unavailable")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (int64, error) {
	if e := in.Validate(); e != nil {
		return 0, apperror.InvalidRequest(e)
	}
	if in.ActorID < 1 {
		return 0, apperror.InvalidRequest(errors.New("actor required"))
	}
	var m Model
	e := s.transaction(ctx, func(ctx context.Context, r *Repository) error {
		var e error
		m, e = r.Create(ctx, in)
		if e != nil {
			return e
		}
		if e = r.ReplaceBindings(ctx, m, in.Bindings); e != nil {
			return e
		}
		if e = r.ReplaceTools(ctx, m.ID, in.Tools); e != nil {
			return e
		}
		if in.ActiveProviderModelID != nil {
			rows, e := r.Bindings(ctx, m.ID)
			if e != nil {
				return e
			}
			for _, b := range rows {
				if b.ProviderModelID == *in.ActiveProviderModelID {
					id := b.ID
					in.ActiveBindingID = &id
					break
				}
			}
			if in.ActiveBindingID == nil {
				return configurationError("initial provider model is not bound")
			}
		}
		if in.ActiveBindingID != nil {
			if e = s.validateLocked(ctx, r, m, *in.ActiveBindingID, in.Capabilities); e != nil {
				return e
			}
			if e = r.SetCurrent(ctx, m.ID, in.ActiveBindingID); e != nil {
				return e
			}
		} else if in.Status == 1 {
			return configurationError("enabled agent requires current binding")
		}
		if in.Status == 1 {
			return r.SetStatus(ctx, m.ID, 1, in.ActorID)
		}
		return nil
	})
	return m.ID, serviceErr(e)
}
func (s *Service) validateLocked(ctx context.Context, r *Repository, m Model, id int64, caps []string) error {
	if e := s.requireValidator(); e != nil {
		return e
	}
	b, e := r.ValidateBinding(ctx, m.ID, id)
	if e != nil {
		return e
	}
	return s.validator.ValidateBinding(ctx, b.ProviderModelID, b.TextProtocol, caps)
}
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if id < 1 || in.ActorID < 1 {
		return apperror.InvalidRequest(errors.New("id and actor required"))
	}
	if e := in.Validate(); e != nil {
		return apperror.InvalidRequest(e)
	}
	e := s.transaction(ctx, func(ctx context.Context, r *Repository) error {
		m, e := r.Lock(ctx, id)
		if e != nil {
			return e
		}
		if e = r.SaveConfiguration(ctx, m, in); e != nil {
			return e
		}
		if e = r.ReplaceBindings(ctx, m, in.Bindings); e != nil {
			return e
		}
		return r.ReplaceTools(ctx, id, in.Tools)
	})
	return serviceErr(e)
}
func (s *Service) SwitchBinding(ctx context.Context, id, bindingID int64) error {
	if id < 1 || bindingID < 1 {
		return apperror.InvalidRequest(errors.New("agent and binding required"))
	}
	e := s.transaction(ctx, func(ctx context.Context, r *Repository) error {
		m, e := r.Lock(ctx, id)
		if e != nil {
			return e
		}
		if e = s.validateLocked(ctx, r, m, bindingID, []string(m.Capabilities)); e != nil {
			return e
		}
		return r.SetCurrent(ctx, id, &bindingID)
	})
	return serviceErr(e)
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, status int16, actor int64) error {
	if id < 1 || actor < 1 || (status != 0 && status != 1) {
		return apperror.InvalidRequest(errors.New("status invalid"))
	}
	e := s.transaction(ctx, func(ctx context.Context, r *Repository) error {
		m, e := r.Lock(ctx, id)
		if e != nil {
			return e
		}
		if status == 1 {
			if m.ActiveBindingID == nil {
				return configurationError("enabled agent requires current binding")
			}
			if e = s.validateLocked(ctx, r, m, *m.ActiveBindingID, []string(m.Capabilities)); e != nil {
				return e
			}
		}
		return r.SetStatus(ctx, id, status, actor)
	})
	return serviceErr(e)
}
func (s *Service) Find(ctx context.Context, id int64) (Model, error) {
	m, e := s.repository.Find(ctx, id)
	return m, serviceErr(e)
}
func (s *Service) Bindings(ctx context.Context, id int64) ([]Binding, error) {
	v, e := s.repository.Bindings(ctx, id)
	return v, serviceErr(e)
}
func (s *Service) Tools(ctx context.Context, id int64) ([]ToolBinding, error) {
	v, e := s.repository.ToolBindings(ctx, id)
	return v, serviceErr(e)
}
func (s *Service) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, apperror.InvalidRequest(fmt.Errorf("pagination invalid"))
	}
	v, n, e := s.repository.List(ctx, page, size)
	return v, n, serviceErr(e)
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return apperror.InvalidRequest(errors.New("id invalid"))
	}
	return serviceErr(s.transaction(ctx, func(ctx context.Context, r *Repository) error {
		m, e := r.Lock(ctx, id)
		if e != nil {
			return e
		}
		return r.Delete(ctx, m)
	}))
}
