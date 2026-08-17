package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/taskdemo"
	"admin/server/internal/queue"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("Worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := config.LoadWorker(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load Worker config: %w", err)
	}

	processContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	postgres, err := database.Open(processContext, settings.PostgresDSN)
	if err != nil {
		return err
	}
	defer postgres.Close()

	repository := taskdemo.NewRepository(postgres.GORM)
	service := taskdemo.NewService(repository, nil, logger)
	mux := asynq.NewServeMux()
	taskdemo.Register(mux, service)
	server, err := queue.NewServer(settings.RedisURL)
	if err != nil {
		return err
	}
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start Asynq Worker: %w", err)
	}
	<-processContext.Done()
	server.Shutdown()
	return nil
}
