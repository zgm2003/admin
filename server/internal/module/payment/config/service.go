package config

import (
	"admin/server/internal/module/payment"
	"admin/server/internal/payment/alipay"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	cache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/pagination"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

type repository interface {
	CatalogFacts(context.Context) ([]CatalogItem, error)
	CredentialFacts(context.Context, int64, int64) (BoundCredential, error)
	ValidateDependencies() error
	Count(context.Context, ListQuery) (int64, error)
	List(context.Context, ListQuery) ([]Current, error)
	FindByID(context.Context, int64) (Current, error)
	Create(context.Context, *Model, Version, int64) (cache.MutationResult, error)
	UpdateMetadata(context.Context, int64, MetadataInput, int64, time.Time) (cache.MutationResult, error)
	PublishVersion(context.Context, int64, Version, int64, time.Time) (cache.MutationResult, error)
	UpdateStatus(context.Context, int64, yesno.Value, int64, time.Time) (cache.MutationResult, error)
	MarkDeleted(context.Context, int64, int64, time.Time) (cache.MutationResult, error)
}
type Service struct {
	repository  repository
	keys        *secretkey.KeyRing
	generations *cache.Repository
	states      *cache.Store
	cache       *Cache
	now         func() time.Time
}

func NewService(r repository, keys *secretkey.KeyRing) *Service {
	return &Service{repository: r, keys: keys, now: time.Now}
}
func (s *Service) SetGenerations(g *cache.Repository, states *cache.Store) {
	s.generations = g
	s.states = states
}
func (s *Service) SetCache(c *Cache) { s.cache = c }
func (s *Service) ValidateDependencies() error {
	if s == nil || s.repository == nil || s.keys == nil || s.generations == nil || s.states == nil {
		return fmt.Errorf("payment configuration dependencies missing")
	}
	return s.repository.ValidateDependencies()
}
func dependency(err error) error { return apperror.DependencyUnavailable(err) }
func mapError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.NotFound(err)
	case errors.Is(err, ErrIdentityConflict):
		return apperror.Conflict(i18n.KeyConflict, nil, err)
	default:
		var public *apperror.Error
		if errors.As(err, &public) {
			return err
		}
		return dependency(err)
	}
}
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.Result[SafeValue], error) {
	n, err := s.repository.Count(ctx, q)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	rows, err := s.repository.List(ctx, q)
	if err != nil {
		return pagination.Result[SafeValue]{}, mapError(err)
	}
	values := make([]SafeValue, 0, len(rows))
	for _, r := range rows {
		v, err := safeValue(ctx, r)
		if err != nil {
			return pagination.Result[SafeValue]{}, dependency(err)
		}
		values = append(values, v)
	}
	return pagination.Result[SafeValue]{List: values, Total: n, Page: q.Page, PageSize: q.PageSize}, nil
}
func (s *Service) Get(ctx context.Context, id int64) (SafeValue, error) {
	if id < 1 {
		return SafeValue{}, invalid(fmt.Errorf("id invalid"))
	}
	c, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return SafeValue{}, mapError(err)
	}
	v, err := safeValue(ctx, c)
	if err != nil {
		return SafeValue{}, dependency(err)
	}
	return v, nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (int64, error) {
	if err := validateCreate(in); err != nil {
		return 0, invalid(err)
	}
	if s.keys == nil {
		return 0, dependency(fmt.Errorf("payment encryption key missing"))
	}
	methods, err := payment.NormalizePayMethods(in.PayMethods)
	if err != nil {
		return 0, invalid(err)
	}
	credentials := alipay.Credentials{AppID: in.AppID, SellerID: in.SellerID, Environment: in.Environment, PrivateKey: in.PrivateKey, AppCertPEM: in.AppCertPEM, AlipayCertPEM: in.AlipayCertPEM, RootCertPEM: in.RootCertPEM}
	if _, err := alipay.NewClient(ctx, credentials); err != nil {
		return 0, invalid(err)
	}
	ciphertext, err := secretkey.EncryptPaymentValue(s.keys.PaymentEncryptionKey(), strings.TrimSpace(in.PrivateKey))
	if err != nil {
		return 0, dependency(err)
	}
	now := s.now().UTC()
	m := &Model{Name: strings.TrimSpace(in.Name), Provider: in.Provider, Environment: in.Environment, AppID: in.AppID, SellerID: in.SellerID, SortOrder: in.SortOrder, Status: in.Status, CreatedAt: now, UpdatedAt: now}
	v := Version{PrivateKeyCiphertext: ciphertext, AppCertPEM: strings.TrimSpace(in.AppCertPEM), AlipayCertPEM: strings.TrimSpace(in.AlipayCertPEM), RootCertPEM: strings.TrimSpace(in.RootCertPEM), NotifyURL: in.NotifyURL, PayMethods: PayMethods(methods), CreatedAt: now, UpdatedAt: now}
	if err = validateVersionSize(v); err != nil {
		return 0, invalid(err)
	}
	err = s.mutate(ctx, func(work context.Context, base int64) (cache.MutationResult, error) {
		return s.repository.Create(work, m, v, base)
	})
	return m.ID, err
}
func (s *Service) Update(ctx context.Context, id int64, v MetadataInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	if err := validateMetadata(v); err != nil {
		return invalid(err)
	}
	v.Name = strings.TrimSpace(v.Name)
	return s.mutate(ctx, func(work context.Context, base int64) (cache.MutationResult, error) {
		return s.repository.UpdateMetadata(work, id, v, base, s.now().UTC())
	})
}
func (s *Service) credentials(c Current) (alipay.Credentials, error) {
	if s.keys == nil {
		return alipay.Credentials{}, fmt.Errorf("payment encryption key missing")
	}
	key, err := secretkey.DecryptPaymentValue(s.keys.PaymentEncryptionKey(), c.Credential.PrivateKeyCiphertext)
	if err != nil {
		return alipay.Credentials{}, err
	}
	return alipay.Credentials{AppID: c.Config.AppID, SellerID: c.Config.SellerID, NotifyURL: c.Credential.NotifyURL, Environment: c.Config.Environment, PrivateKey: key, AppCertPEM: c.Credential.AppCertPEM, AlipayCertPEM: c.Credential.AlipayCertPEM, RootCertPEM: c.Credential.RootCertPEM}, nil
}
func (s *Service) PublishVersion(ctx context.Context, id int64, in VersionInput) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	methods, err := payment.NormalizePayMethods(in.PayMethods)
	if err != nil {
		return invalid(err)
	}
	if err = validateVersionNotifyURL(in.NotifyURL); err != nil {
		return invalid(err)
	}
	return s.mutate(ctx, func(work context.Context, base int64) (cache.MutationResult, error) {
		c, err := s.repository.FindByID(work, id)
		if err != nil {
			return cache.MutationResult{}, err
		}
		notifyURL := in.NotifyURL
		if !strings.Contains(notifyURL, "{configId}") && !strings.Contains(notifyURL, "{version}") {
			notifyURL, err = notifyTemplate(notifyURL, id, c.Credential.Version)
			if err != nil {
				return cache.MutationResult{}, invalid(errNotifyURLInvalid)
			}
		}
		cred, err := s.credentials(c)
		if err != nil {
			return cache.MutationResult{}, err
		}
		oldKey := cred.PrivateKey
		replace := func(dst *string, src *string) {
			if src != nil {
				*dst = strings.TrimSpace(*src)
			}
		}
		replace(&cred.PrivateKey, in.PrivateKey)
		replace(&cred.AppCertPEM, in.AppCertPEM)
		replace(&cred.AlipayCertPEM, in.AlipayCertPEM)
		replace(&cred.RootCertPEM, in.RootCertPEM)
		if _, err := alipay.NewClient(work, cred); err != nil {
			return cache.MutationResult{}, invalid(err)
		}
		v := c.Credential
		if cred.PrivateKey != oldKey {
			v.PrivateKeyCiphertext, err = secretkey.EncryptPaymentValue(s.keys.PaymentEncryptionKey(), cred.PrivateKey)
			if err != nil {
				return cache.MutationResult{}, err
			}
		}
		v.AppCertPEM = cred.AppCertPEM
		v.AlipayCertPEM = cred.AlipayCertPEM
		v.RootCertPEM = cred.RootCertPEM
		v.NotifyURL = notifyURL
		v.PayMethods = PayMethods(methods)
		if err := validateVersionSize(v); err != nil {
			return cache.MutationResult{}, invalid(err)
		}
		return s.repository.PublishVersion(work, id, v, base, s.now().UTC())
	})
}
func (s *Service) ValidateLocal(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	c, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return mapError(err)
	}
	cred, err := s.credentials(c)
	if err != nil {
		return dependency(err)
	}
	_, err = alipay.NewClient(ctx, cred)
	if err != nil {
		return invalid(err)
	}
	return nil
}
func (s *Service) UpdateStatus(ctx context.Context, id int64, value yesno.Value) error {
	if id < 1 || !yesno.IsValid(value) {
		return invalid(fmt.Errorf("status invalid"))
	}
	return s.mutate(ctx, func(work context.Context, base int64) (cache.MutationResult, error) {
		if value == yesno.Yes {
			if err := s.ValidateLocal(work, id); err != nil {
				return cache.MutationResult{}, err
			}
		}
		return s.repository.UpdateStatus(work, id, value, base, s.now().UTC())
	})
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return invalid(fmt.Errorf("id invalid"))
	}
	return s.mutate(ctx, func(work context.Context, base int64) (cache.MutationResult, error) {
		return s.repository.MarkDeleted(work, id, base, s.now().UTC())
	})
}
func validateVersionSize(v Version) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > MaxSnapshotBytes-4096 {
		return fmt.Errorf("credential snapshot size limit exceeded")
	}
	return nil
}
func (s *Service) mutate(ctx context.Context, apply func(context.Context, int64) (cache.MutationResult, error)) error {
	if err := s.ValidateDependencies(); err != nil {
		return dependency(err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		err := s.mutateOnce(ctx, apply)
		if err == nil {
			return nil
		}
		if !errors.Is(err, cache.ErrUpdating) && !errors.Is(err, cache.ErrGenerationChanged) {
			return mapError(err)
		}
		if !time.Now().Before(deadline) {
			return dependency(err)
		}
		if err := wait(ctx); err != nil {
			return dependency(err)
		}
	}
}
func wait(ctx context.Context) error {
	timer := time.NewTimer(20 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (s *Service) mutateOnce(ctx context.Context, apply func(context.Context, int64) (cache.MutationResult, error)) error {
	base, err := s.generations.Current(ctx, configScope)
	if err != nil {
		return err
	}
	lease, err := s.states.Acquire(ctx, configScope, base)
	if errors.Is(err, cache.ErrStateMissing) || errors.Is(err, cache.ErrStateCorrupt) {
		if _, err = s.states.Reconcile(ctx, configScope, base); err != nil {
			return err
		}
		lease, err = s.states.Acquire(ctx, configScope, base)
	}
	if err != nil {
		return err
	}
	work, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(cache.MutationRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if err := lease.Renew(work); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	result, applyErr := apply(work, base)
	cancel(nil)
	<-done
	if applyErr != nil {
		current, readErr := s.generations.Current(ctx, configScope)
		if readErr != nil {
			return errors.Join(applyErr, readErr)
		}
		if current > base {
			if err := lease.Commit(ctx, current); err != nil {
				return errors.Join(applyErr, err)
			}
			return applyErr
		}
		return errors.Join(applyErr, lease.Rollback(ctx))
	}
	if !result.Changed {
		return lease.Rollback(ctx)
	}
	if err = lease.Commit(ctx, result.Generation); err != nil {
		return committedPending(err)
	}
	if _, err = s.generations.MarkPublishedIfUnclaimed(ctx, result.OutboxID, s.now().UTC()); err != nil {
		return committedPending(err)
	}
	return nil
}
func committedPending(cause error) error {
	return &apperror.Error{HTTPStatus: 503, Code: apperror.CodeDependencyUnavailable, MessageKey: i18n.KeyPaymentConfigCommittedPending, Cause: cause}
}
