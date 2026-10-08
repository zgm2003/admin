// Offline audit and targeted mail-generation publication. SQL migration is external.
package main

import (
	"admin/server/internal/config"
	projectredis "admin/server/internal/redis"
	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"io"
	"os"
	"time"
)

var mailScope = cachegeneration.Scope{Namespace: "message.mail", ScopeKey: "global"}

type report struct {
	ScopeType           string `json:"scopeType"`
	ActionType          string `json:"actionType"`
	RowCount            int64  `json:"rowCount"`
	RuleFactsHash       string `json:"ruleFactsHash"`
	MailGeneration      int64  `json:"mailGeneration"`
	ControlHash         string `json:"controlHash"`
	MailOutboxHash      string `json:"mailOutboxHash"`
	MailStateFound      bool   `json:"mailStateFound"`
	MailState           string `json:"mailState"`
	MailStateGeneration int64  `json:"mailStateGeneration"`
}

func parseMode(args []string) (string, bool, error) {
	f := flag.NewFlagSet("mail-recipient-rule-numeric-migration", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	m := f.String("mode", "inspect", "inspect|sync")
	stopped := f.Bool("old-api-stopped", false, "API and Worker are stopped")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || (*m != "inspect" && *m != "sync") {
		return "", false, errors.New("only -mode inspect|sync and -old-api-stopped are accepted")
	}
	if *m == "inspect" && *stopped {
		return "", false, errors.New("-old-api-stopped is valid only with -mode sync")
	}
	if *m == "sync" && !*stopped {
		return "", false, errors.New("sync requires API/Worker stopped and -old-api-stopped")
	}
	return *m, *stopped, nil
}

func main() {
	if err := runMain(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
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
	s, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return errors.New("invalid PostgreSQL/Redis configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg, err := pgx.ParseConfig(s.PostgresDSN)
	if err != nil {
		return errors.New("invalid PostgreSQL configuration")
	}
	cfg.RuntimeParams["search_path"] = "public"
	db, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("PostgreSQL connection failed")
	}
	defer func() { _ = db.Close(ctx) }()
	r, err := inspectPostgres(ctx, db, mode == "sync")
	if err != nil {
		return err
	}
	client, err := projectredis.Open(ctx, s.RedisURL)
	if err != nil {
		return errors.New("Redis connection failed")
	}
	defer func() { _ = client.Close() }()
	store := cachegeneration.NewStore(client)
	if mode == "sync" {
		if err := syncMailState(ctx, store, mailScope, r.MailGeneration); err != nil {
			return err
		}
	}
	state, found, err := store.Read(ctx, mailScope)
	if err != nil {
		return errors.New("mail Redis state inspection failed")
	}
	r.MailStateFound = found
	if found {
		r.MailState = state.State
		r.MailStateGeneration = state.Generation
		if state.State == cachegeneration.StateInvalidating {
			r.MailStateGeneration = state.BaseGeneration
		}
	}
	if mode == "sync" && (!found || state.State != cachegeneration.StateReady || state.Generation != r.MailGeneration) {
		return errors.New("mail Redis state verification failed")
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		return errors.New("audit output failed")
	}
	return nil
}

func inspectPostgres(ctx context.Context, db *pgx.Conn, numericRequired bool) (report, error) {
	var r report
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return r, errors.New("PostgreSQL audit snapshot unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Canonical timestamps must not depend on connection timezone.
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return r, errors.New("PostgreSQL audit timezone unavailable")
	}
	const typesQuery = `SELECT
 (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule' AND column_name='scope'),
 (SELECT data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='message_mail_recipient_rule' AND column_name='action')`
	if err := tx.QueryRow(ctx, typesQuery).Scan(&r.ScopeType, &r.ActionType); err != nil {
		return r, errors.New("recipient-rule column types unavailable")
	}
	numeric := r.ScopeType == "smallint" && r.ActionType == "smallint"
	legacy := r.ScopeType == "character varying" && r.ActionType == "character varying"
	if !numeric && !legacy || numericRequired && !numeric {
		return r, errors.New("recipient-rule columns have unexpected types")
	}
	var invalid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM message_mail_recipient_rule WHERE scope IS NULL OR action IS NULL OR ($1 AND (scope::text NOT IN ('0','1') OR action::text NOT IN ('0','1'))) OR (NOT $1 AND (scope::text NOT IN ('email','domain') OR action::text NOT IN ('deny','allow'))))`, numeric).Scan(&invalid); err != nil || invalid {
		return r, errors.New("recipient-rule data contains invalid enum values")
	}
	// to_jsonb preserves every row field, including soft deletion and timestamps.
	const factsQuery = `SELECT count(*),md5(COALESCE(jsonb_agg(
 (to_jsonb(r)-'scope'-'action') || jsonb_build_object(
 'scope',CASE scope::text WHEN 'email' THEN 0 WHEN 'domain' THEN 1 ELSE scope::text::smallint END,
 'action',CASE action::text WHEN 'deny' THEN 0 WHEN 'allow' THEN 1 ELSE action::text::smallint END)
 ORDER BY id),'[]'::jsonb)::text) FROM message_mail_recipient_rule r`
	if err := tx.QueryRow(ctx, factsQuery).Scan(&r.RowCount, &r.RuleFactsHash); err != nil {
		return r, errors.New("recipient-rule audit facts unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT generation FROM system_config_cache_generation WHERE namespace='message.mail' AND scope_key='global'`).Scan(&r.MailGeneration); err != nil || r.MailGeneration < 1 {
		return r, errors.New("message.mail generation unavailable")
	}
	const controlQuery = `SELECT md5(jsonb_build_object(
 'generation',(SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY namespace,scope_key),'[]'::jsonb) FROM system_config_cache_generation g WHERE NOT(namespace='message.mail' AND scope_key='global')),
 'platform',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'menu_version',menu_version) ORDER BY id),'[]'::jsonb) FROM permission_auth_platform))::text)`
	if err := tx.QueryRow(ctx, controlQuery).Scan(&r.ControlHash); err != nil {
		return r, errors.New("control audit facts unavailable")
	}
	if err := tx.QueryRow(ctx, `SELECT md5(COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]'::jsonb)::text) FROM system_config_cache_outbox o WHERE namespace='message.mail' AND scope_key='global'`).Scan(&r.MailOutboxHash); err != nil {
		return r, errors.New("mail outbox audit facts unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return r, errors.New("PostgreSQL audit snapshot failed")
	}
	return r, nil
}

func syncMailState(ctx context.Context, store *cachegeneration.Store, scope cachegeneration.Scope, generation int64) error {
	if generation < 1 {
		return errors.New("authoritative generation is invalid")
	}
	previous, found, err := store.Read(ctx, scope)
	if err != nil {
		return errors.New("mail Redis state is corrupt or unavailable")
	}
	if found && previous.State != cachegeneration.StateReady {
		return errors.New("mail Redis mutation is active; rerun after the writer finishes")
	}
	if found && previous.Generation > generation {
		return errors.New("mail Redis generation is ahead of PostgreSQL; refusing downgrade")
	}
	result, err := store.Reconcile(ctx, scope, generation)
	if err != nil || result == cachegeneration.PublishSkippedInvalidating {
		return errors.New("mail Redis state publication failed; rerun this maintenance command")
	}
	current, found, err := store.Read(ctx, scope)
	if err != nil || !found || current.State != cachegeneration.StateReady || current.Generation != generation {
		return errors.New("mail Redis state verification failed")
	}
	return nil
}
