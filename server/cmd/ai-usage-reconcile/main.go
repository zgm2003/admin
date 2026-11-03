package main

import (
	"admin/server/internal/database"
	"admin/server/internal/module/ai/run"
	"admin/server/internal/module/ai/run/event"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/system/scheduler"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type settings struct {
	RunID              int64
	Evidence, Operator string
	Apply              bool
}
type usageReconciler interface {
	Execute(context.Context, int64, []byte, string, bool) (run.ReconcileReport, error)
}

func parseArgs(args []string, output io.Writer) (settings, error) {
	var v settings
	var id string
	f := flag.NewFlagSet("ai-usage-reconcile", flag.ContinueOnError)
	f.SetOutput(output)
	f.StringVar(&id, "run-id", "", "Run ID to reconcile")
	f.StringVar(&v.Evidence, "evidence", "", "Path to local traceable supplier usage evidence JSON")
	f.StringVar(&v.Operator, "operator", "", "Operator audit identity (required with --apply)")
	f.BoolVar(&v.Apply, "apply", false, "Explicitly apply verified evidence; default is read-only dry-run")
	if e := f.Parse(args); e != nil {
		return v, e
	}
	if len(f.Args()) != 0 {
		return v, errors.New("unexpected positional arguments")
	}
	parsed, e := strconv.ParseInt(id, 10, 64)
	if e != nil || parsed < 1 || strings.TrimSpace(v.Evidence) == "" {
		return v, errors.New("--run-id and --evidence are required")
	}
	v.RunID = parsed
	v.Operator = strings.TrimSpace(v.Operator)
	if v.Apply && v.Operator == "" {
		return v, errors.New("--operator is required with --apply")
	}
	if v.Operator == "" {
		v.Operator = "dry-run"
	}
	if len(v.Operator) > 128 || strings.ContainsAny(v.Operator, "\r\n\t") {
		return v, errors.New("operator audit identity is invalid")
	}
	return v, nil
}
func readEvidence(path string) ([]byte, error) {
	if strings.HasPrefix(path, string([]byte{92, 92})) || strings.HasPrefix(path, "//") {
		return nil, errors.New("evidence must be a local regular file")
	}

	file, e := os.Open(path)
	if e != nil {
		return nil, errors.New("cannot open local evidence file")
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > run.MaxReconcileEvidenceBytes {
		return nil, errors.New("evidence must be a regular file of at most 1 MiB")
	}
	data, e := io.ReadAll(io.LimitReader(file, run.MaxReconcileEvidenceBytes+1))
	if e != nil || len(data) == 0 || len(data) > run.MaxReconcileEvidenceBytes {
		return nil, errors.New("evidence file is empty, oversized or unreadable")
	}
	return data, nil
}
func execute(ctx context.Context, reconciler usageReconciler, v settings, output io.Writer) error {
	raw, e := readEvidence(v.Evidence)
	if e != nil {
		return e
	}
	report, e := reconciler.Execute(ctx, v.RunID, raw, v.Operator, v.Apply)
	if e != nil {
		switch {
		case errors.Is(e, run.ErrReconcileEvidence):
			return run.ErrReconcileEvidence
		case errors.Is(e, run.ErrReconcileConflict):
			return run.ErrReconcileConflict
		case errors.Is(e, run.ErrInvalidSnapshot):
			return errors.New("frozen AI configuration or pricing facts are invalid")
		default:
			return errors.New("reconciliation failed; database unchanged or commit outcome must be checked before retry")
		}
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if e = encoder.Encode(report); e != nil {
		return errors.New("cannot write reconciliation report; check durable audit before retrying --apply")
	}
	return nil
}
func runCLI(args []string, output io.Writer) error {
	v, e := parseArgs(args, output)
	if errors.Is(e, flag.ErrHelp) {
		return nil
	}
	if e != nil {
		return e
	}
	if e = godotenv.Load(); e != nil && !os.IsNotExist(e) {
		return errors.New("cannot load local environment")
	}
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}
	if _, e := pgx.ParseConfig(dsn); e != nil {
		return errors.New("POSTGRES_DSN is invalid")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	open, cancel := context.WithTimeout(ctx, 5*time.Second)
	connection, e := database.Open(open, dsn)
	cancel()
	if e != nil {
		return errors.New("PostgreSQL connection failed")
	}
	defer connection.Close()
	db := connection.GORM.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	jobs := scheduler.NewRepository(db)
	events := event.NewRepository(db, realtime.NewRepository(db))
	service := run.NewReconcileService(run.NewReconcileRepository(db, jobs, events))
	return execute(ctx, service, v, output)
}
func main() {
	if e := runCLI(os.Args[1:], os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
