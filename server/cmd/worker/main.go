package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	aimedia "admin/server/internal/module/ai/media"
	aiprovider "admin/server/internal/module/ai/provider"
	providerrequest "admin/server/internal/module/ai/providerRequest"
	airun "admin/server/internal/module/ai/run"
	runevent "admin/server/internal/module/ai/run/event"
	aitool "admin/server/internal/module/ai/tool"
	aiusage "admin/server/internal/module/ai/usage"
	"admin/server/internal/module/message/notification"
	notificationtask "admin/server/internal/module/message/notificationTask"
	callbackevent "admin/server/internal/module/payment/callbackEvent"
	paymentconfig "admin/server/internal/module/payment/config"
	paymentorder "admin/server/internal/module/payment/order"
	rechargepackage "admin/server/internal/module/payment/rechargePackage"
	"admin/server/internal/module/payment/transaction"
	"admin/server/internal/module/payment/wallet"
	permissionnotification "admin/server/internal/module/permission/notification"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/storage/cosConfig"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/module/system/operationLog"
	"admin/server/internal/module/system/scheduler"
	systemsetting "admin/server/internal/module/system/setting"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/configCache"
	storagecos "admin/server/internal/storage/cos"

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

type aiWorkerExecutor struct {
	text  *airun.TextEngine
	media *aimedia.Engine
}

