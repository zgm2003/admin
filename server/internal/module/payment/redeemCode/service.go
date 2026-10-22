package redeemcode

import (
	"context"
	"errors"

	"strings"
	"time"
	"unicode/utf8"

	"admin/server/internal/module/payment/transaction"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/pagination"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrRequestConflict = errors.New("redeem request fingerprint conflict")
	ErrCodeUnavailable = errors.New("redeem code unavailable")
	ErrCodeExpired     = errors.New("redeem code expired")
	ErrInvalidInput    = errors.New("redeem code input invalid")
	ErrInvariant       = errors.New("redeem code invariant failed")
	ErrLimit           = errors.New("redeem code limit reached")
	ErrLeaseLost       = errors.New("redeem code lease lost")
)

type repository interface {
	CreateBatch(context.Context, Batch, []Model, time.Time) (Batch, []Model, error)
	List(context.Context, ListQuery, time.Time) ([]Model, int64, error)
	Get(context.Context, int64) (Model, error)
	Lookup(context.Context, string) (Model, error)
	Export(context.Context, []int64) ([]Model, error)
	Void(context.Context, []int64, time.Time) error
	Redeem(context.Context, int64, int64, string, time.Time) (transaction.Model, error)
}

// Reserve is user-scoped (not platform-scoped), cross-instance and fail closed.
// Finish increments only invalid-code outcomes; a token guards release/count.
// The 10s service deadline is strictly shorter than the 15s admission lease.
type Limiter interface {
	Reserve(context.Context, int64) (func(context.Context, bool) error, error)
}

type Service struct {
	repository repository
	limiter    Limiter
}

func NewService(r repository, l Limiter) *Service {
	return &Service{repository: r, limiter: l}
}
func (s *Service) ready() error {
	if s == nil || s.repository == nil {
		return apperror.DependencyUnavailable(errors.New("redeem service missing dependencies"))
	}
	return nil
}

