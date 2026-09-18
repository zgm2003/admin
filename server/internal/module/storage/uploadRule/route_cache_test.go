package uploadrule

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database/testquery"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/apperror"
	"admin/server/internal/shared/yesno"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type recordingRouteSource struct {
	mu    sync.Mutex
	route ObjectRoute
	err   error
	calls int
	delay time.Duration
}

func (s *recordingRouteSource) RuleRoute(ctx context.Context, _ int64) (ObjectRoute, error) {
	s.mu.Lock()
	s.calls++
	delay := s.delay
	route := s.route
	err := s.err
	s.mu.Unlock()
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ObjectRoute{}, ctx.Err()
		case <-timer.C:
		}
	}
	return route, err
}

func (s *recordingRouteSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestRouteCacheKeyPayloadAndTTL(t *testing.T) {
	key, err := routeCacheKey(42)
	if err != nil || key != "storage:object-route:v2:rule:42" {
		t.Fatalf("route key = %q, %v", key, err)
	}
	if _, err := routeCacheKey(0); err == nil {
		t.Fatal("zero rule id was accepted")
	}

	route := ObjectRoute{RuleID: 42, PlatformID: 2, CosConfigID: 7, AccessMode: "private"}
	raw, err := encodeRoutePayload(route)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := decodeRoutePayload(raw)
	if err != nil || payload.route() != route {
		t.Fatalf("decoded payload = %+v, %v", payload, err)
	}

	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	cleanupRouteCacheKey(t, client, route.RuleID)
	if err := cache.PutRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	ttl, found, err := client.TTL(context.Background(), key)
	if err != nil || !found || ttl < 23*time.Hour || ttl > routeSnapshotTTL {
		t.Fatalf("route ttl = %v, found=%v, err=%v", ttl, found, err)
	}
	got, found, err := cache.GetRoute(context.Background(), route.RuleID)
	if err != nil || !found || got != route {
		t.Fatalf("cached route = %+v, found=%v, err=%v", got, found, err)
	}
}

func TestRouteSnapshotCodecRejectsMalformedPayloads(t *testing.T) {
	valid := `{"schemaVersion":1,"ruleId":7,"platformId":2,"cosConfigId":3,"accessMode":"public"}`
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"not object", `[]`},
		{"unknown field", `{"schemaVersion":1,"ruleId":7,"platformId":2,"cosConfigId":3,"accessMode":"public","extra":1}`},
		{"duplicate field", `{"schemaVersion":1,"ruleId":7,"ruleId":8,"platformId":2,"cosConfigId":3,"accessMode":"public"}`},
		{"trailing json", valid + ` {}`},
		{"wrong schema", `{"schemaVersion":2,"ruleId":7,"platformId":2,"cosConfigId":3,"accessMode":"public"}`},
		{"missing field", `{"schemaVersion":1,"ruleId":7,"platformId":2,"cosConfigId":3}`},
		{"zero rule", `{"schemaVersion":1,"ruleId":0,"platformId":2,"cosConfigId":3,"accessMode":"public"}`},
		{"zero platform", `{"schemaVersion":1,"ruleId":7,"platformId":0,"cosConfigId":3,"accessMode":"public"}`},
		{"zero config", `{"schemaVersion":1,"ruleId":7,"platformId":2,"cosConfigId":0,"accessMode":"public"}`},
		{"bad access mode", `{"schemaVersion":1,"ruleId":7,"platformId":2,"cosConfigId":3,"accessMode":"shared"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeRoutePayload(test.raw); !errors.Is(err, ErrRouteSnapshotCorrupt) {
				t.Fatalf("decode error = %v, want ErrRouteSnapshotCorrupt", err)
			}
		})
	}
	if _, err := encodeRoutePayload(ObjectRoute{RuleID: 1, PlatformID: 1, CosConfigID: 1, AccessMode: "shared"}); err == nil {
		t.Fatal("invalid route was encoded")
	}
}

func TestRouteCacheHitDoesNotReadPostgreSQL(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	route := ObjectRoute{RuleID: 910001, PlatformID: 1, CosConfigID: 2, AccessMode: "public"}
	cleanupRouteCacheKey(t, client, route.RuleID)
	if err := cache.PutRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	source := &recordingRouteSource{err: errors.New("PostgreSQL must not be called")}
	service := newRouteTestService(cache, source)
	got, err := service.loadRoute(context.Background(), route.RuleID)
	if err != nil || got != route {
		t.Fatalf("load route = %+v, %v", got, err)
	}
	if calls := source.callCount(); calls != 0 {
		t.Fatalf("source calls = %d, want 0", calls)
	}
}

func TestRouteCacheMissUsesOneLeaderAndFillsFollowers(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	route := ObjectRoute{RuleID: 910002, PlatformID: 2, CosConfigID: 3, AccessMode: "private"}
	cleanupRouteCacheKey(t, client, route.RuleID)
	source := &recordingRouteSource{route: route, delay: 80 * time.Millisecond}

	const readers = 8
	start := make(chan struct{})
	errorsByReader := make([]error, readers)
	routes := make([]ObjectRoute, readers)
	var wait sync.WaitGroup
	for index := 0; index < readers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			routes[index], errorsByReader[index] = newRouteTestService(cache, source).loadRoute(context.Background(), route.RuleID)
		}(index)
	}
	close(start)
	wait.Wait()
	for index := range routes {
		if errorsByReader[index] != nil || routes[index] != route {
			t.Fatalf("reader %d route = %+v, err=%v", index, routes[index], errorsByReader[index])
		}
	}
	if calls := source.callCount(); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}
}

func TestRouteCacheColdAndHotPostgresSelectBudgets(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	counter := testquery.New(db.Logger, "storage_upload_rule")
	db = db.Session(&gorm.Session{Logger: counter})
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	ruleID, err := NewService(NewRepository(db), nil, nil, nil, nil).Create(ctx, validCreate(platformID, configID, []string{"avatar"}, yesno.Yes))
	if err != nil {
		t.Fatal(err)
	}
	client := testRouteRedisClient(t)
	cleanupRouteCacheKey(t, client, ruleID)
	service := newRouteTestService(NewRouteCache(client), NewRepository(db))

	counter.Reset()
	if _, err := service.loadRoute(ctx, ruleID); err != nil {
		t.Fatal(err)
	}
	if got := counter.Count("storage_upload_rule"); got != 1 || counter.Total() != 1 {
		t.Fatalf("cold route SELECT budget: route=%d total=%d, want 1/1", got, counter.Total())
	}
	counter.Reset()
	if _, err := service.loadRoute(ctx, ruleID); err != nil {
		t.Fatal(err)
	}
	if got := counter.Total(); got != 0 {
		t.Fatalf("hot route SELECT budget = %d, want 0", got)
	}
}

func TestRouteFillSkipsSourceWhenAnotherInstanceFilledBeforeLease(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	route := ObjectRoute{RuleID: 910007, PlatformID: 2, CosConfigID: 3, AccessMode: "private"}
	cleanupRouteCacheKey(t, client, route.RuleID)
	if err := cache.PutRoute(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	source := &recordingRouteSource{err: errors.New("another source load must not run")}
	if err := newRouteTestService(cache, source).fillRoute(context.Background(), route.RuleID); err != nil {
		t.Fatalf("fillRoute() error = %v", err)
	}
	if calls := source.callCount(); calls != 0 {
		t.Fatalf("source calls = %d want 0", calls)
	}
}

func TestCorruptRouteSnapshotIsRebuilt(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	route := ObjectRoute{RuleID: 910003, PlatformID: 3, CosConfigID: 4, AccessMode: "public"}
	cleanupRouteCacheKey(t, client, route.RuleID)
	key, _ := routeCacheKey(route.RuleID)
	if err := client.SetString(context.Background(), key, `{"schemaVersion":1,"ruleId":910003,"platformId":3,"cosConfigId":4,"accessMode":"public","extra":true}`, time.Hour); err != nil {
		t.Fatal(err)
	}
	source := &recordingRouteSource{route: route}
	got, err := newRouteTestService(cache, source).loadRoute(context.Background(), route.RuleID)
	if err != nil || got != route {
		t.Fatalf("rebuilt route = %+v, %v", got, err)
	}
	if calls := source.callCount(); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}
}

func TestRouteCacheRedisErrorFailsClosedWithoutPostgreSQL(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	source := &recordingRouteSource{route: ObjectRoute{RuleID: 910004, PlatformID: 1, CosConfigID: 1, AccessMode: "private"}}
	_, err := newRouteTestService(cache, source).loadRoute(context.Background(), 910004)
	if appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("load error = %v", err)
	}
	if calls := source.callCount(); calls != 0 {
		t.Fatalf("source calls = %d, want 0", calls)
	}
}

func TestRouteCacheFollowerStopsAtReadBudget(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	const ruleID int64 = 910005
	cleanupRouteCacheKey(t, client, ruleID)
	lease, err := cache.TryFill(context.Background(), ruleID)
	if err != nil || lease == nil {
		t.Fatalf("leader lease = %v, %v", lease, err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	start := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	current := start
	waits := 0
	source := &recordingRouteSource{err: errors.New("follower must not read PostgreSQL")}
	service := newRouteTestService(cache, source)
	service.now = func() time.Time { return current }
	service.wait = func(_ context.Context, duration time.Duration) error {
		waits++
		current = current.Add(duration)
		return nil
	}

	_, err = service.loadRoute(context.Background(), ruleID)
	if appCode(err) != apperror.CodeDependencyUnavailable {
		t.Fatalf("load error = %v", err)
	}
	if elapsed := current.Sub(start); elapsed != routeReadBudgetDefault {
		t.Fatalf("logical wait = %v, want %v", elapsed, routeReadBudgetDefault)
	}
	if waits != int(routeReadBudgetDefault/routeReadWaitStep) {
		t.Fatalf("wait calls = %d", waits)
	}
	if calls := source.callCount(); calls != 0 {
		t.Fatalf("source calls = %d, want 0", calls)
	}
}

func TestRepositoryRuleRouteIncludesSoftDeletedRule(t *testing.T) {
	db, ctx := openRuleDatabase(t)
	platformID := insertPlatform(t, db, ctx, "admin", yesno.Yes)
	configID := insertConfig(t, db, ctx, "main", yesno.Yes, "")
	service := NewService(NewRepository(db), nil, nil, nil, nil)
	ruleID, err := service.Create(ctx, validCreate(platformID, configID, []string{"avatar"}, yesno.No))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx).Model(&Model{}).Where("id = ?", ruleID).Update("deleted_at", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	route, err := NewRepository(db).RuleRoute(ctx, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	want := ObjectRoute{RuleID: ruleID, PlatformID: platformID, CosConfigID: configID, AccessMode: "private"}
	if route != want {
		t.Fatalf("route = %+v, want %+v", route, want)
	}
	if _, err := NewRepository(db).RuleRoute(ctx, ruleID+1000); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing route error = %v", err)
	}
}

func TestMissingRouteReturnsNotFound(t *testing.T) {
	client := testRouteRedisClient(t)
	cache := NewRouteCache(client)
	const ruleID int64 = 910006
	cleanupRouteCacheKey(t, client, ruleID)
	source := &recordingRouteSource{err: gorm.ErrRecordNotFound}
	_, err := newRouteTestService(cache, source).loadRoute(context.Background(), ruleID)
	if appCode(err) != apperror.CodeNotFound {
		t.Fatalf("load error = %v", err)
	}
}

func newRouteTestService(cache *RouteCache, source routeSource) *Service {
	service := NewService(nil, nil, nil, nil, cache)
	service.routeSource = source
	return service
}

func testRouteRedisClient(t *testing.T) *projectredis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("Redis integration test")
	}
	if err := godotenv.Load("../../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(settings.RedisURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/3"
	client, err := projectredis.Open(context.Background(), parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func cleanupRouteCacheKey(t *testing.T, client *projectredis.Client, ruleID int64) {
	t.Helper()
	key, err := routeCacheKey(ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Delete(context.Background(), key) })
}
