package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"admin/server/internal/database"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/rbac/role"
	"admin/server/internal/module/user/account"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type lookupEnv func(string) (string, bool)

type bootstrapSettings struct {
	PostgresDSN string
	Username    string
	Email       string
	Password    string
}

type adminCreator interface {
	Create(context.Context, auth.BootstrapAdminInput) (auth.Registered, error)
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := loadBootstrapSettings(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	postgres, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		return err
	}
	defer postgres.Close()
	creator := auth.NewBootstrapService(account.NewRepository(postgres.GORM), role.NewRepository(postgres.GORM))
	return execute(ctx, creator, settings, os.Stdout)
}

func loadBootstrapSettings(lookup lookupEnv) (bootstrapSettings, error) {
	postgresDSN, err := requiredSetting(lookup, "POSTGRES_DSN", true)
	if err != nil {
		return bootstrapSettings{}, err
	}
	if _, err := pgx.ParseConfig(postgresDSN); err != nil {
		return bootstrapSettings{}, fmt.Errorf("POSTGRES_DSN: %w", err)
	}
	username, err := requiredSetting(lookup, "BOOTSTRAP_ADMIN_USERNAME", true)
	if err != nil {
		return bootstrapSettings{}, err
	}
	email, err := requiredSetting(lookup, "BOOTSTRAP_ADMIN_EMAIL", true)
	if err != nil {
		return bootstrapSettings{}, err
	}
	password, err := requiredSetting(lookup, "BOOTSTRAP_ADMIN_PASSWORD", false)
	if err != nil {
		return bootstrapSettings{}, err
	}
	return bootstrapSettings{PostgresDSN: postgresDSN, Username: username, Email: email, Password: password}, nil
}

func requiredSetting(lookup lookupEnv, key string, trimSpace bool) (string, error) {
	value, found := lookup(key)
	if trimSpace {
		value = strings.TrimSpace(value)
	}
	if !found || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func execute(ctx context.Context, creator adminCreator, settings bootstrapSettings, output io.Writer) error {
	created, err := creator.Create(ctx, auth.BootstrapAdminInput{
		Username: settings.Username,
		Email:    settings.Email,
		Password: settings.Password,
	})
	if err != nil {
		return fmt.Errorf("create super admin: %w", err)
	}
	_, err = fmt.Fprintf(output, "super admin created: userId=%d username=%s\n", created.UserID, created.Username)
	if err != nil {
		return fmt.Errorf("write bootstrap result: %w", err)
	}
	return nil
}
