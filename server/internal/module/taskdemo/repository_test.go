package taskdemo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/taskdemo"
	"github.com/joho/godotenv"
)

func TestRepositoryPersistsAndUpdatesTask(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration test")
	}
	if err := godotenv.Load("../../../.env"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("load server .env: %v", err)
	}
	if _, exists := os.LookupEnv("TRUSTED_PROXIES"); !exists {
		if err := os.Setenv("TRUSTED_PROXIES", "none"); err != nil {
			t.Fatalf("set test trusted proxy mode: %v", err)
		}
	}
	settings, err := config.LoadAPI(os.LookupEnv)
	if err != nil {
		t.Fatalf("load API config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := database.Open(ctx, settings.PostgresDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	defer connection.Close()
	if err := database.AutoMigrate(ctx, connection.GORM, &taskdemo.Task{}); err != nil {
		t.Fatalf("migrate task table: %v", err)
	}
	for _, columnName := range []string{"created_at", "updated_at"} {
		var column struct {
			DataType      string  `gorm:"column:data_type"`
			IsNullable    string  `gorm:"column:is_nullable"`
			ColumnDefault *string `gorm:"column:column_default"`
		}
		if err := connection.GORM.WithContext(ctx).Raw(`
			SELECT data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = ?
			  AND column_name = ?`, "foundation_task", columnName).Scan(&column).Error; err != nil {
			t.Fatalf("inspect %s: %v", columnName, err)
		}
		if column.DataType != "timestamp with time zone" {
			t.Errorf("%s data type = %q, want timestamp with time zone", columnName, column.DataType)
		}
		if column.IsNullable != "NO" {
			t.Errorf("%s nullable = %q, want NO", columnName, column.IsNullable)
		}
		if column.ColumnDefault == nil || *column.ColumnDefault == "" {
			t.Errorf("%s default = %v, want non-empty default", columnName, column.ColumnDefault)
		}
	}

	repository := taskdemo.NewRepository(connection.GORM)
	task := taskdemo.Task{
		ID:      "integration" + time.Now().Format("150405.000000"),
		Message: "repository-check",
		Status:  taskdemo.StatusPending,
	}
	t.Cleanup(func() {
		connection.GORM.WithContext(context.Background()).Delete(&taskdemo.Task{}, "id = ?", task.ID)
	})

	if err := repository.Create(ctx, &task); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	stored, err := repository.Find(ctx, task.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if stored.Message != task.Message || stored.Status != taskdemo.StatusPending {
		t.Fatalf("stored task = %+v", stored)
	}

	if err := repository.UpdateStatus(ctx, task.ID, taskdemo.StatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	stored, err = repository.Find(ctx, task.ID)
	if err != nil {
		t.Fatalf("Find() after update error = %v", err)
	}
	if stored.Status != taskdemo.StatusCompleted {
		t.Fatalf("status = %q", stored.Status)
	}
}
