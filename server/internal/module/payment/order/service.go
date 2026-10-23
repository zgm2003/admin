package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	auth "admin/server/internal/module/auth/login"
	"admin/server/internal/module/payment"
	paymentconfig "admin/server/internal/module/payment/config"
	rechargepackage "admin/server/internal/module/payment/rechargePackage"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/money"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/pagination"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

var (
	ErrRequestConflict = errors.New("payment order request fingerprint conflict")
	ErrOrderCreate     = errors.New("payment order create failed")
)

type Service struct {
	repository repository
	runtime    runtime
	packages   packageCatalog
	settings   sharedsetting.Reader
	checkout   checkoutFunc
	now        func() time.Time
	newOrderNo func(time.Time) (string, error)
}

func NewService(repository repository) *Service {
	return &Service{repository: repository, now: time.Now, newOrderNo: newOrderNo}
}

type runtime interface {
	Catalog(context.Context) ([]paymentconfig.CatalogItem, error)
	Credential(context.Context, int64, int64) (paymentconfig.BoundCredential, error)
	Credentials(context.Context, int64, int64) (alipay.Credentials, error)
}

type packageCatalog interface {
	Catalog(context.Context) ([]rechargepackage.CatalogItem, error)
}

type createRepository interface {
	FindByRequest(context.Context, int64, int64, string) (OrderRecord, error)
	Create(context.Context, *Model) error
}

type listRepository interface {
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]OrderRecord, error)
	Find(context.Context, int64) (OrderRecord, error)
}

type checkoutFunc func(context.Context, alipay.Credentials, alipay.CheckoutInput) (string, error)

func (s *Service) SetRuntime(value runtime, packages packageCatalog) {
	s.runtime, s.packages = value, packages
}

func (s *Service) SetCheckout(value checkoutFunc) { s.checkout = value }

func (s *Service) Options(ctx context.Context) (Options, error) {
	if err := ctx.Err(); err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	if s == nil || s.runtime == nil || s.packages == nil {
		return Options{}, apperror.DependencyUnavailable(errors.New("payment options dependencies missing"))
	}
	limits, err := s.rechargeLimits(ctx)
	if err != nil {
		return Options{}, err
	}
	catalog, err := s.runtime.Catalog(ctx)
	if err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	methods := make(map[payment.PayMethod]struct{})
	available := false
	for _, item := range catalog {
		if item.Status != yesno.Yes || item.Provider != payment.ProviderAlipay || !checkoutEnvironment(item.Environment) || item.CurrentVersion < 1 {
			continue
		}
		available = true
		bound, boundErr := s.runtime.Credential(ctx, item.ID, item.CurrentVersion)
		if boundErr != nil {
			return Options{}, apperror.DependencyUnavailable(boundErr)
		}
		for _, method := range bound.Version.PayMethods {
			methods[method] = struct{}{}
		}
	}
	orderedMethods := make([]option.Option[payment.PayMethod], 0, len(methods))
	for _, method := range []payment.PayMethod{payment.PayMethodWeb, payment.PayMethodH5} {
		if _, ok := methods[method]; ok {
			switch method {
			case payment.PayMethodWeb:
				orderedMethods = append(orderedMethods, option.New(ctx, method, "PC 网页支付", "PC web payment"))
			case payment.PayMethodH5:
				orderedMethods = append(orderedMethods, option.New(ctx, method, "H5 支付", "H5 payment"))
			}
		}
	}
	packages, err := s.packages.Catalog(ctx)
	if err != nil {
		return Options{}, apperror.DependencyUnavailable(err)
	}
	values := make([]PackageOption, 0, len(packages))
	for _, item := range packages {
		if item.ID < 1 || item.Name == "" || item.AmountCents < 1 {
			return Options{}, apperror.DependencyUnavailable(errors.New("stored recharge package is invalid"))
		}
		amount, formatErr := money.FormatRMBCents(item.AmountCents)
		if formatErr != nil {
			return Options{}, apperror.DependencyUnavailable(formatErr)
		}
		values = append(values, PackageOption{ID: item.ID, Name: item.Name, Amount: amount, Badge: item.Badge})
	}
	return Options{PayMethods: orderedMethods, Packages: values, Available: available && len(orderedMethods) > 0, Limits: limits}, nil
}

