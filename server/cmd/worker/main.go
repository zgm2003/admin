package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/system/operationLog"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheGeneration"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

// workerRelayShutdownBudget 是取消 relay 后等待其退出的上限。
const workerRelayShutdownBudget = 5 * time.Second

// relayRunner 由 Asynq server 与进程 context 共享生命周期。
type relayRunner interface {
	Run(ctx context.Context) error
}

type asynqServer interface {
	Start(handler asynq.Handler) error
	Shutdown()
}

type workerAssembly struct {
	ProcessContext context.Context
	Logger         *slog.Logger
	Mux            *asynq.ServeMux
	NewRelay       func() (relayRunner, error)
	NewServer      func() (asynqServer, error)
}

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
	redisClient, err := openWorkerRedis(processContext, settings.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
	mux := buildWorkerMux(operationLogService)

	return runWorkerAssembly(workerAssembly{
		ProcessContext: processContext,
		Logger:         logger,
		Mux:            mux,
		NewRelay: func() (relayRunner, error) {
			repository := cachegeneration.NewRepository(postgres.GORM)
			store := cachegeneration.NewStore(redisClient)
			relay := cachegeneration.NewRelay(repository, store, logger)
			if relay == nil {
				return nil, fmt.Errorf("build cache generation relay")
			}
			return relay, nil
		},
		NewServer: func() (asynqServer, error) {
			return queue.NewServer(settings.RedisURL)
		},
	})
}

// runWorkerAssembly 启动 relay goroutine 后启动 Asynq；任一启动失败都会
// 关闭已启动资源。进程 context 取消时先停 relay 再 Shutdown Asynq。
func runWorkerAssembly(assembly workerAssembly) error {
	if assembly.ProcessContext == nil || assembly.Mux == nil || assembly.NewRelay == nil || assembly.NewServer == nil {
		return fmt.Errorf("worker assembly requires context, mux, relay and server factories")
	}
	relay, err := assembly.NewRelay()
	if err != nil {
		return fmt.Errorf("build cache generation relay: %w", err)
	}
	server, err := assembly.NewServer()
	if err != nil {
		return fmt.Errorf("build Asynq Worker: %w", err)
	}

	relayContext, stopRelay := context.WithCancel(assembly.ProcessContext)
	relayDone := make(chan error, 1)
	go func() { relayDone <- relay.Run(relayContext) }()

	if err := server.Start(assembly.Mux); err != nil {
		stopRelay()
		waitForRelayShutdown(assembly.Logger, relayDone)
		return fmt.Errorf("start Asynq Worker: %w", err)
	}

	<-assembly.ProcessContext.Done()
	stopRelay()
	waitForRelayShutdown(assembly.Logger, relayDone)
	server.Shutdown()
	return nil
}

func waitForRelayShutdown(logger *slog.Logger, relayDone <-chan error) {
	timer := time.NewTimer(workerRelayShutdownBudget)
	defer timer.Stop()
	select {
	case err := <-relayDone:
		if err != nil && logger != nil {
			logger.Error("cache generation relay stopped with error", "errorClass", cachegeneration.ErrorClass(err))
		}
	case <-timer.C:
		if logger != nil {
			logger.Error("cache generation relay did not stop within the shutdown budget")
		}
	}
}

func buildWorkerMux(operationLogProcessor operationlog.Processor) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	operationlog.Register(mux, operationLogProcessor)
	return mux
}

func openWorkerRedis(ctx context.Context, redisURL string) (*projectredis.Client, error) {
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return nil, fmt.Errorf("open Worker Redis: %w", err)
	}
	return client, nil
}
