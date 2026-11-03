package tool

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/configCache"
	"admin/server/internal/shared/i18n"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (configcache.MutationResult, error)) error
}
type Service struct {
	repository *Repository
	runtime    RuntimeCoordinator
	cipher     CredentialCipher
}

func NewService(r *Repository, runtime ...RuntimeCoordinator) *Service {
	var c RuntimeCoordinator
	if len(runtime) > 0 {
		c = runtime[0]
	}
	return &Service{repository: r, runtime: c}
}

type CredentialCipher interface {
	Seal(string) (string, string, error)
	Open(string, string) (string, error)
}

func (s *Service) SetCipher(c CredentialCipher) { s.cipher = c }
func (s *Service) seal(in VersionInput) (VersionInput, error) {
	if len(in.SecretHeaders) == 0 {
		return in, nil
	}
	if s.cipher == nil {
		return in, errors.New("tool encryption unavailable")
	}
	b, e := json.Marshal(in.SecretHeaders)
	if e != nil {
		return in, e
	}
	in.SecretCiphertext, in.EncryptionKeyID, e = s.cipher.Seal(string(b))
	return in, e
}
func (s *Service) SetRuntimeCoordinator(c RuntimeCoordinator) { s.runtime = c }
func (s *Service) mutate(ctx context.Context, fn func(context.Context, *Repository) error) error {
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, expected int64) (configcache.MutationResult, error) {
		return s.repository.Mutate(ctx, expected, func(r *Repository) error { return fn(ctx, r) })
	})
}

func mapped(err error) error {
	if err == nil {
		return nil
	}
	var app *apperror.Error
	if errors.As(err, &app) {
		return app
	}
	if errors.Is(err, ErrReferenced) {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	return apperror.DependencyUnavailable(err)
}
func (s *Service) Create(ctx context.Context, in ToolInput) (int64, error) {
	if err := in.Validate(); err != nil {
		return 0, apperror.InvalidRequest(err)
	}
	if in.ActorID < 1 {
		return 0, apperror.InvalidRequest(errors.New("actor required"))
	}
	var sealErr error
	in.Version, sealErr = s.seal(in.Version)
	if sealErr != nil {
		return 0, mapped(sealErr)
	}
	var id int64
	err := s.mutate(ctx, func(ctx context.Context, r *Repository) error { var e error; id, e = r.Create(ctx, in); return e })
	return id, mapped(err)
}
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if id < 1 || in.ActorID < 1 {
		return apperror.InvalidRequest(errors.New("id and actor required"))
	}
	if err := in.Validate(); err != nil {
		return apperror.InvalidRequest(err)
	}
	var e error
	in.Version, e = s.seal(in.Version)
	if e != nil {
		return mapped(e)
	}
	return mapped(s.mutate(ctx, func(ctx context.Context, r *Repository) error { return r.Update(ctx, id, in) }))
}
func (s *Service) CurrentVersion(ctx context.Context, id int64) (Version, error) {
	v, err := s.repository.CurrentVersion(ctx, id)
	return v, mapped(err)
}
func (s *Service) Version(ctx context.Context, id int64) (Version, error) {
	v, err := s.repository.Version(ctx, id)
	return v, mapped(err)
}
func (s *Service) Find(ctx context.Context, id int64) (Model, error) {
	v, err := s.repository.Find(ctx, id)
	return v, mapped(err)
}
func (s *Service) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, apperror.InvalidRequest(errors.New("pagination invalid"))
	}
	rows, total, err := s.repository.List(ctx, page, size)
	return rows, total, mapped(err)
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, status Status, actor int64) error {
	if id < 1 || actor < 1 || (status != StatusDisabled && status != StatusEnabled) {
		return apperror.InvalidRequest(errors.New("status invalid"))
	}
	return mapped(s.mutate(ctx, func(ctx context.Context, r *Repository) error { return r.UpdateStatus(ctx, id, status, actor) }))
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return apperror.InvalidRequest(errors.New("id invalid"))
	}
	return mapped(s.mutate(ctx, func(ctx context.Context, r *Repository) error { return r.Delete(ctx, id) }))
}