// Production checkout remains closed pending production payment acceptance.
// Trusted historical settlement is independent of this checkout gate.
func checkoutEnvironment(environment payment.Environment) bool {
	return environment == payment.EnvironmentSandbox
}

func (s *Service) List(ctx context.Context, query ListQuery) (pagination.Result[SafeValue], error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || (query.Page-1) > int(^uint(0)>>1)/query.PageSize {
		return pagination.Result[SafeValue]{}, apperror.InvalidRequest(errors.New("pagination invalid"))
	}
	if query.Personal && (query.UserID < 1 || query.PlatformID < 1) {
		return pagination.Result[SafeValue]{}, apperror.Unauthorized(nil)
	}
	if s == nil || s.repository == nil {
		return pagination.Result[SafeValue]{}, apperror.DependencyUnavailable(errors.New("payment order repository missing"))
	}
	repo, ok := s.repository.(listRepository)
	if !ok {
		return pagination.Result[SafeValue]{}, apperror.DependencyUnavailable(errors.New("payment order list repository missing"))
	}
	total, err := repo.Count(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	rows, err := repo.List(ctx, query)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	values := make([]SafeValue, 0, len(rows))
	for _, row := range rows {
		if query.Personal && (row.UserID != query.UserID || row.PlatformID != query.PlatformID) {
			return pagination.Result[SafeValue]{}, apperror.NotFound(ErrOrderNotFound)
		}
		value, valueErr := safeValue(ctx, row.Model, s.now().UTC(), checkoutEnvironment(row.ConfigEnvironment))
		if valueErr != nil {
			return pagination.Result[SafeValue]{}, apperror.DependencyUnavailable(valueErr)
		}
		values = append(values, value)
	}
	return pagination.Result[SafeValue]{List: values, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if id < 1 {
		return SafeValue{}, apperror.InvalidRequest(errors.New("order id invalid"))
	}
	if s == nil || s.repository == nil {
		return SafeValue{}, apperror.DependencyUnavailable(errors.New("payment order repository missing"))
	}
	repo, ok := s.repository.(listRepository)
	if !ok {
		return SafeValue{}, apperror.DependencyUnavailable(errors.New("payment order lookup repository missing"))
	}
	row, err := repo.Find(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrOrderNotFound) {
			return SafeValue{}, apperror.NotFound(err)
		}
		return SafeValue{}, apperror.DependencyUnavailable(err)
	}
	value, err := safeValue(ctx, row.Model, s.now().UTC(), checkoutEnvironment(row.ConfigEnvironment))
	if err != nil {
		return SafeValue{}, apperror.DependencyUnavailable(err)
	}
	return value, nil
}

func (s *Service) GetForIdentity(ctx context.Context, actor auth.Identity, id int64) (SafeValue, error) {
	if actor.UserID < 1 || actor.PlatformID < 1 {
		return SafeValue{}, apperror.Unauthorized(nil)
	}
	if id < 1 {
		return SafeValue{}, apperror.InvalidRequest(errors.New("order id invalid"))
	}
	if s == nil || s.repository == nil {
		return SafeValue{}, apperror.DependencyUnavailable(errors.New("payment order repository missing"))
	}
	repo, ok := s.repository.(listRepository)
	if !ok {
		return SafeValue{}, apperror.DependencyUnavailable(errors.New("payment order lookup repository missing"))
	}
	row, err := repo.Find(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrOrderNotFound) {
			return SafeValue{}, apperror.NotFound(err)
		}
		return SafeValue{}, apperror.DependencyUnavailable(err)
	}
	if row.UserID != actor.UserID || row.PlatformID != actor.PlatformID {
		return SafeValue{}, apperror.NotFound(ErrOrderNotFound)
	}
	value, err := safeValue(ctx, row.Model, s.now().UTC(), checkoutEnvironment(row.ConfigEnvironment))
	if err != nil {
		return SafeValue{}, apperror.DependencyUnavailable(err)
	}
	return value, nil
}