func validateGenerate(in GenerateInput) (GenerateInput, error) {
	id, e := uuid.Parse(in.RequestID)
	if e != nil || id == uuid.Nil || id.String() != in.RequestID {
		return in, ErrInvalidInput
	}
	if in.AmountCents < 1 || in.Quantity < 1 || in.Quantity > maxBatchSize {
		return in, ErrInvalidInput
	}
	if _, e = money.CentsToUnits(in.AmountCents); e != nil {
		return in, ErrInvalidInput
	}
	if !utf8.ValidString(in.Remark) || utf8.RuneCountInString(in.Remark) > 255 {
		return in, ErrInvalidInput
	}
	in.Remark = strings.TrimSpace(in.Remark)
	if in.ExpiresAt != nil {
		v := in.ExpiresAt.UTC().Truncate(time.Microsecond)
		in.ExpiresAt = &v
	}
	return in, nil
}
func (s *Service) Generate(ctx context.Context, actor int64, in GenerateInput) (GenerateResult, error) {
	if ctx == nil {
		return GenerateResult{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return GenerateResult{}, e
	}
	if actor < 1 {
		return GenerateResult{}, apperror.Unauthorized(nil)
	}
	in, e := validateGenerate(in)
	if e != nil {
		return GenerateResult{}, apperror.InvalidRequest(e)
	}
	now := time.Now().UTC()
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		return GenerateResult{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	no, e := batchNo()
	if e != nil {
		return GenerateResult{}, mapError(e)
	}
	batch := Batch{BatchNo: no, RequestID: in.RequestID, RequestFingerprint: fingerprint(in), AmountCents: in.AmountCents, Quantity: in.Quantity, ExpiresAt: in.ExpiresAt, CreatedBy: actor, CreatedAt: now, UpdatedAt: now}
	if in.Remark != "" {
		batch.Remark = &in.Remark
	}
	rows := make([]Model, 0, in.Quantity)
	seen := make(map[string]struct{}, in.Quantity)
	for attempts := 0; len(rows) < in.Quantity; attempts++ {
		if attempts >= in.Quantity*2 {
			return GenerateResult{}, mapError(ErrInvariant)
		}
		code, e := newCode()
		if e != nil {
			return GenerateResult{}, mapError(e)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		rows = append(rows, Model{Code: code, Status: StatusUnused, CreatedAt: now, UpdatedAt: now})
	}
	batch, rows, e = s.repository.CreateBatch(ctx, batch, rows, now)
	if e != nil {
		return GenerateResult{}, mapError(e)
	}
	codes := make([]string, 0, len(rows))
	for _, r := range rows {
		code, e := storedCode(r.Code)
		if e != nil {
			return GenerateResult{}, mapError(e)
		}
		codes = append(codes, code)
	}
	return GenerateResult{BatchID: batch.ID, BatchNo: batch.BatchNo, Codes: codes}, nil
}
func (s *Service) project(r Model, now time.Time) (Value, error) {
	if r.ID < 1 || r.BatchID < 1 || r.Status < StatusUnused || r.Status > StatusVoided || r.AmountCents < 1 {
		return Value{}, ErrInvariant
	}
	if (r.Status == StatusUsed) != (r.UsedBy != nil && r.UsedAt != nil) || (r.Status != StatusUsed && (r.UsedBy != nil || r.UsedAt != nil)) {
		return Value{}, ErrInvariant
	}
	code, e := storedCode(r.Code)
	if e != nil {
		return Value{}, e
	}
	amount, e := money.FormatRMBCents(r.AmountCents)
	if e != nil {
		return Value{}, e
	}
	status := projectedStatus(r, now)
	return Value{ID: r.ID, BatchID: r.BatchID, BatchNo: r.BatchNo, Code: code, Status: status, Amount: amount, Remark: r.Remark, ExpiresAt: r.ExpiresAt, UsedBy: r.UsedBy, UsedAt: r.UsedAt, CreatedAt: r.CreatedAt, Actions: Actions{Void: status == StatusUnused, Export: true}}, nil
}
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.Result[Value], error) {
	if ctx == nil {
		return pagination.Result[Value]{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return pagination.Result[Value]{}, e
	}
	if q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || (q.Page-1) > int(^uint(0)>>1)/q.PageSize || q.BatchID < 0 || (q.Status != nil && (*q.Status < 1 || *q.Status > 4)) {
		return pagination.Result[Value]{}, apperror.InvalidRequest(nil)
	}
	now := time.Now()
	rows, total, e := s.repository.List(ctx, q, now)
	if e != nil {
		return pagination.Result[Value]{}, mapError(e)
	}
	out := make([]Value, 0, len(rows))
	for _, r := range rows {
		v, e := s.project(r, now)
		if e != nil {
			return pagination.Result[Value]{}, mapError(e)
		}
		out = append(out, v)
	}
	return pagination.Result[Value]{List: out, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
func (s *Service) Get(ctx context.Context, id int64) (Value, error) {
	if ctx == nil {
		return Value{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return Value{}, e
	}
	if id < 1 {
		return Value{}, apperror.InvalidRequest(nil)
	}
	r, e := s.repository.Get(ctx, id)
	if e != nil {
		return Value{}, mapError(e)
	}
	v, e := s.project(r, time.Now())
	if e != nil {
		return Value{}, mapError(e)
	}
	return v, nil
}
func (s *Service) Lookup(ctx context.Context, raw string) (Value, error) {
	if ctx == nil {
		return Value{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return Value{}, e
	}
	code, e := normalizeCode(raw)
	if e != nil {
		return Value{}, apperror.InvalidRequest(e)
	}
	r, e := s.repository.Lookup(ctx, code)
	if e != nil {
		return Value{}, mapError(e)
	}
	v, e := s.project(r, time.Now())
	if e != nil {
		return Value{}, mapError(e)
	}
	return v, nil
}
func validateIDs(ids []int64, limit int) error {
	if len(ids) < 1 || len(ids) > limit {
		return ErrInvalidInput
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id < 1 {
			return ErrInvalidInput
		}
		if _, ok := seen[id]; ok {
			return ErrInvalidInput
		}
		seen[id] = struct{}{}
	}
	return nil
}
func (s *Service) Void(ctx context.Context, ids []int64) error {
	if ctx == nil {
		return apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return e
	}
	if e := validateIDs(ids, 1000); e != nil {
		return apperror.InvalidRequest(e)
	}
	if e := s.repository.Void(ctx, ids, time.Now()); e != nil {
		return mapError(e)
	}
	return nil
}
func (s *Service) Export(ctx context.Context, ids []int64) ([]ExportValue, error) {
	if ctx == nil {
		return nil, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return nil, e
	}
	if e := validateIDs(ids, 10000); e != nil {
		return nil, apperror.InvalidRequest(e)
	}
	rows, e := s.repository.Export(ctx, ids)
	if e != nil {
		return nil, mapError(e)
	}
	values := make([]ExportValue, 0, len(rows))
	now := time.Now()
	for _, r := range rows {
		v, e := s.project(r, now)
		if e != nil {
			return nil, mapError(e)
		}
		values = append(values, ExportValue{Value: v})
	}
	return values, nil
}
func invalidRedemption(e error) bool {
	return errors.Is(e, ErrInvalidInput) || errors.Is(e, gorm.ErrRecordNotFound) || errors.Is(e, ErrCodeUnavailable) || errors.Is(e, ErrCodeExpired)
}
func (s *Service) Redeem(ctx context.Context, userID, platformID int64, raw string) (Redemption, error) {
	if ctx == nil {
		return Redemption{}, apperror.InvalidRequest(ErrInvalidInput)
	}
	if e := s.ready(); e != nil {
		return Redemption{}, e
	}
	if userID < 1 || platformID < 1 {
		return Redemption{}, apperror.Unauthorized(nil)
	}
	if s.limiter == nil {
		return Redemption{}, apperror.DependencyUnavailable(errors.New("redeem limiter missing"))
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	finish, e := s.limiter.Reserve(work, userID)
	if e != nil {
		return Redemption{}, mapError(e)
	}
	code, e := normalizeCode(raw)
	var row transaction.Model
	if e != nil {
		e = ErrInvalidInput
	}
	if e == nil {
		row, e = s.repository.Redeem(work, userID, platformID, code, time.Now().UTC())
	}
	// Still use the inbound parent context for this final I/O: the work timeout
	// never turns a failed release/count into a silent success.
	finishErr := finish(ctx, invalidRedemption(e))
	if finishErr != nil {
		return Redemption{}, mapError(finishErr)
	}
	if e != nil {
		return Redemption{}, mapError(e)
	}
	amount, e := money.FormatRMBUnits(row.AmountUnits)
	if e != nil {
		return Redemption{}, mapError(e)
	}
	balance, e := money.FormatRMBUnits(row.BalanceAfterUnits)
	if e != nil {
		return Redemption{}, mapError(e)
	}
	return Redemption{TransactionID: row.ID, TransactionNo: row.TransactionNo, Amount: amount, Balance: balance, CreatedAt: row.CreatedAt}, nil
}
func mapError(e error) error {
	var public *apperror.Error
	if errors.As(e, &public) {
		return e
	}
	switch {
	case errors.Is(e, ErrLimit):
		return apperror.RateLimited(e)
	case errors.Is(e, ErrInvalidInput):
		return apperror.InvalidRequest(e)
	case errors.Is(e, gorm.ErrRecordNotFound):
		return apperror.NotFound(e)
	case errors.Is(e, ErrRequestConflict), errors.Is(e, ErrCodeUnavailable), errors.Is(e, ErrCodeExpired):
		return apperror.Conflict(i18n.KeyConflict, nil, e)
	default:
		return apperror.DependencyUnavailable(e)
	}
}
