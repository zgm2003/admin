package config

import (
	"admin/server/internal/module/payment"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/apperror"
	cache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"math/big"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCommittedMutationReportsPendingAndExistingRelayRecovers(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, r := newServiceTest(t, db, ctx)
	s.SetCache(NewCache(r, s.states))
	id, err := s.Create(ctx, validInput(t))
	if err != nil {
		t.Fatal(err)
	}
	var beforePending int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace=? AND scope_key=? AND published_at IS NULL`, configScope.Namespace, configScope.ScopeKey).Scan(&beforePending).Error; err != nil {
		t.Fatal(err)
	}
	var rejected atomic.Bool
	r.UniversalClient().AddHook(runtimeRaceHook{before: func(_ context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "eval" && len(args) > 1 && strings.Contains(fmt.Sprint(args[1]), "return 'committed'") && rejected.CompareAndSwap(false, true) {
			return errors.New("local simulated Redis publication outage")
		}
		return nil
	}})
	err = s.Update(ctx, id, MetadataInput{Name: "Committed while publication failed"})
	var public *apperror.Error
	if !errors.As(err, &public) || public.MessageKey != i18n.KeyPaymentConfigCommittedPending {
		t.Fatalf("committed update lost public pending semantic: %v", err)
	}
	c, err := s.Get(ctx, id)
	if err != nil || c.Name != "Committed while publication failed" {
		t.Fatal("committed update lost", err)
	}
	var pending int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM system_config_cache_outbox WHERE namespace=? AND scope_key=? AND published_at IS NULL`, configScope.Namespace, configScope.ScopeKey).Scan(&pending).Error; err != nil || pending != beforePending+1 {
		t.Fatalf("durable recovery event missing: %d %v", pending, err)
	}
	if _, err := s.Catalog(ctx); err == nil {
		t.Fatal("invalidating state returned stale configuration")
	}
	// Equivalent to lease expiry, without waiting 30s or changing other scopes.
	if err := r.Delete(ctx, cache.StateKey(configScope)); err != nil {
		t.Fatal(err)
	}
	relay := cache.NewRelay(s.generations, s.states, nil)
	if n, err := relay.RunOnce(ctx); err != nil || int64(n) != pending {
		t.Fatalf("generic worker relay failed: %d %v", n, err)
	}
	items, err := s.Catalog(ctx)
	if err != nil || len(items) != 1 || items[0].Name != c.Name {
		t.Fatalf("relay did not recover latest facts: %+v %v", items, err)
	}
}

func validInput(t *testing.T) CreateInput {
	t.Helper()
	now := time.Now()
	rootKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	appKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	aliKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Local root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, SignatureAlgorithm: x509.SHA256WithRSA}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	cert := func(n int64, cn string, key *rsa.PrivateKey) string {
		c := &x509.Certificate{SerialNumber: big.NewInt(n), Subject: pkix.Name{CommonName: cn}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, SignatureAlgorithm: x509.SHA256WithRSA}
		der, err := x509.CreateCertificate(rand.Reader, c, root, &key.PublicKey, rootKey)
		if err != nil {
			t.Fatal(err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	return CreateInput{Code: "alipay_main", Name: "Local channel", Provider: 1, Environment: 1, AppID: "20260001", SellerID: "20880001", PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(appKey)})), AppCertPEM: cert(2, "20880001-20260001", appKey), AlipayCertPEM: cert(3, "Local Alipay", aliKey), RootCertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), NotifyURL: "https://pay.example.test" + NotifyPathTemplate, PayMethods: []payment.PayMethod{1}}
}

