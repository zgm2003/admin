// Command migrate-status-columns is an explicitly invoked maintenance tool.
// It never runs during API/Worker startup and never accepts Redis key patterns.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"admin/server/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
)

var tables = []string{"user_account", "permission_auth_platform", "permission_role", "permission_menu", "message_mail_config", "message_mail_template", "message_mail_recipient_rule", "message_sms_config", "message_sms_template", "message_sms_recipient_rule", "storage_cos_config", "storage_upload_rule", "system_setting", "system_scheduler_schedule", "payment_config", "payment_recharge_package"}
var snapshotPatterns = []string{
	"config-cache:snapshot:v1:system.setting:*",
	"config-cache:snapshot:v1:storage.cosconfig:*",
	"config-cache:snapshot:v1:message.mail:*",
	"config-cache:snapshot:v1:message.sms:*",
	"config-cache:snapshot:v1:payment.config:*",
	"auth:policy:v2:*",
}

func main() {
	mode := flag.String("mode", "inspect", "pg-env, inspect, preflight, cleanup, verify")
	audit := flag.String("audit", "", "private maintenance audit directory (required for cleanup)")
	flag.Parse()
	if err := run(*mode, *audit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode, audit string) error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return errors.New("read local configuration failed")
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return errors.New("local maintenance configuration is invalid")
	}
	pg, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		return errors.New("PostgreSQL configuration is invalid")
	}
	if pg.Host != "127.0.0.1" && pg.Host != "localhost" && pg.Host != "::1" {
		return errors.New("this runner only operates on the configured local PostgreSQL")
	}
	if mode == "pg-env" {
		// Captured in memory by the runner; never write this output to an audit log.
		sslmode := "disable"
		if pg.TLSConfig != nil {
			return errors.New("TLS PostgreSQL requires an explicit reviewed runner configuration")
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"PGHOST": pg.Host, "PGPORT": strconv.Itoa(int(pg.Port)), "PGUSER": pg.User, "PGPASSWORD": pg.Password, "PGDATABASE": pg.Database, "PGSSLMODE": sslmode})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if mode == "inspect" || mode == "verify" {
		db, err := pgx.ConnectConfig(ctx, pg)
		if err != nil {
			return errors.New("connect PostgreSQL failed")
		}
		defer db.Close(ctx)
		result := map[string]any{}
		digests := map[string]string{}
		columns := map[string]string{}
		for _, table := range tables {
			var old, new bool
			if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name='is_enabled'),EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name='status')`, table).Scan(&old, &new); err != nil {
				return errors.New("read status catalog failed")
			}
			if old == new {
				return fmt.Errorf("table %s has missing or conflicting state columns", table)
			}
			column := "is_enabled"
			if new {
				column = "status"
			}
			columns[table] = column
			if mode == "verify" && !new {
				return fmt.Errorf("table %s has not migrated", table)
			}
			// Normalize only the renamed field; all other business facts remain in the digest.
			q := fmt.Sprintf(`SELECT ((to_jsonb(t)-'%s')||jsonb_build_object('status',t.%s))::text FROM public.%s t ORDER BY t.id`, column, column, table)
			digest, err := digestRows(ctx, db, q)
			if err != nil {
				return fmt.Errorf("digest table %s failed", table)
			}
			digests[table] = digest
		}
		for _, table := range []string{"system_config_cache_state", "system_config_cache_outbox", "permission_menu_catalog_version", "permission_access_version"} {
			var exists bool
			if err := db.QueryRow(ctx, `SELECT to_regclass('public.'||$1) IS NOT NULL`, table).Scan(&exists); err != nil {
				return errors.New("read metadata catalog failed")
			}
			if !exists {
				continue
			}
			q := fmt.Sprintf(`SELECT to_jsonb(t)::text FROM public.%s t ORDER BY to_jsonb(t)::text`, table)
			digest, err := digestRows(ctx, db, q)
			if err != nil {
				return fmt.Errorf("digest metadata %s failed", table)
			}
			digests[table] = digest
		}
		for table, id := range map[string]int64{"system_scheduler_job": 38142, "system_scheduler_run": 38184} {
			q := fmt.Sprintf("SELECT to_jsonb(t)::text FROM public.%s t WHERE id=$1 ORDER BY id", table)
			digest, err := digestRows(ctx, db, q, id)
			if err != nil {
				return fmt.Errorf("verify retained scheduler fixture %s failed", table)
			}
			digests[table+"_retained_fixture"] = digest
		}
		result["columns"] = columns
		result["digests"] = digests
		if mode == "verify" {
			if err := verifyCaches(ctx, settings.RedisURL); err != nil {
				return err
			}
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if mode != "preflight" && mode != "cleanup" {
		return errors.New("unknown maintenance mode")
	}
	options, err := redis.ParseURL(settings.RedisURL)
	if err != nil {
		return errors.New("Redis configuration is invalid")
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		return errors.New("Redis is unavailable")
	}
	// Never turn an unfinished invalidation into ready state.
	for _, pattern := range []string{"auth:user-state:*", "auth:policy:v2:*", "config-cache:state:v1:system.setting:*", "config-cache:state:v1:storage.cosconfig:*", "config-cache:state:v1:message.mail:*", "config-cache:state:v1:message.sms:*", "config-cache:state:v1:payment.config:*"} {
		if err := scan(ctx, client, pattern, func(key string) error {
			raw, err := client.Get(ctx, key).Result()
			if err == redis.Nil {
				return nil
			}
			if err != nil {
				return errors.New("read preflight state failed")
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal([]byte(raw), &fields) != nil {
				return errors.New("malformed preflight cache state")
			}
			if string(fields["state"]) != `"ready"` {
				return errors.New("cache invalidation is unfinished; no cache cleanup performed")
			}
			if strings.HasPrefix(key, "auth:user-state:") {
				_, err = upgradeUser(key, raw)
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if mode == "preflight" {
		fmt.Println(`{"cachePreflight":"ready"}`)
		return nil
	}
	if audit == "" {
		return errors.New("cleanup requires an audit directory")
	}
	backup, err := os.OpenFile(filepath.Join(audit, "redis-before.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("create private Redis backup failed")
	}
	defer backup.Close()
	encoder := json.NewEncoder(backup)
	changed, deleted := 0, 0
	preserve := func(key, raw string) error {
		ttl, err := client.PTTL(ctx, key).Result()
		if err != nil {
			return errors.New("read Redis TTL failed")
		}
		if err := encoder.Encode(map[string]any{"key": key, "value": raw, "ttlMilliseconds": backupTTLMilliseconds(ttl)}); err != nil {
			return errors.New("write Redis backup failed")
		}
		if err := backup.Sync(); err != nil {
			return errors.New("flush Redis backup failed")
		}
		return nil
	}
	if err := scan(ctx, client, "auth:user-state:*", func(key string) error {
		raw, err := client.Get(ctx, key).Result()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			return errors.New("read user state failed")
		}
		next, err := upgradeUser(key, raw)
		if err != nil {
			return err
		}
		if next == raw {
			return nil
		}
		if err := preserve(key, raw); err != nil {
			return err
		}
		n, err := client.Eval(ctx, `if redis.call('GET',KEYS[1])~=ARGV[1] then return 0 end redis.call('SET',KEYS[1],ARGV[2],'KEEPTTL') return 1`, []string{key}, raw, next).Int()
		if err != nil || n != 1 {
			return errors.New("user state changed concurrently; stop and audit before retry")
		}
		changed++
		return nil
	}); err != nil {
		return err
	}
	for _, pattern := range snapshotPatterns {
		if err := scan(ctx, client, pattern, func(key string) error {
			raw, err := client.Get(ctx, key).Result()
			if err == redis.Nil {
				return nil
			}
			if err != nil {
				return errors.New("read obsolete snapshot failed")
			}
			if err := preserve(key, raw); err != nil {
				return err
			}
			n, err := client.Eval(ctx, `if redis.call('GET',KEYS[1])~=ARGV[1] then return 0 end return redis.call('DEL',KEYS[1])`, []string{key}, raw).Int()
			if err != nil || n != 1 {
				return errors.New("snapshot changed concurrently; cleanup incomplete")
			}
			deleted++
			return nil
		}); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]int{"userStatesUpgraded": changed, "obsoleteSnapshotsDeleted": deleted})
}

// go-redis represents PTTL -1/-2 as duration sentinels, not milliseconds.
func backupTTLMilliseconds(ttl time.Duration) int64 {
	if ttl < 0 {
		return int64(ttl)
	}
	return ttl.Milliseconds()
}

func scan(ctx context.Context, client *redis.Client, pattern string, visit func(string) error) error {
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return errors.New("scan maintenance Redis namespace failed")
		}
		for _, key := range keys {
			if err := visit(key); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func upgradeUser(key, raw string) (string, error) {
	if len(raw) > 8192 {
		return "", errors.New("user cache payload exceeds protocol size")
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return "", errors.New("invalid user cache object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return "", errors.New("invalid user cache field")
		}
		name, ok := token.(string)
		if !ok {
			return "", errors.New("invalid user cache name")
		}
		if _, exists := fields[name]; exists {
			return "", errors.New("duplicate user cache field")
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return "", errors.New("invalid user cache value")
		}
		fields[name] = value
	}
	if _, err := d.Token(); err != nil {
		return "", errors.New("incomplete user cache object")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return "", errors.New("trailing user cache data")
	}
	var version int
	var id int64
	var generation, state string
	var deleted bool
	if json.Unmarshal(fields["schemaVersion"], &version) != nil || json.Unmarshal(fields["userId"], &id) != nil || id < 1 || key != "auth:user-state:"+strconv.FormatInt(id, 10) || json.Unmarshal(fields["generation"], &generation) != nil || generation == "" || json.Unmarshal(fields["state"], &state) != nil || state != "ready" || string(fields["mutationToken"]) != "null" || json.Unmarshal(fields["deleted"], &deleted) != nil || string(fields["deleted"]) == "null" {
		return "", errors.New("invalid or unfinished authentication user state")
	}
	name := "isEnabled"
	if _, ok := fields["status"]; ok {
		name = "status"
	}
	var enabled bool
	if len(fields) != 7 || json.Unmarshal(fields[name], &enabled) != nil || string(fields[name]) == "null" || deleted && enabled || (version != 1 && version != 2) || (version == 2 && name != "status") {
		return "", errors.New("user status cache has incompatible schema")
	}
	if version == 2 {
		return raw, nil
	}
	fields["status"] = fields[name]
	delete(fields, "isEnabled")
	fields["schemaVersion"] = json.RawMessage("2")
	result, err := json.Marshal(fields)
	if err != nil {
		return "", errors.New("encode upgraded user state failed")
	}
	return string(result), nil
}

func verifyCaches(ctx context.Context, url string) error {
	options, err := redis.ParseURL(url)
	if err != nil {
		return errors.New("invalid Redis configuration")
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		return errors.New("Redis is unavailable during verification")
	}
	for _, pattern := range snapshotPatterns {
		if err := scan(ctx, client, pattern, func(string) error { return errors.New("obsolete configuration snapshots still exist") }); err != nil {
			return err
		}
	}
	return scan(ctx, client, "auth:user-state:*", func(key string) error {
		raw, err := client.Get(ctx, key).Result()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			return errors.New("read upgraded authentication state failed")
		}
		next, err := upgradeUser(key, raw)
		if err != nil {
			return err
		}
		if next != raw {
			return errors.New("authentication user state upgrade incomplete")
		}
		return nil
	})
}

// digestRows hashes one normalized row at a time; memory does not grow with user count.
func digestRows(ctx context.Context, db *pgx.Conn, query string, args ...any) (string, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hash := sha256.New()
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return "", err
		}
		hash.Write([]byte(row))
		hash.Write([]byte{'\n'})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
