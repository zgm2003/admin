package callbackevent

import (
	"context"
	"errors"
	"net/url"
	"time"

	"admin/server/internal/module/payment/order"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/pagination"
	"gorm.io/gorm"
)

var (
	ErrEventFactConflict = errors.New("payment callback event fact conflict")
	ErrEventPending      = errors.New("payment callback event remains pending")
	ErrEventRejected     = errors.New("payment callback event was rejected")
	ErrUnknownStatus     = errors.New("payment callback provider status is unknown")
)

type runtime interface {
	Credentials(context.Context, int64, int64) (alipay.Credentials, error)
}
type confirmer interface {
	Confirm(context.Context, order.ConfirmedPayment, *int64) (order.SettlementResult, error)
}
type repository interface {
	FindByDedupe(context.Context, int64, int64, []byte) (Model, error)
	Create(context.Context, *Model) error
	UpdateResult(context.Context, int64, ProcessStatus, *ErrorCode, time.Time) error
	Find(context.Context, int64) (Model, error)
	FindOrderID(context.Context, string) (*int64, error)
	CountByOrder(context.Context, int64) (int64, error)
	ListByOrder(context.Context, int64, int, int) ([]Model, error)
}

type Service struct {
	repository repository
	runtime    runtime
	confirmer  confirmer
	now        func() time.Time
}

func NewService(repository repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func (s *Service) SetRuntime(value runtime)     { s.runtime = value }
func (s *Service) SetConfirmer(value confirmer) { s.confirmer = value }

type ReceiveResult struct {
	EventID  int64
	Replayed bool
	Ignored  bool
}

func isPaidStatus(status string) bool { return status == "TRADE_SUCCESS" || status == "TRADE_FINISHED" }
func isIgnoredStatus(status string) bool {
	return status == "WAIT_BUYER_PAY" || status == "TRADE_CLOSED"
}

func (s *Service) Receive(ctx context.Context, configID, version int64, values url.Values) (ReceiveResult, error) {
	if ctx == nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.New("payment callback context missing"))
	}
	if err := ctx.Err(); err != nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	if configID < 1 || version < 1 {
		return ReceiveResult{}, apperror.InvalidRequest(errors.New("payment callback coordinates invalid"))
	}
	if s == nil || s.repository == nil || s.runtime == nil || s.confirmer == nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.New("payment callback dependencies missing"))
	}
	credentials, err := s.runtime.Credentials(ctx, configID, version)
	if err != nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	if !credentials.Environment.Valid() || !validFact(credentials.AppID, 64) || !validFact(credentials.SellerID, 64) {
		return ReceiveResult{}, apperror.DependencyUnavailable(order.ErrConfigMismatch)
	}
	notify, err := alipay.VerifyNotify(ctx, credentials, values)
	if err != nil {
		if errors.Is(err, alipay.ErrNotifySignature) || errors.Is(err, alipay.ErrNotifyInvalid) {
			return ReceiveResult{}, apperror.InvalidRequest(err)
		}
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	payload := payloadFromNotify(notify)
	body, err := marshalPayload(payload)
	if err != nil {
		return ReceiveResult{}, apperror.InvalidRequest(err)
	}
	key := dedupeKey(configID, version, payload)
	existing, findErr := s.repository.FindByDedupe(ctx, configID, version, key)
	if findErr == nil {
		return s.replay(ctx, existing, payload, notify, credentials)
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return ReceiveResult{}, apperror.DependencyUnavailable(findErr)
	}
	orderID, err := s.repository.FindOrderID(ctx, payload.OrderNo)
	if err != nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	now := s.now().UTC()
	row := &Model{ConfigID: configID, ConfigVersion: version, DedupeKey: key, OrderID: orderID, Payload: body, ProcessStatus: ProcessPending, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.Create(ctx, row); err != nil {
		concurrent, lookupErr := s.repository.FindByDedupe(ctx, configID, version, key)
		if lookupErr == nil {
			return s.replay(ctx, concurrent, payload, notify, credentials)
		}
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	if row.ID < 1 {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.New("callback event id missing after create"))
	}
	return s.process(ctx, *row, notify, credentials, false)
}

func (s *Service) replay(ctx context.Context, row Model, payload Payload, notify alipay.Notify, credentials alipay.Credentials) (ReceiveResult, error) {
	stored, err := value(ctx, row)
	if err != nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(err)
	}
	if payloadFacts(stored.Payload) != payloadFacts(payload) {
		return ReceiveResult{}, apperror.Conflict(i18n.KeyConflict, nil, ErrEventFactConflict)
	}
	return s.process(ctx, row, notify, credentials, true)
}

