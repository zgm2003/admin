package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"admin/server/internal/module/payment"
	cache "admin/server/internal/shared/configCache"
)

func TestCallbackAddressLifecycleOnCurrentSchema(t *testing.T) {
	// Exercise the current PostgreSQL contract without replaying historical DDL.
	db, ctx := configFixture(t, false)
	raw, err := os.ReadFile("../../../../../docs/database/current.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range []string{
		`(?s)CREATE FUNCTION public\.payment_config(?:_version)?_guard\(\).*?END\s*\$\$;`,
		`(?s)CREATE TABLE public\.payment_config(?:_version)? \(.*?\n\);`,
		`(?s)ALTER TABLE public\.payment_config ALTER COLUMN id ADD GENERATED.*?\n\);`,
		`(?s)ALTER TABLE ONLY public\.payment_config(?:_version)?\s+ADD CONSTRAINT.*?;`,
		`CREATE UNIQUE INDEX ux_payment_config_app_active .*?;`,
		`CREATE TRIGGER payment_config(?:_version)?_[^\r\n]+;`,
	} {
		matches := regexp.MustCompile(pattern).FindAllString(string(raw), -1)
		if len(matches) == 0 {
			t.Fatalf("current schema block missing: %s", pattern)
		}
		for _, sql := range matches {
			if err := db.WithContext(ctx).Exec(strings.ReplaceAll(sql, "public.", "")).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO system_config_cache_state VALUES('payment.config','global',2,now(),now())`).Error; err != nil {
		t.Fatal(err)
	}
	isolateRedisScope(t, db, ctx)
	s, redis := newServiceTest(t, db, ctx)
	t.Cleanup(func() { redis.Delete(ctx, cache.StateKey(configScope)) })
	in := validInput(t)
	in.NotifyURL = "https://pay.example.test/api/admin/v1/payment/callback/alipay"
	id, err := s.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("https://pay.example.test/api/admin/v1/payment/callback/alipay/%d/1", id)
	got, err := s.Get(ctx, id)
	if err != nil || got.NotifyURL != want {
		t.Fatalf("created callback %q: %v", got.NotifyURL, err)
	}
	v := VersionInput{NotifyURL: got.NotifyURL, PayMethods: in.PayMethods}
	if err := s.PublishVersion(ctx, id, v); err != nil {
		t.Fatalf("actual URL no-op: %v", err)
	}
	generation, err := s.generations.Current(ctx, configScope)
	if err != nil || generation != 3 {
		t.Fatalf("no-op changed generation: %d %v", generation, err)
	}
	v.PayMethods = []payment.PayMethod{2}
	if err := s.PublishVersion(ctx, id, v); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, id)
	if err != nil || got.CurrentVersion != 2 || got.NotifyURL != fmt.Sprintf("https://pay.example.test/api/admin/v1/payment/callback/alipay/%d/2", id) {
		t.Fatalf("rotated callback %q: %v", got.NotifyURL, err)
	}
	v.NotifyURL = strings.Replace(got.NotifyURL, "pay.example.test", "new.example.test:8443", 1)
	if err := s.PublishVersion(ctx, id, v); err != nil {
		t.Fatalf("domain change: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil || got.CurrentVersion != 3 || got.NotifyURL != fmt.Sprintf("https://new.example.test:8443/api/admin/v1/payment/callback/alipay/%d/3", id) {
		t.Fatalf("new domain callback %q: %v", got.NotifyURL, err)
	}
	var original Version
	if err := db.WithContext(ctx).Where("config_id=? AND version=1", id).Take(&original).Error; err != nil || original.NotifyURL != want {
		t.Fatalf("old callback changed: %v", err)
	}
	for _, raw := range []string{
		fmt.Sprintf("https://new.example.test:8443/api/admin/v1/payment/callback/alipay/%d/3", id+1),
		fmt.Sprintf("https://new.example.test:8443/api/admin/v1/payment/callback/alipay/%d/2", id),
	} {
		v.NotifyURL = raw
		if err := s.PublishVersion(ctx, id, v); err == nil {
			t.Fatal("incorrect callback coordinates accepted")
		}
	}
	var count int64
	if err := db.WithContext(ctx).Table("payment_config_version").Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("invalid input wrote version: %d %v", count, err)
	}
}
