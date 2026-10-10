// One-time forward maintenance. No application startup invokes this command.
package main

import (
	"admin/server/internal/config"
	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const migrationID = "2026-10-10-remove-system-dictionary"

var cleanupPatterns = []string{
	"system:dictionary:generation:v1",
	"system:dictionary:mutation:v1",
	"system:dictionary:options:v1:*",
	"config-cache:state:v1:system.dictionary:global",
	"config-cache:snapshot:v1:system.dictionary:global:*",
	"config-cache:fill:v1:system.dictionary:global:*",
}

type platformFact struct {
	ID       int64 `json:"id"`
	Version  int64 `json:"version"`
	Affected bool  `json:"affected"`
}
type manifest struct {
	Migration  string         `json:"migration"`
	Database   string         `json:"database"`
	SQLHash    string         `json:"sqlHash"`
	CacheFacts string         `json:"preservedCacheFactsHash"`
	Platforms  []platformFact `json:"platforms"`
	Committed  bool           `json:"postgresCommitted"`
	Verified   bool           `json:"verified"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

func validateManifest(value manifest, database string) error {
	if value.Migration != migrationID || value.Database != database {
		return fmt.Errorf("audit manifest belongs to a different migration or database")
	}
	for _, p := range value.Platforms {
		if p.ID < 1 || p.Version < 1 {
			return fmt.Errorf("audit manifest has invalid platform versions")
		}
	}
	return nil
}
func saveManifest(path string, value *manifest) error {
	value.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(payload)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// Offline repair is monotonic and never overwrites a live tokenized lease.
// Missing state is installed; corrupt, newer, and invalidating states fail closed.
const reconcileMenuScript = `
local raw=redis.call('GET',KEYS[1])
if raw then
 local ok,current=pcall(cjson.decode,raw)
 if not ok or type(current)~='table' or current.schemaVersion~=2 then return 'corrupt' end
 if current.state=='invalidating' then return 'invalidating' end
 if current.state~='ready' or type(current.version)~='number' or current.version<1 or current.mutationToken or current.baseVersion then return 'corrupt' end
 if current.version>tonumber(ARGV[1]) then return 'newer' end
end
redis.call('SET',KEYS[1],ARGV[2])
return 'ready'
`

func reconcileMenu(ctx context.Context, redis *projectredis.Client, fact platformFact) error {
	state := permissionstate.State{SchemaVersion: permissionstate.SchemaVersion, State: permissionstate.StateReady, Version: fact.Version}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	result, err := redis.UniversalClient().Eval(ctx, reconcileMenuScript, []string{permissionstate.MenuStateKey(fact.ID)}, fact.Version, string(raw)).Text()
	if err != nil {
		return fmt.Errorf("menu state publication unavailable for platform %d", fact.ID)
	}
	if result != "ready" {
		return fmt.Errorf("menu state platform %d: %s; retry after any existing lease expires", fact.ID, result)
	}
	return nil
}
func readPlatforms(ctx context.Context, conn *pgx.Conn) ([]platformFact, error) {
	rows, err := conn.Query(ctx, `SELECT p.id,p.menu_version,EXISTS(SELECT 1 FROM permission_menu m WHERE m.platform_id=p.id AND (m.code LIKE 'system:dictionary:%' OR m.path='/system/dictionary' OR m.component_path='system/dictionary')) FROM permission_auth_platform p ORDER BY p.id`)
	if err != nil {
		return nil, fmt.Errorf("read platform catalog failed")
	}
	defer rows.Close()
	facts := make([]platformFact, 0)
	for rows.Next() {
		var f platformFact
		if err = rows.Scan(&f.ID, &f.Version, &f.Affected); err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}
func preservedCacheFacts(ctx context.Context, conn *pgx.Conn) (string, error) {
	var generations, outbox string
	err := conn.QueryRow(ctx, `SELECT
 coalesce((SELECT jsonb_agg(to_jsonb(t) ORDER BY namespace,scope_key)::text FROM system_config_cache_generation t WHERE NOT(namespace='system.dictionary' AND scope_key='global')),'[]'),
 coalesce((SELECT jsonb_agg(to_jsonb(t) ORDER BY id)::text FROM system_config_cache_outbox t WHERE NOT(namespace='system.dictionary' AND scope_key='global')),'[]')`).Scan(&generations, &outbox)
	if err != nil {
		return "", fmt.Errorf("read preserved generation/outbox audit failed")
	}
	hash := sha256.Sum256([]byte(generations + "\n" + outbox))
	return hex.EncodeToString(hash[:]), nil
}

func verifyPostgres(ctx context.Context, conn *pgx.Conn, before []platformFact) ([]platformFact, error) {
	current, err := readPlatforms(ctx, conn)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]platformFact, len(current))
	for _, f := range current {
		byID[f.ID] = f
	}
	if len(current) != len(before) {
		return nil, fmt.Errorf("platform set changed during offline maintenance")
	}
	for _, f := range before {
		now, ok := byID[f.ID]
		expected := f.Version
		if f.Affected {
			expected++
		}
		if !ok || now.Version != expected || now.Affected {
			return nil, fmt.Errorf("menu version verification failed for platform %d", f.ID)
		}
	}
	var clean bool
	err = conn.QueryRow(ctx, `SELECT to_regclass('system_dictionary') IS NULL AND to_regclass('system_dictionary_item') IS NULL AND NOT EXISTS(SELECT 1 FROM system_config_cache_generation WHERE namespace='system.dictionary' AND scope_key='global') AND NOT EXISTS(SELECT 1 FROM system_config_cache_outbox WHERE namespace='system.dictionary' AND scope_key='global')`).Scan(&clean)
	if err != nil || !clean {
		return nil, fmt.Errorf("dictionary PostgreSQL removal verification failed")
	}
	return current, nil
}
func cleanupDictionary(ctx context.Context, redis *projectredis.Client) error {
	for _, pattern := range cleanupPatterns {
		if err := redis.ScanDelete(ctx, pattern); err != nil {
			return fmt.Errorf("dictionary Redis cleanup failed for fixed pattern %s", pattern)
		}
		var cursor uint64
		for {
			keys, next, err := redis.UniversalClient().Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return fmt.Errorf("dictionary Redis verification failed for fixed pattern %s", pattern)
			}
			if len(keys) != 0 {
				return fmt.Errorf("dictionary Redis keys remain for fixed pattern %s", pattern)
			}
			if next == 0 {
				break
			}
			cursor = next
		}
		fmt.Printf("Redis cleanup pattern=%s remaining=0\n", pattern)
	}
	return nil
}
func run(ctx context.Context, settings config.Worker, sqlPath, auditPath string) (resultErr error) {
	script, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read forward SQL failed")
	}
	hash := sha256.Sum256(script)
	sqlHash := hex.EncodeToString(hash[:])
	conn, err := pgx.Connect(ctx, settings.PostgresDSN)
	if err != nil {
		return fmt.Errorf("PostgreSQL connection failed; credentials omitted")
	}
	defer conn.Close(ctx)
	// Serialize command invocations before creating/reading the durable manifest.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(20261010,1002)"); err != nil {
		return fmt.Errorf("acquire maintenance lock failed")
	}
	var identity string
	if err = conn.QueryRow(ctx, `SELECT current_database()||':'||coalesce(inet_server_addr()::text,'local')||':'||coalesce(inet_server_port()::text,'local')||':'||current_schema()`).Scan(&identity); err != nil {
		return fmt.Errorf("read database identity failed")
	}
	databaseHash := sha256.Sum256([]byte(identity))
	databaseID := hex.EncodeToString(databaseHash[:])
	audit := manifest{Migration: migrationID, Database: databaseID, SQLHash: sqlHash}
	data, readErr := os.ReadFile(auditPath)
	if readErr == nil {
		if err = json.Unmarshal(data, &audit); err != nil {
			return fmt.Errorf("audit manifest is malformed")
		}
		if err = validateManifest(audit, databaseID); err != nil {
			return err
		}
		if audit.SQLHash != sqlHash {
			return fmt.Errorf("forward SQL differs from audited SQL; retain original migration artifact")
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("read audit manifest failed")
	}
	defer func() {
		if resultErr != nil && audit.Committed && !strings.Contains(resultErr.Error(), "PostgreSQL 已提交") {
			resultErr = fmt.Errorf("PostgreSQL 已提交、Redis cleanup/verification 未完成: %w", resultErr)
		}
	}()
	current, err := readPlatforms(ctx, conn)
	if err != nil {
		return err
	}
	if os.IsNotExist(readErr) {
		audit.Platforms = current
		audit.CacheFacts, err = preservedCacheFacts(ctx, conn)
		if err != nil {
			return err
		}
		if err = saveManifest(auditPath, &audit); err != nil {
			return fmt.Errorf("persist pre-migration audit failed")
		}
	}
	redis, err := projectredis.Open(ctx, settings.RedisURL)
	if err != nil {
		return fmt.Errorf("Redis connection failed; PostgreSQL was not changed by this invocation")
	}
	defer redis.Close()
	// A crashed earlier invocation may have committed PG. Repair only audited targets,
	// using the current PG version; the Lua refuses active leases/newer/corrupt states.
	targets := make(map[int64]bool)
	for _, f := range audit.Platforms {
		if f.Affected {
			targets[f.ID] = true
		}
	}
	for _, f := range current {
		if targets[f.ID] {
			if err = reconcileMenu(ctx, redis, f); err != nil {
				return err
			}
		}
	}
	menuStore := permissionstate.NewMenuStore(redis)
	type held struct {
		fact  platformFact
		lease *permissionstate.MenuLease
		stop  func()
	}
	leases := make([]held, 0)
	committed := false
	baseCtx := ctx
	defer func() {
		for _, entry := range leases {
			entry.stop()
			if !committed {
				resultErr = errors.Join(resultErr, entry.lease.Rollback(baseCtx))
			}
		}
	}()
	for _, f := range current {
		if !f.Affected {
			continue
		}
		lease, acquireErr := menuStore.Acquire(ctx, f.ID, f.Version)
		if acquireErr != nil {
			return fmt.Errorf("acquire platform %d menu lease failed", f.ID)
		}
		renewed, stop := lease.StartRenewal(ctx)
		ctx = renewed
		leases = append(leases, held{f, lease, stop})
	}
	if _, err = conn.Exec(ctx, string(script)); err != nil {
		return fmt.Errorf("PostgreSQL migration failed or commit outcome uncertain; rerun the same audited runner before starting services")
	}
	committed = true
	audit.Committed = true
	audit.Verified = false
	if err = saveManifest(auditPath, &audit); err != nil {
		return fmt.Errorf("PostgreSQL 已提交、audit/Redis cleanup 未完成")
	}
	fmt.Println("PostgreSQL 已提交；正在衔接菜单 state 和定向清理 Redis")
	cacheFacts, cacheErr := preservedCacheFacts(ctx, conn)
	if cacheErr != nil || cacheFacts != audit.CacheFacts {
		return fmt.Errorf("PostgreSQL 已提交、verification/Redis cleanup 未完成: unrelated generation/outbox facts changed")
	}
	after, err := verifyPostgres(ctx, conn, audit.Platforms)
	if err != nil {
		return fmt.Errorf("PostgreSQL 已提交、verification/Redis cleanup 未完成: %w", err)
	}
	for _, entry := range leases {
		if err = entry.lease.Commit(ctx, entry.fact.Version+1); err != nil {
			return fmt.Errorf("PostgreSQL 已提交、Redis cleanup 未完成: platform %d menu state publication failed", entry.fact.ID)
		}
	}
	for _, f := range after {
		if !targets[f.ID] {
			continue
		}
		if err = reconcileMenu(ctx, redis, f); err != nil {
			return fmt.Errorf("PostgreSQL 已提交、Redis cleanup 未完成: %w", err)
		}
		state, found, readErr := menuStore.Read(ctx, f.ID)
		if readErr != nil || !found || state.State != permissionstate.StateReady || state.Version != f.Version {
			return fmt.Errorf("PostgreSQL 已提交、Redis cleanup 未完成: platform %d menu state verification failed", f.ID)
		}
	}
	if err = cleanupDictionary(ctx, redis); err != nil {
		return fmt.Errorf("PostgreSQL 已提交、Redis cleanup 未完成: %w", err)
	}
	audit.Verified = true
	if err = saveManifest(auditPath, &audit); err != nil {
		return fmt.Errorf("cleanup completed but audit verification write failed")
	}
	fmt.Println("Verified: dictionary tables/menus/global generation removed; menu versions/state synchronized; fixed Redis patterns empty")
	return nil
}
func main() {
	sqlPath := flag.String("sql", "", "forward SQL path")
	auditPath := flag.String("audit", "", "durable audit manifest path")
	stopped := flag.Bool("old-services-stopped", false, "required offline maintenance acknowledgement")
	flag.Parse()
	if !*stopped || *sqlPath == "" || *auditPath == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "require -old-services-stopped -sql PATH -audit PATH")
		os.Exit(1)
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "load maintenance environment failed")
		os.Exit(1)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "maintenance PostgreSQL/Redis configuration is invalid")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = run(ctx, settings, *sqlPath, *auditPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