func (s *Service) process(ctx context.Context, row Model, notify alipay.Notify, credentials alipay.Credentials, replayed bool) (ReceiveResult, error) {
	switch row.ProcessStatus {
	case ProcessSuccess, ProcessIgnored:
		return ReceiveResult{EventID: row.ID, Replayed: replayed, Ignored: row.ProcessStatus == ProcessIgnored}, nil
	case ProcessRejected:
		return ReceiveResult{}, apperror.InvalidRequest(ErrEventRejected)
	case ProcessPending:
	default:
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.New("stored callback process status invalid"))
	}
	if notify.AppID != credentials.AppID || notify.SellerID == nil || *notify.SellerID != credentials.SellerID {
		code := ErrorIdentity
		return s.recordOutcome(ctx, row, ProcessRejected, &code, apperror.InvalidRequest(ErrEventRejected), replayed)
	}
	if isIgnoredStatus(notify.Status) {
		return s.recordOutcome(ctx, row, ProcessIgnored, nil, nil, replayed)
	}
	if !isPaidStatus(notify.Status) {
		code := ErrorUnknownStatus
		return s.recordOutcome(ctx, row, ProcessPending, &code, apperror.DependencyUnavailable(ErrUnknownStatus), replayed)
	}
	settled, err := s.confirmer.Confirm(ctx, order.ConfirmedPayment{ConfigID: row.ConfigID, ConfigVersion: row.ConfigVersion, Environment: credentials.Environment, OrderNo: notify.OrderNo, TradeNo: notify.TradeNo, AppID: notify.AppID, SellerID: notify.SellerID, ProofSource: order.ProofSourceCallback, AmountCents: notify.AmountCents, Status: notify.Status, PaidAt: notify.PaidAt}, &row.ID)
	if err != nil {
		code := classifyError(err)
		status := ProcessPending
		if code >= ErrorConfigMismatch && code <= ErrorFactConflict {
			status = ProcessRejected
		}
		if status == ProcessRejected {
			return s.recordOutcome(ctx, row, status, &code, apperror.InvalidRequest(err), replayed)
		}
		return s.recordOutcome(ctx, row, status, &code, apperror.DependencyUnavailable(errors.Join(ErrEventPending, err)), replayed)
	}
	return ReceiveResult{EventID: row.ID, Replayed: replayed || settled.Replayed}, nil
}

func (s *Service) recordOutcome(ctx context.Context, row Model, status ProcessStatus, code *ErrorCode, outcomeErr error, replayed bool) (ReceiveResult, error) {
	err := s.repository.UpdateResult(ctx, row.ID, status, code, s.now().UTC())
	if err == nil {
		if outcomeErr != nil {
			return ReceiveResult{}, outcomeErr
		}
		return ReceiveResult{EventID: row.ID, Replayed: replayed, Ignored: status == ProcessIgnored}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.Join(outcomeErr, err))
	}
	// Another receiver may have committed the settlement while this one was
	// recording a failure. Inspect durable state instead of swallowing a CAS miss.
	latest, lookupErr := s.repository.Find(ctx, row.ID)
	if lookupErr != nil {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.Join(err, lookupErr))
	}
	latestValue, validationErr := value(ctx, latest)
	if validationErr != nil || latest.ConfigID != row.ConfigID || latest.ConfigVersion != row.ConfigVersion {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.Join(ErrEventFactConflict, validationErr))
	}
	original, payloadErr := decodePayload(row.Payload)
	if payloadErr != nil || payloadFacts(original) != payloadFacts(latestValue.Payload) {
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.Join(ErrEventFactConflict, payloadErr))
	}
	switch latest.ProcessStatus {
	case ProcessSuccess, ProcessIgnored:
		return ReceiveResult{EventID: row.ID, Replayed: true, Ignored: latest.ProcessStatus == ProcessIgnored}, nil
	case ProcessRejected:
		return ReceiveResult{}, apperror.InvalidRequest(ErrEventRejected)
	default:
		return ReceiveResult{}, apperror.DependencyUnavailable(errors.Join(outcomeErr, err))
	}
}

