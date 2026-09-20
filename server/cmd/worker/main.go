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
	"admin/server/internal/module/system/scheduler"
	systemsetting "admin/server/internal/module/system/setting"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/shared/cacheGeneration"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// workerRelayShutdownBudget 是取消 runner 后等待其退出的上限。
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
	schedulerRepository := scheduler.NewRepository(postgres.GORM)
	var notificationProcessor *notificationtask.Processor
	notificationDefinition := scheduler.NotificationBatchDefinition(func(ctx context.Context, execution scheduler.ExecutionContext) error {
		payload, err := notificationtask.DecodeBatchPayload(execution.Payload)
		if err != nil {
			return err
		}
		return notificationProcessor.Process(ctx, payload)
	})
	batchJobWriter, err := scheduler.NewBatchJobWriter(schedulerRepository, notificationDefinition)
	if err != nil {
		return err
	}
	notificationProcessor = notificationtask.NewProcessor(postgres.GORM, realtimeRepository, batchJobWriter)
	configGenerationRepository := cachegeneration.NewRepository(postgres.GORM)
	configGenerationStore := cachegeneration.NewStore(redisClient)
	settingService, err := buildWorkerSettingService(postgres.GORM, redisClient, configGenerationRepository, configGenerationStore, logger)
	if err != nil {
		return err
	}
	configRelay := cachegeneration.NewRelay(configGenerationRepository, configGenerationStore, logger)
	realtimeRelay := realtime.NewRelay(realtimeRepository, redisClient, logger)
	realtimeRetention := realtime.NewRetentionService(realtimeRepository, settingService)
	notificationRetention := notification.NewRetentionService(notificationRepository, settingService)
	historyCleaner := scheduler.NewHistoryCleaner(schedulerRepository, settingService)
	catalog, err := scheduler.NewTaskCatalog(append(scheduler.BuiltinDefinitions(realtimeRetention, notificationRetention, historyCleaner), notificationDefinition)...)
	if err != nil {
		return err
	}
	scanner := scheduler.NewScanner(schedulerRepository, catalog)
	publisher := scheduler.NewPublisher(schedulerRepository, scheduler.NewAsynqEnqueuer(queueClient), catalog, logger)
	executor := scheduler.NewExecutor(schedulerRepository, catalog, fmt.Sprintf("worker-%d", os.Getpid()))
	mux := buildWorkerMux(operationLogService, scheduler.NewWorkerHandler(executor))

	return runWorkerAssembly(workerAssembly{
		ProcessContext: processContext,
		Logger:         logger,
		Mux:            mux,
		Runners: []namedRunner{
			{Name: "config-generation", Runner: configRelay},
			{Name: "realtime-outbox", Runner: realtimeRelay},
			{Name: "scheduler-scanner", Runner: scanner},
			{Name: "scheduler-publisher", Runner: publisher},
		},
		NewServer: func() (asynqServer, error) {
			return queue.NewServer(settings.RedisURL)
		},
	})
}

func buildWorkerSettingService(db *gorm.DB, redisClient *projectredis.Client, generations *cachegeneration.Repository, states *cachegeneration.Store, logger *slog.Logger) (*systemsetting.Service, error) {
	if db == nil || redisClient == nil || generations == nil || states == nil {
		return nil, errors.New("worker setting cache generation dependencies are required")
	}
	repository := systemsetting.NewRepository(db)
	repository.SetGenerations(generations)
	cache := systemsetting.NewCache(redisClient)
	cache.SetStateStore(states)
	service := systemsetting.NewService(repository)
	service.SetCache(cache)
	service.SetGenerations(generations, states)
	service.SetLogger(logger)
	if err := service.ValidateDependencies(); err != nil {
		return nil, fmt.Errorf("configure Worker system setting service: %w", err)
	}
	return service, nil
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

func buildWorkerMux(operationLogProcessor operationlog.Processor, schedulerHandler asynq.Handler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	operationlog.Register(mux, operationLogProcessor)
	mux.Handle(scheduler.EnvelopeTaskType, schedulerHandler)
	return mux
}

func openWorkerRedis(ctx context.Context, redisURL string) (*projectredis.Client, error) {
	client, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		return nil, fmt.Errorf("open Worker Redis: %w", err)
	}
	return client, nil
}
