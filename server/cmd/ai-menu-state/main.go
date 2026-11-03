// ai-menu-state inspects the Admin AI catalog and publishes its database menu
// version after the maintainer has migrated it with API/Worker stopped.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"admin/server/internal/database"
	permissionstate "admin/server/internal/module/permission/state"
	projectredis "admin/server/internal/redis"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type options struct {
	EnvFile         string
	Apply           bool
	ServicesStopped bool
}

type menuFacts struct {
	PlatformID          int64  `json:"platformId"`
	Version             int64  `json:"version"`
	Nodes               int64  `json:"nodes"`
	Directories         int64  `json:"directories"`
	Pages               int64  `json:"pages"`
	Actions             int64  `json:"actions"`
	MenuDigest          string `json:"menuDigest"`
	OtherMenuDigest     string `json:"otherMenuDigest"`
	OtherPlatformDigest string `json:"otherPlatformDigest"`
	RoleMenuDigest      string `json:"roleMenuDigest"`
}

type report struct {
	Facts        menuFacts `json:"facts"`
	DryRun       bool      `json:"dryRun"`
	RedisFound   bool      `json:"redisFound"`
	RedisState   string    `json:"redisState"`
	RedisVersion int64     `json:"redisVersion"`
}

func parseArgs(args []string, output io.Writer) (options, error) {
	var v options
	f := flag.NewFlagSet("ai-menu-state", flag.ContinueOnError)
	f.SetOutput(output)
	f.StringVar(&v.EnvFile, "env-file", ".env", "Environment file containing POSTGRES_DSN and REDIS_URL")
	f.BoolVar(&v.Apply, "apply", false, "Publish only the database-confirmed Admin menu version; default is preview")
	f.BoolVar(&v.ServicesStopped, "services-stopped", false, "Assert API, Worker and other permission writers are stopped")
	if err := f.Parse(args); err != nil {
		return v, err
	}
	if strings.TrimSpace(v.EnvFile) == "" || len(f.Args()) != 0 || v.Apply && !v.ServicesStopped {
		return v, errors.New("apply requires --services-stopped; environment file must be set and positional arguments are not accepted")
	}
	return v, nil
}

func inspect(ctx context.Context, db *gorm.DB) (menuFacts, error) {
	var rows []menuFacts
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`WITH admin AS (
 SELECT * FROM permission_auth_platform WHERE code='admin' AND status=1 AND deleted_at IS NULL
), ai_menu AS (
 SELECT m.* FROM permission_menu m JOIN admin p ON p.id=m.platform_id
 WHERE m.deleted_at IS NULL AND (m.code='ai' OR m.code LIKE 'ai:%')
)
SELECT p.id AS platform_id,p.menu_version AS version,
 (SELECT count(*) FROM ai_menu) AS nodes,
 (SELECT count(*) FROM ai_menu WHERE menu_type='directory' AND status=1) AS directories,
 (SELECT count(*) FROM ai_menu WHERE menu_type='page' AND status=1) AS pages,
 (SELECT count(*) FROM ai_menu WHERE menu_type='action' AND status=1) AS actions,
 (SELECT md5(coalesce(string_agg(md5(to_jsonb(m)::text),'' ORDER BY id),'')) FROM ai_menu m) AS menu_digest,
 (SELECT md5(coalesce(string_agg(md5(to_jsonb(m)::text),'' ORDER BY id),'')) FROM permission_menu m WHERE NOT(m.platform_id=p.id AND m.deleted_at IS NULL AND (m.code='ai' OR m.code LIKE 'ai:%'))) AS other_menu_digest,
 (SELECT md5(coalesce(string_agg(md5(to_jsonb(a)::text),'' ORDER BY id),'')) FROM permission_auth_platform a WHERE a.id<>p.id) AS other_platform_digest,
 (SELECT md5(coalesce(string_agg(md5(to_jsonb(g)::text),'' ORDER BY id),'')) FROM permission_role_menu g) AS role_menu_digest
FROM admin p`).Scan(&rows).Error
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil || len(rows) != 1 || rows[0].PlatformID < 1 || rows[0].Version < 1 {
		return menuFacts{}, errors.New("Admin menu database inspection failed")
	}
	return rows[0], nil
}

func execute(ctx context.Context, db *gorm.DB, store *permissionstate.MenuStore, apply bool, output io.Writer) error {
	facts, err := inspect(ctx, db)
	if err != nil {
		return err
	}
	if apply {
		if facts.Nodes != 45 || facts.Directories != 1 || facts.Pages != 6 || facts.Actions != 38 {
			return errors.New("Admin AI menu must contain the complete enabled catalog before publication")
		}
		if err := store.RebuildReadyState(ctx, facts.PlatformID, facts.Version); err != nil {
			return errors.New("PostgreSQL menu is committed; Redis menu publication failed; keep services stopped and retry after inspecting Redis state")
		}
	}
	state, found, err := store.Read(ctx, facts.PlatformID)
	if err != nil {
		return errors.New("Redis menu inspection failed; no connection details are printed")
	}
	if apply && (!found || state.State != permissionstate.StateReady || state.Version != facts.Version) {
		return errors.New("Redis menu publication could not be verified; keep services stopped")
	}
	if err := json.NewEncoder(output).Encode(report{Facts: facts, DryRun: !apply, RedisFound: found, RedisState: state.State, RedisVersion: state.Version}); err != nil {
		return errors.New("cannot write inspection report; inspect menu state before retrying apply")
	}
	return nil
}

func runCLI(args []string, output io.Writer) error {
	v, err := parseArgs(args, output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	env, err := godotenv.Read(v.EnvFile)
	if err != nil {
		return errors.New("cannot read environment file")
	}
	connectionConfig, err := pgx.ParseConfig(strings.TrimSpace(env["POSTGRES_DSN"]))
	if err != nil || strings.TrimSpace(env["POSTGRES_DSN"]) == "" || strings.TrimSpace(env["REDIS_URL"]) == "" {
		return errors.New("POSTGRES_DSN and REDIS_URL must be configured")
	}
	// The command can only read database facts. Its single optional write is the
	// existing MenuStore CAS publication for the Admin platform found below.
	connectionConfig.RuntimeParams["search_path"] = "public"
	connectionConfig.RuntimeParams["default_transaction_read_only"] = "on"
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	connection, err := database.Open(ctx, connectionConfig.ConnString())
	if err != nil {
		return errors.New("PostgreSQL connection failed")
	}
	defer connection.Close()
	redis, err := projectredis.Open(ctx, strings.TrimSpace(env["REDIS_URL"]))
	if err != nil {
		return errors.New("Redis connection failed")
	}
	defer redis.Close()
	db := connection.GORM.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	return execute(ctx, db, permissionstate.NewMenuStore(redis), v.Apply, output)
}

func main() {
	if err := runCLI(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