func newServiceTest(t *testing.T, db *gorm.DB, ctx context.Context) (*Service, *projectredis.Client) {
	t.Helper()
	u, err := url.Parse(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/15"
	client, err := projectredis.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	keys, err := secretkey.New(strings.Repeat("p", 64))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	g := cache.NewRepository(db)
	r.SetGenerations(g)
	s := NewService(r, keys)
	s.SetGenerations(g, cache.NewStore(client))
	return s, client
}
func isolateRedisScope(t *testing.T, db *gorm.DB, ctx context.Context) {
	t.Helper()
	old := configScope
	configScope = cache.Scope{Namespace: "payment.config", ScopeKey: fmt.Sprintf("test_%d", time.Now().UnixNano())}
	t.Cleanup(func() { configScope = old })
	if err := db.WithContext(ctx).Exec(`UPDATE system_config_cache_state SET scope_key=? WHERE namespace='payment.config'`, configScope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Exec(`UPDATE system_config_cache_outbox SET scope_key=? WHERE namespace='payment.config'`, configScope.ScopeKey).Error; err != nil {
		t.Fatal(err)
	}
}
func TestServiceCredentialSafetyAndVersionLifecycle(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, redis := newServiceTest(t, db, ctx)
	t.Cleanup(func() { redis.Delete(ctx, cache.StateKey(configScope)) })
	in := validInput(t)
	id, err := s.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{in.PrivateKey, in.AppCertPEM, "privateKeyCiphertext", "appCertPEM"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("credential leaked in management DTO")
		}
	}
	if !got.CredentialConfigured || got.Provider != 1 || got.CurrentVersion != 1 {
		t.Fatalf("safe DTO %+v", got)
	}
	if err = s.Update(ctx, id, MetadataInput{in.Name, 0}); err != nil {
		t.Fatal(err)
	}
	v := VersionInput{NotifyURL: in.NotifyURL, PayMethods: in.PayMethods}
	if err = s.PublishVersion(ctx, id, v); err != nil {
		t.Fatal(err)
	}
	g, _ := s.generations.Current(ctx, configScope)
	if g != 2 {
		t.Fatalf("no-op generation %d", g)
	}
	if err = s.Update(ctx, id, MetadataInput{"Updated", 3}); err != nil {
		t.Fatal(err)
	}
	v.PayMethods = []payment.PayMethod{2}
	if err = s.PublishVersion(ctx, id, v); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, id)
	if got.CurrentVersion != 2 || got.PayMethods[0] != 2 {
		t.Fatalf("version %+v", got)
	}
	if err = s.ValidateLocal(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateStatus(ctx, id, yesno.Yes); err != nil {
		t.Fatal(err)
	}
	bad := in
	bad.Code = "bad"
	bad.PrivateKey = "invalid"
	if _, err = s.Create(ctx, bad); err == nil {
		t.Fatal("bad credentials persisted")
	}
	var n int64
	db.WithContext(ctx).Model(&Model{}).Count(&n)
	if n != 1 {
		t.Fatal("failed create persisted")
	}
	if err = s.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, id); err == nil {
		t.Fatal("deleted channel visible")
	}
	db.WithContext(ctx).Model(&Version{}).Count(&n)
	if n != 2 {
		t.Fatal("history removed")
	}
}
func TestServiceConcurrentPublishAndRollback(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, r1 := newServiceTest(t, db, ctx)
	s2, r2 := newServiceTest(t, db, ctx)
	defer r1.Delete(ctx, cache.StateKey(configScope))
	in := validInput(t)
	id, err := s.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, service := range []*Service{s, s2} {
		wg.Add(1)
		go func(s *Service) {
			defer wg.Done()
			errs <- s.PublishVersion(ctx, id, VersionInput{NotifyURL: in.NotifyURL, PayMethods: []payment.PayMethod{2}})
		}(service)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			// A contender may exhaust the fixed admission budget in a loaded
			// environment. Only that explicit contention error is retryable.
			var public *apperror.Error
			if !errors.As(err, &public) || public.Code != apperror.CodeDependencyUnavailable || (!errors.Is(err, cache.ErrUpdating) && !errors.Is(err, cache.ErrGenerationChanged)) {
				t.Fatalf("unexpected concurrent publication failure: %v cause=%v", err, errors.Unwrap(err))
			}
			if retry := s.PublishVersion(ctx, id, VersionInput{NotifyURL: in.NotifyURL, PayMethods: []payment.PayMethod{2}}); retry != nil {
				t.Fatalf("idempotent retry failed: %v cause=%v", retry, errors.Unwrap(retry))
			}
		}
	}
	value, _ := s.Get(ctx, id)
	if value.CurrentVersion != 2 {
		t.Fatal("duplicate versions on replay")
	}
	before, _ := s.generations.Current(ctx, configScope)
	if _, err = s.Create(ctx, in); err == nil {
		t.Fatal("identity collision accepted")
	}
	after, _ := s.generations.Current(ctx, configScope)
	state, found, err := s.states.Read(ctx, configScope)
	if err != nil || !found || state.Generation != after || before != after || state.State != cache.StateReady {
		t.Fatalf("rollback state %+v %v", state, err)
	}
	r2.Close()
	if err = s2.Update(ctx, id, MetadataInput{"Must not commit", 1}); err == nil {
		t.Fatal("Redis unavailable allowed write")
	}
	value, _ = s.Get(ctx, id)
	if value.Name == "Must not commit" {
		t.Fatal("write leaked past failed lease")
	}
}
