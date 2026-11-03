package run

import (
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

type QueryService struct {
	repository *QueryRepository
	redis      redis.UniversalClient
	now        func() time.Time
}

func NewQueryService(r *QueryRepository) *QueryService {
	return &QueryService{repository: r, now: time.Now}
}
func (s *QueryService) SetSummaryRedis(r redis.UniversalClient) { s.redis = r }
func queryError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, ErrQueryRangeTooLarge) {
		return apperror.InvalidRequest(e)
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return apperror.NotFound(e)
	}
	return apperror.DependencyUnavailable(e)
}
func (s *QueryService) Detail(ctx context.Context, id int64, o QueryOwner, admin bool) (Model, error) {
	if id < 1 {
		return Model{}, apperror.InvalidRequest(nil)
	}
	if !o.valid() {
		return Model{}, apperror.Unauthorized(nil)
	}
	if s == nil || s.repository == nil {
		return Model{}, apperror.DependencyUnavailable(nil)
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	scope := o
	if admin {
		scope = QueryOwner{}
	}
	m, e := s.repository.Find(ctx, id, scope)
	return m, queryError(e)
}
func (s *QueryService) List(ctx context.Context, q RunQuery, o QueryOwner, admin bool) ([]Model, bool, error) {
	if !o.valid() {
		return nil, false, apperror.Unauthorized(nil)
	}
	if e := q.normalize(s.now().UTC(), false); e != nil {
		return nil, false, apperror.InvalidRequest(e)
	}
	if !admin {
		q.UserID = o.UserID
		q.PlatformID = o.PlatformID
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	rows, e := s.repository.List(ctx, q)
	if e != nil {
		return nil, false, queryError(e)
	}
	more := len(rows) > q.Limit
	if more {
		rows = rows[:q.Limit]
	}
	return rows, more, nil
}
func (s *QueryService) Summary(ctx context.Context, q RunQuery) (QuerySummary, error) {
	if e := q.normalize(s.now().UTC(), true); e != nil {
		return QuerySummary{}, apperror.InvalidRequest(e)
	}
	if s.redis == nil {
		return QuerySummary{}, apperror.DependencyUnavailable(nil)
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	q.StartAt = q.StartAt.Truncate(time.Second)
	q.EndAt = q.EndAt.Truncate(time.Second)
	q.BeforeID = 0
	q.Limit = 20
	b, _ := json.Marshal(q)
	key := fmt.Sprintf("ai:run:summary:%x", sha256.Sum256(b))
	value, e := s.redis.Get(ctx, key).Result()
	if e == nil {
		var v QuerySummary
		if json.Unmarshal([]byte(value), &v) != nil {
			return v, apperror.DependencyUnavailable(nil)
		}
		return v, nil
	}
	if !errors.Is(e, redis.Nil) {
		return QuerySummary{}, queryError(e)
	}
	token := uuid.NewString()
	ok, e := s.redis.SetNX(ctx, "ai:run:summary:rebuild", token, time.Second).Result()
	if e != nil || !ok {
		return QuerySummary{}, apperror.DependencyUnavailable(e)
	}
	v, e := s.repository.Summary(ctx, q)
	if e != nil {
		return v, queryError(e)
	}
	b, e = json.Marshal(v)
	if e != nil {
		return v, queryError(e)
	}
	if e = s.redis.Set(ctx, key, b, 5*time.Second).Err(); e != nil {
		return v, queryError(e)
	}
	return v, nil
}
func (s *QueryService) Events(ctx context.Context, id, after int64, limit int, o QueryOwner) (CursorPage[EventResponse], error) {
	if after < 0 || limit < 1 || limit > 100 {
		return CursorPage[EventResponse]{}, apperror.InvalidRequest(nil)
	}
	m, e := s.Detail(ctx, id, o, false)
	if e != nil {
		return CursorPage[EventResponse]{}, e
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	rows, e := s.repository.Events(ctx, id, after, limit)
	if e != nil {
		return CursorPage[EventResponse]{}, queryError(e)
	}
	if m.NextEventSequence > after && (len(rows) == 0 || rows[0].Sequence > after+1) {
		return CursorPage[EventResponse]{List: []EventResponse{}, NextSequence: m.NextEventSequence, ResetReason: "EVENTS_EXPIRED"}, nil
	}
	out := page(rows, limit, func(v EventResponse) int64 { return v.Sequence })
	out.NextBeforeID = ""
	out.NextSequence = after
	if len(out.List) > 0 {
		out.NextSequence = out.List[len(out.List)-1].Sequence
	}
	return out, nil
}
func (s *QueryService) Children(ctx context.Context, id, before int64, limit int, kind string, o QueryOwner, admin bool) (any, error) {
	if before < 0 || limit < 1 || limit > 100 {
		return nil, apperror.InvalidRequest(nil)
	}
	if _, e := s.Detail(ctx, id, o, admin); e != nil {
		return nil, e
	}
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	v, e := s.repository.Children(ctx, id, before, limit, kind)
	return v, queryError(e)
}
func (s *Service) Cancel(ctx context.Context, id, user, platform int64) error {
	if id < 1 || user < 1 || platform < 1 {
		return apperror.InvalidRequest(nil)
	}
	r, ok := s.repository.(interface {
		Cancel(context.Context, int64, int64, int64, time.Time) error
	})
	if !ok {
		return apperror.DependencyUnavailable(nil)
	}
	return queryError(r.Cancel(ctx, id, user, platform, s.now().UTC()))
}

func (s *QueryService) Content(ctx context.Context, m Model) (RunInput, json.RawMessage, error) {
	var input RunInput
	if e := json.Unmarshal(m.InputSnapshot, &input); e != nil {
		return input, nil, queryError(e)
	}
	input.FileSnapshots = nil
	ctx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	result, e := s.repository.Result(ctx, m.ID)
	if e != nil {
		return input, nil, queryError(e)
	}
	result, e = publicResult(result)
	return input, result, queryError(e)
}

func publicCreateError(e error) error {
	if e == nil {
		return nil
	}
	var existing *apperror.Error
	if errors.As(e, &existing) {
		return existing
	}
	if errors.Is(e, ErrGateDenied) {
		return apperror.RateLimited(e)
	}
	if errors.Is(e, ErrAdmissionDenied) || errors.Is(e, ErrUnknownInflight) || errors.Is(e, ErrIdempotencyConflict) {
		return apperror.Conflict(i18n.KeyConflict, nil, e)
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return apperror.NotFound(e)
	}
	return apperror.DependencyUnavailable(e)
}
func (s *Service) PublicCreate(ctx context.Context, in CreateInput) (Model, bool, error) {
	m, created, e := s.Create(ctx, in)
	return m, created, publicCreateError(e)
}
