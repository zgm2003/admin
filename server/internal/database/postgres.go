package database

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Connection struct {
	GORM *gorm.DB
	SQL  *sql.DB
}

func Open(ctx context.Context, dsn string) (*Connection, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access PostgreSQL connection: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return &Connection{GORM: db, SQL: sqlDB}, nil
}

func (c *Connection) Ping(ctx context.Context) error {
	return c.SQL.PingContext(ctx)
}

func (c *Connection) Close() error {
	return c.SQL.Close()
}

func AutoMigrate(ctx context.Context, db *gorm.DB, models ...any) error {
	if db == nil {
		return fmt.Errorf("AutoMigrate requires a database")
	}
	if len(models) == 0 {
		return fmt.Errorf("AutoMigrate requires at least one model")
	}
	if err := db.WithContext(ctx).AutoMigrate(models...); err != nil {
		return fmt.Errorf("AutoMigrate: %w", err)
	}
	return nil
}
