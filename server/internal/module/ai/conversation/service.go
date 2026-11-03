package conversation

import (
	"admin/server/internal/module/ai/message"
	"admin/server/internal/shared/apperror"
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	repository *Repository
	agents     AgentResolver
	now        func() time.Time
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound(err)
	}
	return apperror.DependencyUnavailable(err)
}
func validOwner(user, platform int64) error {
	if user < 1 || platform < 1 {
		return apperror.Unauthorized(nil)
	}
	return nil
}
func validTitle(title string) bool {
	return strings.TrimSpace(title) != "" && utf8.RuneCountInString(title) <= 200
}
func (s *Service) Create(ctx context.Context, user, platform, agent int64, title string) (Model, error) {
	if e := validOwner(user, platform); e != nil {
		return Model{}, e
	}
	if agent < 1 || !validTitle(title) {
		return Model{}, apperror.InvalidRequest(nil)
	}
	if s.agents == nil {
		return Model{}, apperror.DependencyUnavailable(nil)
	}
	runtime, e := s.agents(ctx, agent, s.now().UTC())
	if e != nil {
		return Model{}, mapError(e)
	}
	runtime, e = PublicAgentRuntime(runtime)
	if e != nil {
		return Model{}, apperror.InvalidRequest(e)
	}
	text := false
	for _, operation := range runtime.Operations {
		if operation == "text.generate" {
			text = true
		}
	}
	if !text {
		return Model{}, apperror.InvalidRequest(nil)
	}
	row, e := s.repository.Create(ctx, user, platform, agent, strings.TrimSpace(title), s.now().UTC())
	return row, mapError(e)
}
func (s *Service) AgentOptions(ctx context.Context, user, platform int64) ([]AgentOption, error) {
	if e := validOwner(user, platform); e != nil {
		return nil, e
	}
	rows, e := s.repository.AgentOptions(ctx)
	return rows, mapError(e)
}
func (s *Service) Find(ctx context.Context, id, user, platform int64) (Model, error) {
	if e := validOwner(user, platform); e != nil {
		return Model{}, e
	}
	if id < 1 {
		return Model{}, apperror.InvalidRequest(nil)
	}
	row, e := s.repository.Find(ctx, id, user, platform)
	return row, mapError(e)
}
func (s *Service) List(ctx context.Context, user, platform, before int64, limit int) ([]Model, error) {
	if e := validOwner(user, platform); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 || before < 0 {
		return nil, apperror.InvalidRequest(nil)
	}
	rows, e := s.repository.List(ctx, user, platform, before, limit)
	return rows, mapError(e)
}
func (s *Service) Update(ctx context.Context, id, user, platform int64, title string) error {
	if e := validOwner(user, platform); e != nil {
		return e
	}
	if id < 1 || !validTitle(title) {
		return apperror.InvalidRequest(nil)
	}
	return mapError(s.repository.Update(ctx, id, user, platform, strings.TrimSpace(title), s.now().UTC()))
}
func (s *Service) Hide(ctx context.Context, id, user, platform int64) error {
	if e := validOwner(user, platform); e != nil {
		return e
	}
	if id < 1 {
		return apperror.InvalidRequest(nil)
	}
	return mapError(s.repository.Hide(ctx, id, user, platform, s.now().UTC()))
}
func (s *Service) Messages(ctx context.Context, id, user, platform, after int64, limit int) ([]message.Model, error) {
	if e := validOwner(user, platform); e != nil {
		return nil, e
	}
	if id < 1 || after < 0 || limit < 1 || limit > 100 {
		return nil, apperror.InvalidRequest(nil)
	}
	rows, e := s.repository.Messages(ctx, id, user, platform, after, limit)
	return rows, mapError(e)
}
