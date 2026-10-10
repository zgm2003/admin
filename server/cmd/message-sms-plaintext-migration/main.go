// Explicit offline audit and Redis publication for the SMS plaintext migration.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var smsScope = cachegeneration.Scope{Namespace: "message.sms", ScopeKey: "global"}

type report struct {
	RuleScopeType   string `json:"ruleScopeType"`
	RuleActionType  string `json:"ruleActionType"`
	RuleRows        int64  `json:"ruleRows"`
	LogRows         int64  `json:"logRows"`
	RuleFactsHash   string `json:"ruleFactsHash"`
	LogFactsHash    string `json:"logFactsHash"`
	PlaintextSchema bool   `json:"plaintextSchema"`
	Generation      int64  `json:"generation"`
	ControlHash     string `json:"controlHash"`
	RedisState      string `json:"redisState"`
	RedisGeneration int64  `json:"redisGeneration"`
}

func parseMode(args []string) (string, bool, error) {
	f := flag.NewFlagSet("message-sms-plaintext-migration", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	mode := f.String("mode", "inspect", "inspect|sync")
	stopped := f.Bool("old-api-stopped", false, "API and Worker are stopped")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || (*mode != "inspect" && *mode != "sync") {
		return "", false, errors.New("only -mode inspect|sync and -old-api-stopped are accepted")
	}
	if *mode == "inspect" && *stopped {
		return "", false, errors.New("-old-api-stopped is valid only with -mode sync")
	}
	if *mode == "sync" && !*stopped {
		return "", false, errors.New("sync requires API/Worker stopped and -old-api-stopped")
	}
	return *mode, *stopped, nil
}

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runMain() error {
	mode, _, err := parseMode(os.Args[1:])
	if err != nil {
		return err
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return errors.New("cannot load server environment")
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return errors.New("invalid PostgreSQL/Redis configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(settings.PostgresDSN)
	if err != nil {
		return errors.New("invalid PostgreSQL configuration")
	}
	cfg.RuntimeParams["search_path"] = "public"
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("PostgreSQL connection failed")
	}
	defer db.Close(ctx)
	report, err := inspectPostgres(ctx, db, mode == "sync")
	if err != nil {
		return err
	}
	redis, err := projectredis.Open(ctx, settings.RedisURL)
	if err != nil {
		return errors.New("Redis connection failed")
	}
	defer redis.Close()
	store := cachegeneration.NewStore(redis)
	if mode == "sync" {
		if err := syncState(ctx, store, report.Generation); err != nil {
			return err
		}
	}
	state, found, err := store.Read(ctx, smsScope)
	if err != nil {
		return errors.New("SMS Redis state inspection failed")
	}
	if found {
		report.RedisState, report.RedisGeneration = state.State, state.Generation
	}
	if mode == "sync" && (!found || state.State != cachegeneration.StateReady || state.Generation != report.Generation) {
		return errors.New("SMS Redis state verification failed")
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func inspectPostgres(ctx context.Context, db *pgx.Conn, numericRequired bool) (report, error) {
	var out report
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, errors.New("PostgreSQL audit snapshot unavailable")
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return out, errors.New("audit timezone unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name='scope'), (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name='action')`).Scan(&out.RuleScopeType, &out.RuleActionType); err != nil {
		return out, errors.New("SMS rule column types unavailable")
	}
	numeric := out.RuleScopeType == "smallint" && out.RuleActionType == "smallint"
	if numericRequired && !numeric {
		return out, errors.New("SMS rule scope/action are not numeric")
	}
	if !numeric && (out.RuleScopeType != "character varying" || out.RuleActionType != "character varying") {
		return out, errors.New("SMS rule column types are unexpected")
	}
	if err := tx.QueryRow(ctx, `SELECT count(*),md5(COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id),'[]'::jsonb)::text) FROM message_sms_recipient_rule r`).Scan(&out.RuleRows, &out.RuleFactsHash); err != nil {
		return out, errors.New("SMS rule facts unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT count(*),md5(COALESCE(jsonb_agg(to_jsonb(l) ORDER BY id),'[]'::jsonb)::text) FROM message_sms_log l`).Scan(&out.LogRows, &out.LogFactsHash); err != nil {
		return out, errors.New("SMS log facts unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT
 NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name IN ('pattern_ciphertext','pattern_hint','pattern_hmac'))
 AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_recipient_rule' AND column_name='pattern')
 AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_log' AND column_name IN ('to_phone_ciphertext','to_phone_hint','to_phone_hmac'))
 AND EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_sms_log' AND column_name='to_phone')`).Scan(&out.PlaintextSchema); err != nil {
		return out, errors.New("SMS plaintext schema facts unavailable")
	}
	if numericRequired && !out.PlaintextSchema {
		return out, errors.New("SMS plaintext columns are not complete")
	}
	if err := tx.QueryRow(ctx, `SELECT generation FROM system_config_cache_generation WHERE namespace='message.sms' AND scope_key='global'`).Scan(&out.Generation); err != nil || out.Generation < 1 {
		return out, errors.New("message.sms/global generation unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT md5(jsonb_build_object('otherGenerations',(SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key),'[]'::jsonb) FROM system_config_cache_generation g WHERE NOT(namespace='message.sms' AND scope_key='global')),'platforms',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'menu_version',menu_version) ORDER BY id),'[]'::jsonb) FROM permission_auth_platform),'mailGeneration',(SELECT generation FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global'))::text)`).Scan(&out.ControlHash); err != nil {
		return out, errors.New("control audit facts unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return out, errors.New("PostgreSQL audit snapshot failed")
	}
	return out, nil
}

func syncState(ctx context.Context, store *cachegeneration.Store, generation int64) error {
	previous, found, err := store.Read(ctx, smsScope)
	if err != nil {
		return errors.New("SMS Redis state unavailable")
	}
	if found && previous.State != cachegeneration.StateReady {
		return errors.New("SMS Redis mutation is active; retry after writer finishes")
	}
	if found && previous.Generation > generation {
		return errors.New("SMS Redis generation is ahead; refusing downgrade")
	}
	result, err := store.Reconcile(ctx, smsScope, generation)
	if err != nil || result == cachegeneration.PublishSkippedInvalidating {
		return errors.New("SMS Redis publication failed")
	}
	return nil
}
