package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"admin/server/internal/database"
	"admin/server/internal/module/ai/run/event"
	"admin/server/internal/module/ai/usage"
	ledger "admin/server/internal/module/payment/transaction"
	"admin/server/internal/module/payment/wallet"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/system/scheduler"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type settings struct {
	ChargeID int64
	Apply    bool
}
type settlementService interface {
	Preview(context.Context, int64) (usage.SettlementPreview, error)
	Settle(context.Context, int64) error
}

func parseArgs(args []string, output io.Writer) (settings, error) {
	var v settings
	var id string
	f := flag.NewFlagSet("ai-usage-settle", flag.ContinueOnError)
	f.SetOutput(output)
	f.StringVar(&id, "charge-id", "", "Existing Charge ID to inspect or settle")
	f.BoolVar(&v.Apply, "apply", false, "Explicitly settle the existing immutable charge; default is dry-run")
	if err := f.Parse(args); err != nil {
		return v, err
	}
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed < 1 || len(f.Args()) != 0 {
		return v, errors.New("a positive --charge-id is required; positional arguments are not accepted")
	}
	v.ChargeID = parsed
	return v, nil
}
func execute(ctx context.Context, service settlementService, v settings, output io.Writer) error {
	if service == nil || v.ChargeID < 1 {
		return errors.New("invalid settlement command")
	}
	preview, err := service.Preview(ctx, v.ChargeID)
	if err != nil {
		return publicError(err)
	}
	if v.Apply {
		if err = service.Settle(ctx, v.ChargeID); err != nil {
			return publicError(err)
		}
		preview, err = service.Preview(ctx, v.ChargeID)
		if err != nil {
			return errors.New("settlement returned successfully but final inspection failed; inspect durable charge and ledger before retry")
		}
	}
	report := struct {
		usage.SettlementPreview
		DryRun bool `json:"dryRun"`
	}{preview, !v.Apply}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(report); err != nil {
		return errors.New("cannot print report; inspect durable charge and ledger before retrying --apply")
	}
	return nil
}
func publicError(err error) error {
	switch {
	case errors.Is(err, usage.ErrUsageUnknown):
		return usage.ErrUsageUnknown
	case errors.Is(err, usage.ErrLedgerConflict):
		return usage.ErrLedgerConflict
	case errors.Is(err, usage.ErrFrozenSnapshotMismatch):
		return usage.ErrFrozenSnapshotMismatch
	default:
		return errors.New("settlement inspection or execution failed; inspect durable charge and ledger before retry")
	}
}
func runCLI(args []string, output io.Writer) error {
	v, err := parseArgs(args, output)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return errors.New("cannot load local environment")
	}
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}
	if _, err = pgx.ParseConfig(dsn); err != nil {
		return errors.New("POSTGRES_DSN is invalid")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	open, cancel := context.WithTimeout(ctx, 5*time.Second)
	connection, err := database.Open(open, dsn)
	cancel()
	if err != nil {
		return errors.New("PostgreSQL connection failed")
	}
	defer connection.Close()
	db := connection.GORM.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	events := event.NewRepository(db, realtime.NewRepository(db))
	repo := usage.NewRepository(db, wallet.NewRepository(db), ledger.NewRepository(db), events)
	repo.SetScheduler(scheduler.NewRepository(db))
	return execute(ctx, usage.NewService(repo), v, output)
}
func main() {
	if err := runCLI(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
