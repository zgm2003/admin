package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	"admin/server/internal/module/message/notification"
	notificationtask "admin/server/internal/module/message/notificationTask"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/system/operationLog"
	systemsetting "admin/server/internal/module/system/setting"
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

type namedRunner struct {
	Name   string
	Runner relayRunner
}

type runnerResult struct {
	name string
	err  error
}

type asynqServer interface {
	Start(handler asynq.Handler) error
	Shutdown()
}

type workerAssembly struct {
	ProcessContext context.Context
	Logger         *slog.Logger
	Mux            *asynq.ServeMux
	Runners        []namedRunner
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
	queueClient, err := queue.NewClient(settings.RedisURL)
	if err != nil {
		return err
	}
	defer queueClient.Close()

	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
	realtimeRepository := realtime.NewRepository(postgres.GORM)
	notificationRepository := notification.NewRepository(postgres.GORM, realtimeRepository)
	notificationTaskRepository := notificationtask.NewRepository(postgres.GORM)
	notificationTaskHandler := notificationtask.NewTaskHandler(notificationtask.NewProcessor(postgres.GORM, realtimeRepository), notificationTaskRepository)
	mux := buildWorkerMux(operationLogService, notificationTaskHandler)
	settingService := systemsetting.NewService(systemsetting.NewRepository(postgres.GORM))
	configRelay := cachegeneration.NewRelay(cachegeneration.NewRepository(postgres.GORM), cachegeneration.NewStore(redisClient), logger)
	realtimeRelay := realtime.NewRelay(realtimeRepository, redisClient, logger)
	dispatchRelay := notificationtask.NewDispatchRelay(notificationTaskRepository, notificationtask.NewQueueEnqueuer(queueClient), logger)
	realtimeRetention := realtime.NewRetentionTrigger(realtime.NewRetentionService(realtimeRepository, settingService), logger)
	notificationRetention := notification.NewRetentionTrigger(notification.NewRetentionService(notificationRepository, settingService), logger)

	return runWorkerAssembly(workerAssembly{
		ProcessContext: processContext,
		Logger:         logger,
		Mux:            mux,
		Runners: []namedRunner{
			{Name: "config-generation", Runner: configRelay},
			{Name: "realtime-outbox", Runner: realtimeRelay},
			{Name: "notification-dispatch", Runner: dispatchRelay},
			{Name: "realtime-retention-trigger", Runner: realtimeRetention},
			{Name: "notification-retention-trigger", Runner: notificationRetention},
		},
		NewServer: func() (asynqServer, error) {
			return queue.NewServer(settings.RedisURL)
		},
	})
}

// runWorkerAssembly 启动 relay goroutine 后启动 Asynq；任一启动失败都会
// 关闭已启动资源。进程 context 取消时先停 relay 再 Shutdown Asynq。
func runWorkerAssembly(assembly workerAssembly) error {
	if assembly.ProcessContext == nil || assembly.Mux == nil || len(assembly.Runners) == 0 || assembly.NewServer == nil {
		return fmt.Errorf("worker assembly requires context, mux, named runners and server factory")
	}
	for _, runner := range assembly.Runners {
		if runner.Name == "" || runner.Runner == nil {
			return fmt.Errorf("worker assembly contains invalid named runner")
		}
	}
	server, err := assembly.NewServer()
	if err != nil {
		return fmt.Errorf("build Asynq Worker: %w", err)
	}

	runnerContext, stopRunners := context.WithCancel(assembly.ProcessContext)
	runnerDone := make(chan runnerResult, len(assembly.Runners))
	for _, runner := range assembly.Runners {
		runner := runner
		go func() { runnerDone <- runnerResult{name: runner.Name, err: runner.Runner.Run(runnerContext)} }()
	}

	if err := server.Start(assembly.Mux); err != nil {
		stopRunners()
		waitForRunnerShutdown(assembly.Logger, runnerDone, len(assembly.Runners))
		return fmt.Errorf("start Asynq Worker: %w", err)
	}

	select {
	case <-assembly.ProcessContext.Done():
		stopRunners()
		waitForRunnerShutdown(assembly.Logger, runnerDone, len(assembly.Runners))
		server.Shutdown()
		return nil
	case result := <-runnerDone:
		stopRunners()
		waitForRunnerShutdown(assembly.Logger, runnerDone, len(assembly.Runners)-1)
		server.Shutdown()
		if result.err == nil {
			result.err = errors.New("runner stopped unexpectedly")
		}
		return fmt.Errorf("runner %s stopped: %w", result.name, result.err)
	}
}

func waitForRunnerShutdown(logger *slog.Logger, runnerDone <-chan runnerResult, count int) {
	timer := time.NewTimer(workerRelayShutdownBudget)
	defer timer.Stop()
	for count > 0 {
		select {
		case result := <-runnerDone:
			count--
			if result.err != nil && logger != nil {
				logger.Error("worker runner stopped with error", "runner", result.name, "error", result.err)
			}
		case <-timer.C:
			if logger != nil {
				logger.Error("worker runners did not stop within the shutdown budget", "remaining", count)
			}
			return
		}
	}
}

func buildWorkerMux(operationLogProcessor operationlog.Processor, notificationHandler asynq.Handler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	operationlog.Register(mux, operationLogProcessor)
	mux.Handle(notificationtask.TaskType, notificationHandler)
	return mux
}

func openWorkerRedis(ctx context.Context, redisURL string) (*projectredis.Client, error) {
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return nil, fmt.Errorf("open Worker Redis: %w", err)
	}
	return client, nil
}