func classifyError(err error) ErrorCode {
	switch {
	case errors.Is(err, order.ErrConfigMismatch):
		return ErrorConfigMismatch
	case errors.Is(err, order.ErrIdentityMismatch):
		return ErrorIdentity
	case errors.Is(err, order.ErrAmountMismatch):
		return ErrorAmountMismatch
	case errors.Is(err, order.ErrCallbackMismatch), errors.Is(err, ErrEventFactConflict):
		return ErrorFactConflict
	case errors.Is(err, order.ErrTradeConflict):
		return ErrorTradeConflict
	case errors.Is(err, order.ErrOrderMismatch), errors.Is(err, order.ErrOrderNotFound):
		return ErrorOrderMismatch
	case errors.Is(err, order.ErrBalanceOverflow):
		return ErrorBalance
	default:
		return ErrorDependency
	}
}

func (s *Service) Get(ctx context.Context, id int64) (Value, error) {
	if id < 1 {
		return Value{}, apperror.InvalidRequest(errors.New("callback event id invalid"))
	}
	if s == nil || s.repository == nil {
		return Value{}, apperror.DependencyUnavailable(errors.New("callback event repository missing"))
	}
	row, err := s.repository.Find(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Value{}, apperror.NotFound(err)
		}
		return Value{}, apperror.DependencyUnavailable(err)
	}
	result, err := value(ctx, row)
	if err != nil {
		return Value{}, apperror.DependencyUnavailable(err)
	}
	return result, nil
}

func (s *Service) GetForOrder(ctx context.Context, orderID, eventID int64) (Value, error) {
	if orderID < 1 || eventID < 1 {
		return Value{}, apperror.InvalidRequest(errors.New("callback audit coordinates invalid"))
	}
	result, err := s.Get(ctx, eventID)
	if err != nil {
		return Value{}, err
	}
	if result.OrderID == nil || *result.OrderID != orderID {
		return Value{}, apperror.NotFound(nil)
	}
	return result, nil
}

func (s *Service) ListByOrder(ctx context.Context, orderID int64, page, pageSize int) (pagination.Result[Value], error) {
	if orderID < 1 || page < 1 || pageSize < 1 || pageSize > 100 || (page-1) > int(^uint(0)>>1)/pageSize {
		return pagination.Result[Value]{}, apperror.InvalidRequest(errors.New("callback event pagination invalid"))
	}
	if s == nil || s.repository == nil {
		return pagination.Result[Value]{}, apperror.DependencyUnavailable(errors.New("callback event repository missing"))
	}
	total, err := s.repository.CountByOrder(ctx, orderID)
	if err != nil {
		return pagination.Result[Value]{}, apperror.DependencyUnavailable(err)
	}
	rows, err := s.repository.ListByOrder(ctx, orderID, (page-1)*pageSize, pageSize)
	if err != nil {
		return pagination.Result[Value]{}, apperror.DependencyUnavailable(err)
	}
	values := make([]Value, 0, len(rows))
	for _, row := range rows {
		projected, valueErr := value(ctx, row)
		if valueErr != nil {
			return pagination.Result[Value]{}, apperror.DependencyUnavailable(valueErr)
		}
		values = append(values, projected)
	}
	return pagination.Result[Value]{List: values, Total: total, Page: page, PageSize: pageSize}, nil
}