func (s *Service) Pay(ctx context.Context, actor auth.Identity, id int64) (Checkout, error) {
	if actor.UserID < 1 || actor.PlatformID < 1 {
		return Checkout{}, apperror.Unauthorized(nil)
	}
	if id < 1 {
		return Checkout{}, apperror.InvalidRequest(errors.New("order id invalid"))
	}
	if s == nil || s.repository == nil {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment order repository missing"))
	}
	repo, ok := s.repository.(listRepository)
	if !ok {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment order lookup repository missing"))
	}
	row, err := repo.Find(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrOrderNotFound) {
			return Checkout{}, apperror.NotFound(err)
		}
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	if row.UserID != actor.UserID || row.PlatformID != actor.PlatformID {
		return Checkout{}, apperror.NotFound(ErrOrderNotFound)
	}
	return s.checkoutExisting(ctx, row)
}

func (s *Service) Create(ctx context.Context, actor auth.Identity, input CreateInput) (Checkout, error) {
	if actor.UserID < 1 || actor.PlatformID < 1 {
		return Checkout{}, apperror.Unauthorized(nil)
	}
	if err := validateCreateInput(input); err != nil {
		return Checkout{}, apperror.InvalidRequest(err)
	}
	if s == nil || s.repository == nil || s.runtime == nil || s.packages == nil {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment order create dependencies missing"))
	}
	repo, ok := s.repository.(createRepository)
	if !ok {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment order create repository missing"))
	}
	fingerprint := requestFingerprint(input)
	existing, err := repo.FindByRequest(ctx, actor.UserID, actor.PlatformID, input.RequestID)
	if err == nil {
		if !equalFingerprint(existing.RequestFingerprint, fingerprint) {
			return Checkout{}, apperror.Conflict(i18n.KeyConflict, nil, ErrRequestConflict)
		}
		return s.checkoutExisting(ctx, existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	var packageID *int64
	packageName := "Custom recharge"
	amountCents := input.AmountCents
	if input.PackageID > 0 {
		packages, err := s.packages.Catalog(ctx)
		if err != nil {
			return Checkout{}, apperror.DependencyUnavailable(err)
		}
		for _, candidate := range packages {
			if candidate.ID == input.PackageID {
				id := candidate.ID
				packageID, packageName, amountCents = &id, candidate.Name, candidate.AmountCents
				break
			}
		}
		if packageID == nil || amountCents < 1 || packageName == "" {
			return Checkout{}, apperror.NotFound(errors.New("recharge package not available"))
		}
	} else {
		minimum, err := s.minimumRechargeCents(ctx)
		if err != nil {
			return Checkout{}, err
		}
		if amountCents < minimum {
			return Checkout{}, apperror.InvalidRequest(errors.New("custom recharge amount below minimum"))
		}
	}
	if _, err := money.CentsToUnits(amountCents); err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	catalog, err := s.runtime.Catalog(ctx)
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	var selected paymentconfig.CatalogItem
	channelAvailable := false
	for _, candidate := range catalog {
		if candidate.Status == yesno.Yes && candidate.Provider == payment.ProviderAlipay && checkoutEnvironment(candidate.Environment) && candidate.CurrentVersion > 0 {
			channelAvailable = true
			bound, err := s.runtime.Credential(ctx, candidate.ID, candidate.CurrentVersion)
			if err != nil {
				return Checkout{}, apperror.DependencyUnavailable(err)
			}
			if !containsPayMethod(bound.Version.PayMethods, input.PayMethod) {
				continue
			}
			selected = candidate
			break
		}
	}
	if selected.ID < 1 {
		if channelAvailable {
			return Checkout{}, apperror.InvalidRequest(errors.New("payment method unavailable"))
		}
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment channel unavailable"))
	}
	credentials, bound, err := s.credentials(ctx, selected.ID, selected.CurrentVersion)
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	if !checkoutEnvironment(bound.Environment) {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment environment mismatch"))
	}
	if !containsPayMethod(bound.Version.PayMethods, input.PayMethod) {
		return Checkout{}, apperror.InvalidRequest(errors.New("payment method unavailable"))
	}
	if _, err := parseReturnURL(input.ReturnURL); err != nil {
		return Checkout{}, apperror.InvalidRequest(err)
	}
	lifetime, err := s.orderLifetime(ctx)
	if err != nil {
		return Checkout{}, err
	}
	now := s.now().UTC()
	orderNo, err := s.newOrderNo(now)
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	row := &Model{OrderNo: orderNo, UserID: actor.UserID, PlatformID: actor.PlatformID, PackageID: packageID, PackageName: packageName, AmountCents: amountCents, ConfigID: selected.ID, ConfigVersion: selected.CurrentVersion, PayMethod: int16(input.PayMethod), RequestID: input.RequestID, RequestFingerprint: fingerprint, ReturnURL: input.ReturnURL, Status: StatusPending, ExpiresAt: now.Add(lifetime), NextCheckAt: pointerTime(now.Add(time.Minute)), CreatedAt: now, UpdatedAt: now}
	if err := repo.Create(ctx, row); err != nil {
		if concurrent, lookupErr := repo.FindByRequest(ctx, actor.UserID, actor.PlatformID, input.RequestID); lookupErr == nil {
			if !equalFingerprint(concurrent.RequestFingerprint, fingerprint) {
				return Checkout{}, apperror.Conflict(i18n.KeyConflict, nil, ErrRequestConflict)
			}
			return s.checkoutExisting(ctx, concurrent)
		}
		return Checkout{}, apperror.DependencyUnavailable(errors.Join(ErrOrderCreate, err))
	}
	return s.checkoutRow(ctx, *row, credentials)
}

func (s *Service) checkoutExisting(ctx context.Context, row OrderRecord) (Checkout, error) {
	model := row.Model
	if model.UserID < 1 || model.Status < StatusPending || model.Status > StatusClosed {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("stored payment order is invalid"))
	}
	value, err := safeValue(ctx, model, s.now().UTC(), checkoutEnvironment(row.ConfigEnvironment))
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	if model.Status != StatusPending || !model.ExpiresAt.After(s.now().UTC()) {
		return Checkout{Order: value}, nil
	}
	if s.runtime == nil {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment runtime missing"))
	}
	credentials, bound, err := s.credentials(ctx, model.ConfigID, model.ConfigVersion)
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	if bound.Environment != row.ConfigEnvironment || !checkoutEnvironment(bound.Environment) {
		return Checkout{}, apperror.DependencyUnavailable(errors.New("payment environment mismatch"))
	}
	return s.checkoutRow(ctx, model, credentials)
}

func (s *Service) credentials(ctx context.Context, configID, version int64) (alipay.Credentials, paymentconfig.BoundCredential, error) {
	if s.runtime == nil {
		return alipay.Credentials{}, paymentconfig.BoundCredential{}, errors.New("payment runtime missing")
	}
	bound, err := s.runtime.Credential(ctx, configID, version)
	if err != nil {
		return alipay.Credentials{}, paymentconfig.BoundCredential{}, err
	}
	if bound.ConfigID != configID || bound.Version.ConfigID != configID || bound.Version.Version != version || bound.Provider != payment.ProviderAlipay || !bound.Environment.Valid() || bound.AppID == "" || bound.SellerID == "" {
		return alipay.Credentials{}, paymentconfig.BoundCredential{}, errors.New("payment credential binding invalid")
	}
	credentials, err := s.runtime.Credentials(ctx, configID, version)
	if err != nil {
		return alipay.Credentials{}, paymentconfig.BoundCredential{}, err
	}
	if credentials.AppID != bound.AppID || credentials.SellerID != bound.SellerID || credentials.Environment != bound.Environment || credentials.NotifyURL != bound.Version.NotifyURL {
		return alipay.Credentials{}, paymentconfig.BoundCredential{}, errors.New("payment credential identity mismatch")
	}
	return credentials, bound, nil
}

func (s *Service) checkoutRow(ctx context.Context, row Model, credentials alipay.Credentials) (Checkout, error) {
	value, err := safeValue(ctx, row, s.now().UTC(), checkoutEnvironment(credentials.Environment))
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	if !value.Actions.Pay {
		return Checkout{Order: value}, nil
	}
	checkout := s.checkout
	if checkout == nil {
		checkout = func(work context.Context, c alipay.Credentials, input alipay.CheckoutInput) (string, error) {
			return alipay.Checkout(work, c, input)
		}
	}
	url, err := checkout(ctx, credentials, alipay.CheckoutInput{OrderNo: row.OrderNo, Subject: row.PackageName, AmountCents: row.AmountCents, Method: payment.PayMethod(row.PayMethod), ReturnURL: row.ReturnURL})
	if err != nil {
		return Checkout{}, apperror.DependencyUnavailable(err)
	}
	return Checkout{Order: value, URL: url}, nil
}

func equalFingerprint(a, b []byte) bool { return len(a) == len(b) && string(a) == string(b) }

func containsPayMethod(methods paymentconfig.PayMethods, method payment.PayMethod) bool {
	for _, value := range methods {
		if value == method {
			return true
		}
	}
	return false
}

func newOrderNo(now time.Time) (string, error) {
	random := make([]byte, 5)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "PO" + now.UTC().Format("20060102150405") + hex.EncodeToString(random), nil
}

func pointerTime(value time.Time) *time.Time { return &value }

func (s *Service) Confirm(ctx context.Context, payment ConfirmedPayment, callbackEventID *int64) (SettlementResult, error) {
	if err := validateConfirmation(payment, callbackEventID); err != nil {
		return SettlementResult{}, apperror.InvalidRequest(err)
	}
	if s == nil || s.repository == nil {
		return SettlementResult{}, apperror.DependencyUnavailable(errors.New("payment order repository missing"))
	}
	result, err := s.repository.Confirm(ctx, payment, callbackEventID, s.now().UTC())
	if err != nil {
		return SettlementResult{}, mapError(err)
	}
	return result, nil
}

func validateConfirmation(payment ConfirmedPayment, callbackEventID *int64) error {
	if !payment.Environment.Valid() {
		return ErrConfigMismatch
	}
	if payment.ConfigID < 1 || payment.ConfigVersion < 1 || payment.OrderNo == "" || len(payment.OrderNo) > 32 || strings.TrimSpace(payment.OrderNo) != payment.OrderNo || payment.TradeNo == "" || len(payment.TradeNo) > 64 || strings.TrimSpace(payment.TradeNo) != payment.TradeNo || payment.AppID == "" || len(payment.AppID) > 64 || payment.AmountCents < 1 || payment.ProofSource == 0 || !payment.ProofSource.Valid() {
		return fmt.Errorf("payment confirmation coordinates or amount invalid")
	}
	if _, ok := successStatuses[payment.Status]; !ok {
		return fmt.Errorf("payment status is not successful")
	}
	if callbackEventID != nil && *callbackEventID < 1 {
		return fmt.Errorf("callback event id invalid")
	}
	if callbackEventID != nil && payment.ProofSource != ProofSourceCallback {
		return fmt.Errorf("callback event requires callback proof source")
	}
	if payment.ProofSource == ProofSourceCallback {
		if payment.SellerID == nil || strings.TrimSpace(*payment.SellerID) == "" || len(*payment.SellerID) > 64 || strings.TrimSpace(*payment.SellerID) != *payment.SellerID {
			return fmt.Errorf("callback seller identity missing")
		}
	} else if payment.SellerID != nil && (strings.TrimSpace(*payment.SellerID) != *payment.SellerID || len(*payment.SellerID) > 64) {
		return fmt.Errorf("seller identity invalid")
	}
	if payment.PaidAt != nil && payment.PaidAt.IsZero() {
		return fmt.Errorf("paidAt invalid")
	}
	return nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var public *apperror.Error
	if errors.As(err, &public) {
		return err
	}
	return &apperror.Error{HTTPStatus: 503, Code: apperror.CodeDependencyUnavailable, MessageKey: i18n.KeyDependencyUnavailable, Cause: err}
}