func (e *aiWorkerExecutor) Execute(ctx context.Context, row airun.Model) error {
	if e == nil {
		return errors.New("AI worker executor is unavailable")
	}
	if row.Operation == "text.generate" {
		if e.text == nil {
			return errors.New("AI text engine is unavailable")
		}
		return e.text.Execute(ctx, row)
	}
	if e.media == nil {
		return errors.New("AI media engine is unavailable")
	}
	return e.media.Execute(ctx, row)
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
	notificationProcessor = notificationtask.NewProcessor(postgres.GORM, realtimeRepository, batchJobWriter, func(db *gorm.DB) permissionnotification.Reader {
		return permissionnotification.NewRepository(db)
	})
	configCacheRepository := configcache.NewRepository(postgres.GORM)
	configCacheStore := configcache.NewStore(redisClient)
	keys, err := secretkey.New(os.Getenv("APP_SECRET"))
	if err != nil {
		return fmt.Errorf("derive Worker storage keys: %w", err)
	}
	paymentConfigRepository := paymentconfig.NewRepository(postgres.GORM)
	paymentConfigRepository.SetGenerations(configCacheRepository)
	paymentConfigService := paymentconfig.NewService(paymentConfigRepository, keys)
	paymentConfigService.SetGenerations(configCacheRepository, configCacheStore)
	paymentConfigService.SetCache(paymentconfig.NewCache(redisClient, configCacheStore))
	paymentOrderRepository := paymentorder.NewRepository(postgres.GORM)
	paymentOrderService := paymentorder.NewService(paymentOrderRepository)
	paymentOrderService.SetRuntime(paymentConfigService, rechargepackage.NewService(rechargepackage.NewRepository(postgres.GORM)))
	paymentCallbackService := callbackevent.NewService(callbackevent.NewRepository(postgres.GORM))
	paymentCallbackService.SetRuntime(paymentConfigService)
	paymentCallbackService.SetConfirmer(paymentOrderService)
	paymentReconciler := paymentorder.NewReconciler(paymentOrderRepository, paymentConfigService, paymentOrderService, nil)
	paymentReconciler.SetPendingRecovery(paymentCallbackService)
	paymentReconciler.SetChannelLimiter(paymentorder.NewRedisChannelLimiter(redisClient.UniversalClient()))
	paymentDefinition := scheduler.PaymentReconcileDefinition(func(ctx context.Context, _ scheduler.ExecutionContext) error {
		return paymentReconciler.Run(ctx)
	})
	cosClient := storagecos.NewClient(nil)
	cosRepository := cosconfig.NewRepository(postgres.GORM)
	cosRepository.SetGenerations(configCacheRepository)
	cosService := cosconfig.NewService(cosRepository, keys, cosClient)
	cosCache := cosconfig.NewCache(redisClient)
	cosCache.SetStateStore(configCacheStore)
	cosService.SetCache(cosCache)
	cosService.SetGenerations(configCacheRepository, configCacheStore)
	cosService.SetLogger(logger)
	uploadService := uploadrule.NewService(uploadrule.NewRepository(postgres.GORM), keys, cosClient, cosService, uploadrule.NewRouteCache(redisClient))
	fileService := storagefile.NewService(storagefile.NewRepository(postgres.GORM, schedulerRepository), keys, cosClient, cosService, uploadService)
	settingService, err := buildWorkerSettingService(postgres.GORM, redisClient, configCacheRepository, configCacheStore, logger)
	if err != nil {
		return err
	}
	configRelay := configcache.NewRelay(configCacheRepository, configCacheStore, logger)
	realtimeRelay := realtime.NewRelay(realtimeRepository, redisClient, logger)
	realtimeRetention := realtime.NewRetentionService(realtimeRepository, settingService)
	notificationRetention := notification.NewRetentionService(notificationRepository, settingService)
	historyCleaner := scheduler.NewHistoryCleaner(schedulerRepository, settingService)
	// AI execution dependencies are assembled explicitly here; credentials and
	// tool versions are loaded from the frozen Run snapshot only.
	providerCipher, err := aiprovider.NewCipher(os.Getenv("APP_SECRET"), "provider-v1")
	if err != nil {
		return err
	}
	toolCipher, err := aitool.NewCipher(os.Getenv("APP_SECRET"), "tool-v1")
	if err != nil {
		return err
	}
	aiRunRepository := airun.NewRepository(postgres.GORM, wallet.NewRepository(postgres.GORM), schedulerRepository)
	aiEventRepository := runevent.NewRepository(postgres.GORM, realtimeRepository)
	aiRunRepository.SetEvents(aiEventRepository)
	aiToolRepository := aitool.NewRepository(postgres.GORM)
	aiToolExecutor := aitool.NewExecutor(nil, toolCipher)
	aiCredentialLoader := airun.NewCredentialLoader(aiprovider.NewRepository(postgres.GORM), providerCipher)
	aiTextEngine := airun.NewTextEngine(aiRunRepository, aiCredentialLoader, aiToolRepository, aiToolExecutor, providerrequest.NewResponsesClient(nil), providerrequest.NewChatCompletionsClient(nil))
	aiTextEngine.SetFileReader(fileService)
	aiRunGate := airun.NewGate(redisClient, aiRunRepository)
	aiTextEngine.SetStreamPressure(aiRunGate)
	mediaCipher, err := aiprovider.NewCipher(os.Getenv("APP_SECRET"), "media-v1")
	if err != nil {
		return err
	}
	spoolLimit, err := workerAIPositiveSetting(processContext, settingService, "ai.media.spool_max_bytes_per_worker")
	if err != nil {
		return err
	}
	mediaSpool, err := aimedia.NewSpooler(os.TempDir(), spoolLimit)
	if err != nil {
		return fmt.Errorf("configure AI media spool: %w", err)
	}
	aiMediaService := aimedia.NewService(aimedia.NewRepository(postgres.GORM, schedulerRepository), aiRunRepository, aiCredentialLoader, providerrequest.NewVideoClient(nil), mediaSpool, fileService, mediaCipher, aiEventRepository)
	if err := aiMediaService.ValidateDependencies(); err != nil {
		return err
	}
	imageClient := providerrequest.NewImagesClient(nil)
	aiMediaEngine := aimedia.NewEngine(aiRunRepository, aiCredentialLoader, imageClient, imageClient, providerrequest.NewAudioClient(nil), fileService, aiMediaService)
	aiRunWorker := &airun.Worker{Loader: aiRunRepository, Executor: &aiWorkerExecutor{text: aiTextEngine, media: aiMediaEngine}}
	aiUsageRepository := aiusage.NewRepository(postgres.GORM, wallet.NewRepository(postgres.GORM), transaction.NewRepository(postgres.GORM), aiEventRepository)
	aiUsageRepository.SetScheduler(schedulerRepository)
	aiUsageService := aiusage.NewService(aiUsageRepository)
	aiRunExecute := func(ctx context.Context, ec scheduler.ExecutionContext) error {
		return aiRunWorker.Execute(ctx, ec.Payload)
	}
	aiUsageSettle := func(ctx context.Context, ec scheduler.ExecutionContext) error {
		var payload struct {
			ChargeID int64 `json:"chargeId"`
		}
		if err := json.Unmarshal(ec.Payload, &payload); err != nil || payload.ChargeID < 1 {
			return errors.New("AI settlement payload is invalid")
		}
		return aiUsageService.Settle(ctx, payload.ChargeID)
	}
	aiRunRecover := func(ctx context.Context, _ scheduler.ExecutionContext) error {
		if _, err := aiRunRepository.RecoverExpired(ctx, time.Now().UTC(), 100); err != nil {
			return err
		}
		if _, err := aiRunRepository.RecoverStaleEffects(ctx, time.Now().UTC(), 100); err != nil {
			return err
		}
		if _, err := aiRunRepository.RecoverQueued(ctx, time.Now().UTC(), 100); err != nil {
			return err
		}
		if _, err := aiMediaService.RecoverDue(ctx, time.Now().UTC(), 100); err != nil {
			return err
		}
		_, err := aiUsageRepository.RecoverSettlements(ctx, time.Now().UTC(), 100)
		return err
	}
	aiEventCleanup := func(ctx context.Context, _ scheduler.ExecutionContext) error {
		days, err := workerAIPositiveSetting(ctx, settingService, "ai.run.event_retention_days")
		if err != nil || days > int64((1<<63-1)/int64(24*time.Hour)) {
			return errors.New("AI event retention policy invalid")
		}
		_, err = aiEventRepository.Cleanup(ctx, time.Now().UTC().Add(-time.Duration(days)*24*time.Hour), 1000)
		return err
	}
	aiDefinitions, err := scheduler.AITaskDefinitions(scheduler.AITaskExecutors{
		RunExecute: aiRunExecute, MediaPoll: func(ctx context.Context, _ scheduler.ExecutionContext) error { return aiMediaService.PollOne(ctx) }, MediaImport: func(ctx context.Context, _ scheduler.ExecutionContext) error { return aiMediaService.ImportOne(ctx) },
		UsageSettle: aiUsageSettle, RunRecover: aiRunRecover, RunEventCleanup: aiEventCleanup,
	}, 5*time.Minute, 5*time.Minute)
	if err != nil {
		return err
	}
	definitions := append(scheduler.BuiltinDefinitions(realtimeRetention, notificationRetention, historyCleaner), aiDefinitions...)
	definitions = append(definitions, notificationDefinition, paymentDefinition, storagefile.CleanupDefinition(fileService), storagefile.ScanDefinition(fileService))
	catalog, err := scheduler.NewTaskCatalog(definitions...)
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
			{Name: "ai-capacity", Runner: aiCapacityRunner{gate: aiRunGate, policies: airun.NewPolicyReader(settingService)}},
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

func buildWorkerSettingService(db *gorm.DB, redisClient *projectredis.Client, generations *configcache.Repository, states *configcache.Store, logger *slog.Logger) (*systemsetting.Service, error) {
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

func workerAIPositiveSetting(ctx context.Context, settings *systemsetting.Service, key string) (int64, error) {
	if settings == nil || key == "" {
		return 0, errors.New("AI media setting reader unavailable")
	}
	record, err := settings.Find(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("read AI media setting %s: %w", key, err)
	}
	value, err := strconv.ParseInt(record.Value, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("AI media setting %s is invalid", key)
	}
	return value, nil
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
