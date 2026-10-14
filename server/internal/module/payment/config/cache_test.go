package config

import (
	cache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	redis "github.com/redis/go-redis/v9"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type runtimeRaceHook struct {
	before func(context.Context, redis.Cmder) error
}

func (h runtimeRaceHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return next(ctx, network, address)
	}
}
func (h runtimeRaceHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if err := h.before(ctx, cmd); err != nil {
			return err
		}
		return next(ctx, cmd)
	}
}
func (h runtimeRaceHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func TestRuntimeRepairNeverReturnsAConcurrentOldGeneration(t *testing.T) {
	for _, at := range []string{"reconciliation", "snapshot recheck"} {
		t.Run(at, func(t *testing.T) {
			db, ctx := configDB(t)
			isolateRedisScope(t, db, ctx)
			s, r1 := newServiceTest(t, db, ctx)
			s2, r2 := newServiceTest(t, db, ctx)
			s.SetCache(NewCache(r1, s.states))
			s2.SetCache(NewCache(r2, s2.states))
			id, err := s.Create(ctx, validInput(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Catalog(ctx); err != nil {
				t.Fatal(err)
			}
			g, err := s.generations.Current(ctx, configScope)
			if err != nil {
				t.Fatal(err)
			}
			oldKey, _ := cache.SnapshotKey(configScope, g, "catalog")
			if err := r1.Delete(ctx, cache.StateKey(configScope)); err != nil {
				t.Fatal(err)
			}
			var triggered atomic.Bool
			r1.UniversalClient().AddHook(runtimeRaceHook{before: func(work context.Context, cmd redis.Cmder) error {
				args := cmd.Args()
				match := at == "reconciliation" && cmd.Name() == "eval" && strings.Contains(fmt.Sprint(args[1]), "already-newer")
				if at == "snapshot recheck" {
					match = cmd.Name() == "get" && len(args) > 1 && fmt.Sprint(args[1]) == oldKey
				}
				if match && triggered.CompareAndSwap(false, true) {
					return s2.Update(work, id, MetadataInput{Name: "Updated by another instance"})
				}
				return nil
			}})
			items, err := s.Catalog(ctx)
			if err != nil || len(items) != 1 || items[0].Name != "Updated by another instance" || !triggered.Load() {
				t.Fatalf("returned stale repair facts: %+v %v race=%v", items, err, triggered.Load())
			}
		})
	}
}

func TestSnapshotRejectsIncompleteOrInvalidFacts(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, r := newServiceTest(t, db, ctx)
	c := NewCache(r, s.states)
	s.SetCache(c)
	id, err := s.Create(ctx, validInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Credential(ctx, id, 1); err != nil {
		t.Fatal(err)
	}
	g, _ := s.generations.Current(ctx, configScope)
	variant := credentialVariant(id, 1)
	key, _ := cache.SnapshotKey(configScope, g, variant)
	original, _, _ := r.GetString(ctx, key)
	for name, mutate := range map[string]func(map[string]any){
		"missing app":     func(p map[string]any) { delete(p["credential"].(map[string]any), "appId") },
		"null seller":     func(p map[string]any) { p["credential"].(map[string]any)["sellerId"] = nil },
		"bad seller":      func(p map[string]any) { p["credential"].(map[string]any)["sellerId"] = "merchant" },
		"missing catalog": func(p map[string]any) { delete(p, "catalog") },
		"bad notify URL": func(p map[string]any) {
			v := p["credential"].(map[string]any)["version"].(map[string]any)
			for k := range v {
				if strings.EqualFold(k, "notifyUrl") {
					v[k] = fmt.Sprintf("http://user:secret@pay.example.test/api/v1/payment/callback/alipay/%d/1?token=secret", id)
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			var p map[string]any
			json.Unmarshal([]byte(original), &p)
			mutate(p)
			bad, _ := json.Marshal(p)
			r.SetString(ctx, key, string(bad), snapshotTTL)
			if _, hit, err := c.get(ctx, g, variant); hit || !errors.Is(err, errSnapshotCorrupt) {
				t.Fatalf("corrupt hit=%v err=%v", hit, err)
			}
		})
	}
	duplicate := strings.Replace(original, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1)
	r.SetString(ctx, key, duplicate, snapshotTTL)
	if _, hit, err := c.get(ctx, g, variant); hit || !errors.Is(err, errSnapshotCorrupt) {
		t.Fatal("duplicate field accepted")
	}
}

type delayedCatalogRepository struct {
	repository
	delay time.Duration
}

func (r delayedCatalogRepository) CatalogFacts(ctx context.Context) ([]CatalogItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(r.delay):
		return r.repository.CatalogFacts(ctx)
	}
}
func TestRuntimeLeaderRetainsWorkBudgetAndFollowerIsBounded(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, r := newServiceTest(t, db, ctx)
	s.SetCache(NewCache(r, s.states))
	if _, err := s.Create(ctx, validInput(t)); err != nil {
		t.Fatal(err)
	}
	s.repository = delayedCatalogRepository{s.repository, 650 * time.Millisecond}
	items, err := s.Catalog(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("successful slow leader rejected: %v", err)
	}
	g, _ := s.generations.Current(ctx, configScope)
	key, _ := cache.SnapshotKey(configScope, g, "catalog")
	r.Delete(ctx, key)
	lease, err := s.cache.tryFill(ctx, g, "catalog", false)
	if err != nil || lease == nil {
		t.Fatal(err)
	}
	defer lease.Release(ctx)
	start := time.Now()
	if _, err := s.Catalog(ctx); err == nil {
		t.Fatal("follower accepted absent snapshot")
	}
	if time.Since(start) > 850*time.Millisecond {
		t.Fatal("follower budget unbounded")
	}
}

func TestCatalogWriteBudgetMatchesSnapshotEncoding(t *testing.T) {
	db, ctx := configDB(t)
	if err := db.WithContext(ctx).Exec(`INSERT INTO payment_config(name,provider,environment,app_id,seller_id,current_version) SELECT repeat('&',128),1,1,n::text,'20880001',1 FROM generate_series(1,349)n; INSERT INTO payment_config_version(config_id,version,private_key_ciphertext,app_cert_pem,alipay_cert_pem,root_cert_pem,notify_url,pay_methods) SELECT id,1,'x','a','b','c','https://pay.example.test/api/v1/payment/callback/alipay/'||id||'/1',ARRAY[1]::smallint[] FROM payment_config`).Error; err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	r.SetGenerations(cache.NewRepository(db))
	m, v := testRecord()
	m.Name = strings.Repeat("&", 128)
	if _, err := r.Create(ctx, m, v, 1); err == nil {
		t.Fatal("write accepted catalog that cannot be encoded below 256KiB")
	}
	var n int64
	db.WithContext(ctx).Model(&Model{}).Count(&n)
	if n != 349 {
		t.Fatal("oversize write did not roll back")
	}
}

func TestRuntimeCacheCrossInstanceAndFailureClosed(t *testing.T) {
	db, ctx := configDB(t)
	isolateRedisScope(t, db, ctx)
	s, r1 := newServiceTest(t, db, ctx)
	s2, r2 := newServiceTest(t, db, ctx)
	c := NewCache(r1, s.states)
	s.SetCache(c)
	s2.SetCache(NewCache(r2, s2.states))
	in := validInput(t)
	in.IsEnabled = yesno.Yes
	id, err := s.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Delete(ctx, cache.StateKey(configScope))
	items, err := s.Catalog(ctx)
	if err != nil || len(items) != 1 || items[0].ID != id {
		t.Fatalf("catalog %+v %v", items, err)
	}
	credential, err := s2.Credential(ctx, id, 1)
	if err != nil || credential.Version.ConfigID != id {
		t.Fatalf("credential %+v %v cause=%v", credential, err, errors.Unwrap(err))
	}
	g, _ := s.generations.Current(ctx, configScope)
	key, _ := cache.SnapshotKey(configScope, g, credentialVariant(id, 1))
	raw, found, err := r1.GetString(ctx, key)
	if err != nil || !found || strings.Contains(raw, in.PrivateKey) {
		t.Fatal("plaintext key in Redis")
	}
	if err := r1.SetString(ctx, key, `{"schemaVersion":999}`, snapshotTTL); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Credential(ctx, id, 1); err != nil {
		t.Fatalf("corrupt snapshot repair: %v", err)
	}
	if err = s.UpdateStatus(ctx, id, yesno.No); err != nil {
		t.Fatal(err)
	}
	items, err = s2.Catalog(ctx)
	if err != nil || len(items) != 1 || items[0].IsEnabled != 0 {
		t.Fatal("stale catalog")
	}
	r2.Close()
	if _, err = s2.Catalog(ctx); err == nil {
		t.Fatal("Redis unavailable returned stale data")
	}
	r1.Delete(ctx, key)
	ctx2, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.Catalog(ctx2); err == nil {
		t.Fatal("canceled read accepted")
	}
}
