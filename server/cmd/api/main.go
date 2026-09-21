package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"admin/server/internal/config"
	"admin/server/internal/database"
	projectmiddleware "admin/server/internal/middleware"
	authcaptcha "admin/server/internal/module/auth/captcha"
	"admin/server/internal/module/auth/client"
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/auth/state"
	"admin/server/internal/module/health"
	messagemail "admin/server/internal/module/message/mail"
	mailconfig "admin/server/internal/module/message/mail/config"
	maillog "admin/server/internal/module/message/mail/log"
	ratelimitpolicy "admin/server/internal/module/message/mail/rateLimitPolicy"
	recipientrule "admin/server/internal/module/message/mail/recipientRule"
	mailtemplate "admin/server/internal/module/message/mail/template"
	"admin/server/internal/module/message/notification"
	notificationtask "admin/server/internal/module/message/notificationTask"
	messagesms "admin/server/internal/module/message/sms"
	smsconfig "admin/server/internal/module/message/sms/config"
	smslog "admin/server/internal/module/message/sms/log"
	smslogverification "admin/server/internal/module/message/sms/logVerification"
	smsratelimitpolicy "admin/server/internal/module/message/sms/rateLimitPolicy"
	smsrecipientrule "admin/server/internal/module/message/sms/recipientRule"
	smstemplate "admin/server/internal/module/message/sms/template"
	"admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/module/permission/menu"
	permissionnotification "admin/server/internal/module/permission/notification"
	"admin/server/internal/module/permission/role"
	"admin/server/internal/module/permission/state"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/module/storage/uploadRule"
	systemcachegeneration "admin/server/internal/module/system/cacheGeneration"
	"admin/server/internal/module/system/dictionary"
	"admin/server/internal/module/system/operationLog"
	"admin/server/internal/module/system/queueMonitor"
	"admin/server/internal/module/system/scheduler"
	systemsetting "admin/server/internal/module/system/setting"
	account "admin/server/internal/module/user/account"
	useremail "admin/server/internal/module/user/email"
	"admin/server/internal/module/user/loginLog"
	userphone "admin/server/internal/module/user/phone"
	profile "admin/server/internal/module/user/profile"
	usersession "admin/server/internal/module/user/session"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/i18n"
	storagecos "admin/server/internal/storage/cos"
	storagemail "admin/server/internal/storage/mail"
	storagesms "admin/server/internal/storage/sms"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type routerDependencies struct {
	CORSOrigin        string
	TrustedProxies    []string
	Logger            *slog.Logger
	Health            *health.Handler
	Auth              *auth.Handler
	Captcha           *authcaptcha.Handler
	AuthPlatform      *authplatform.Handler
	Permission        *permission.Handler
	Menu              *menu.Handler
	Role              *role.Handler
	User              *account.Handler
	Account           *profile.Handler
	Phone             *userphone.Handler
	Email             *useremail.Handler
	COSConfig         *cosconfig.Handler
	UploadRule        *uploadrule.Handler
	OperationLog      *operationlog.Handler
	Dictionary        *dictionary.Handler
	Setting           *systemsetting.Handler
	CacheGeneration   *systemcachegeneration.Handler
	QueueMonitor      *queuemonitor.Handler
	QueueMonitorUI    http.Handler
	LoginLog          *loginlog.Handler
	Mail              *messagemail.Handler
	MailConfig        *mailconfig.Handler
	MailTemplate      *mailtemplate.Handler
	MailLog           *maillog.Handler
	MailRateLimit     *ratelimitpolicy.Handler
	MailRecipientRule *recipientrule.Handler
	SMS               *messagesms.Handler
	SMSConfig         *smsconfig.Handler
	SMSTemplate       *smstemplate.Handler
	SMSLog            *smslog.Handler
	SMSRateLimit      *smsratelimitpolicy.Handler
	SMSRecipientRule  *smsrecipientrule.Handler
	Realtime          *realtime.Handler
	Notification      *notification.Handler
	NotificationTask  *notificationtask.Handler
	Scheduler         *scheduler.Handler
	OperationEnqueuer operationlog.Enqueuer
	SessionAdmin      *usersession.SessionAdminHandler
	AuthOrigin        gin.HandlerFunc
	Authenticate      gin.HandlerFunc
	RequirePermission func(string) gin.HandlerFunc
}

type runtimeDependency struct {
	name     string
	validate func() error
}

type realtimeSubscriber interface {
	Ready() <-chan struct{}
	Run(context.Context) error
}

type realtimeConnections interface {
	BeginDrain()
	CloseAll()
}

type httpRuntime interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func validateRuntimeDependencies(dependencies ...runtimeDependency) error {
	for _, dependency := range dependencies {
		if dependency.name == "" || dependency.validate == nil {
			return fmt.Errorf("runtime dependency declaration is invalid")
		}
		if err := dependency.validate(); err != nil {
			return fmt.Errorf("validate runtime dependency %s: %w", dependency.name, err)
		}
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := i18n.ValidateCatalogs(); err != nil {
		return fmt.Errorf("validate i18n catalogs: %w", err)
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	settings, err := config.LoadAPI(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load API config: %w", err)
	}

	processContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	postgres, err := database.Open(processContext, settings.PostgresDSN)
	if err != nil {
		return err
	}
	defer postgres.Close()

	redisClient, err := projectredis.Open(processContext, settings.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	queueClient, err := queue.NewClient(settings.RedisURL)
	if err != nil {
		return err
	}
	defer queueClient.Close()
	queueMonitorUI, err := queuemonitor.NewMonitor(settings.RedisURL)
	if err != nil {
		return fmt.Errorf("build queue monitor: %w", err)
	}
	defer queueMonitorUI.Close()
	queueMonitorService := queuemonitor.NewService(queuemonitor.NewRedisGrantStore(redisClient))
	accessStateStore := permissionstate.NewStore(redisClient)
	accessInvalidator := permissionstate.NewInvalidator(accessStateStore)
	menuRepository := menu.NewRepository(postgres.GORM)
	menuStateStore := permissionstate.NewMenuStore(redisClient)
	menuService := menu.NewService(menuRepository, menuStateStore)

	roleRepository := role.NewRepository(postgres.GORM)
	roleService := role.NewService(roleRepository, accessInvalidator, notification.BasePermissionCodes())
	keys, err := secretkey.New(settings.AppSecret)
	if err != nil {
		return fmt.Errorf("derive application keys: %w", err)
	}

	healthService := health.NewService(postgres, redisClient)
	userRepository := account.NewRepository(postgres.GORM)
	configGenerationRepository := cachegeneration.NewRepository(postgres.GORM)
	configGenerationStore := cachegeneration.NewStore(redisClient)
	settingRepository := systemsetting.NewRepository(postgres.GORM)
	settingRepository.SetGenerations(configGenerationRepository)
	settingService := systemsetting.NewService(settingRepository)
	settingCache := systemsetting.NewCache(redisClient)
	settingCache.SetStateStore(configGenerationStore)
	settingService.SetCache(settingCache)
	cacheGenerationService := systemcachegeneration.NewService(systemcachegeneration.NewRepository(postgres.GORM), configGenerationStore)
	settingService.SetGenerations(configGenerationRepository, configGenerationStore)
	settingService.SetLogger(logger)
	profileRepository := profile.NewRepository(postgres.GORM)
	sessionRepository := usersession.NewRepository(postgres.GORM)
	authPlatformRepository := authplatform.NewRepository(postgres.GORM)
	mailRateLimitRepository := ratelimitpolicy.NewRepository(postgres.GORM)
	mailGenerationScope := messagemail.CacheGenerationScope()
	smsGenerationScope := messagesms.CacheGenerationScope()
	authPlatformRepository.SetRateLimitPolicyLifecycle(
		func(ctx context.Context, tx *gorm.DB, platformID int64, expected authplatform.RateLimitGenerationBases, now time.Time) (authplatform.RateLimitGenerationEvents, error) {
			if err := ratelimitpolicy.NewRepository(tx).ProvisionDefaults(ctx, platformID); err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			if err := smsratelimitpolicy.NewRepository(tx).ProvisionDefaults(ctx, platformID, now); err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			mailEvent, err := configGenerationRepository.AdvanceTx(ctx, tx, mailGenerationScope, expected.Mail, now)
			if err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			smsEvent, err := configGenerationRepository.AdvanceTx(ctx, tx, smsGenerationScope, expected.SMS, now)
			if err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			return authplatform.RateLimitGenerationEvents{Mail: mailEvent, SMS: smsEvent}, nil
		},
		func(ctx context.Context, tx *gorm.DB, platformID int64, expected authplatform.RateLimitGenerationBases, now time.Time) (authplatform.RateLimitGenerationEvents, error) {
			if err := ratelimitpolicy.NewRepository(tx).DeleteForPlatform(ctx, platformID); err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			if err := smsratelimitpolicy.NewRepository(tx).DeleteForPlatform(ctx, platformID); err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			mailEvent, err := configGenerationRepository.AdvanceTx(ctx, tx, mailGenerationScope, expected.Mail, now)
			if err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			smsEvent, err := configGenerationRepository.AdvanceTx(ctx, tx, smsGenerationScope, expected.SMS, now)
			if err != nil {
				return authplatform.RateLimitGenerationEvents{}, err
			}
			return authplatform.RateLimitGenerationEvents{Mail: mailEvent, SMS: smsEvent}, nil
		},
	)
	policyStore := authplatform.NewPolicyStore(redisClient)
	authStateStore := authstate.NewStore(redisClient)
	authInvalidator := authstate.NewInvalidator(authStateStore)
	authPlatformService := authplatform.NewService(authPlatformRepository, policyStore, redisClient, authStateStore, authInvalidator, auth.NewSessionCache(redisClient).Delete, logger, authplatform.Deployment{
		CookieSecure: settings.Auth.CookieSecure, CORSOrigin: settings.CORSOrigin,
		TrustedProxyMode: settings.TrustedProxyMode, TrustedProxyCount: len(settings.TrustedProxies),
	})
	authPlatformService.SetRateLimitCacheGenerations(configGenerationRepository, configGenerationStore, mailGenerationScope, smsGenerationScope)
	authService := auth.NewService(
		userRepository,
		roleRepository,
		sessionRepository,
		authPlatformService,
		authStateStore,
		authInvalidator,
		auth.NewSessionCache(redisClient),
		redisClient,
		auth.NewJWT(keys.JWTSigningKey()),
		keys.RefreshTokenHMACKey(),
		logger,
	)
	captchaEngine, err := authcaptcha.NewSlideEngine()
	if err != nil {
		return fmt.Errorf("build captcha engine: %w", err)
	}
	captchaService := authcaptcha.NewService(captchaEngine, authcaptcha.NewRedisStore(redisClient, "captcha:slide:"), settingService)
	authService.SetCaptchaVerifier(captchaService)
	authService.SetPasswordStore(userRepository)
	sessionService := usersession.NewService(sessionRepository, authStateStore, authInvalidator, auth.NewSessionCache(redisClient))
	userService := account.NewService(userRepository, authStateStore, authInvalidator, accessStateStore, accessInvalidator)
	profileService := profile.NewService(profileRepository)
	cosClient := storagecos.NewClient(nil)
	cosConfigRepository := cosconfig.NewRepository(postgres.GORM)
	cosConfigRepository.SetGenerations(configGenerationRepository)
	cosConfigService := cosconfig.NewService(cosConfigRepository, keys, cosClient)
	cosConfigCache := cosconfig.NewCache(redisClient)
	cosConfigCache.SetStateStore(configGenerationStore)
	cosConfigService.SetCache(cosConfigCache)
	cosConfigService.SetGenerations(configGenerationRepository, configGenerationStore)
	cosConfigService.SetLogger(logger)
	uploadRuleRepository := uploadrule.NewRepository(postgres.GORM)
	uploadRouteCache := uploadrule.NewRouteCache(redisClient)
	uploadRuleService := uploadrule.NewService(uploadRuleRepository, keys, cosClient, cosConfigService, uploadRouteCache)
	loginLogService := loginlog.NewService(loginlog.NewRepository(postgres.GORM))
	mailStores := messagemail.NewStores(postgres.GORM)
	mailLimiter := messagemail.NewRedisLimiter(redisClient.UniversalClient())
	mailRateLimitRepository.SetGenerations(configGenerationRepository, messagemail.CacheGenerationScope())
	mailRateLimitStore := ratelimitpolicy.NewStore(mailRateLimitRepository, redisClient)
	mailRateLimitStore.SetGenerations(configGenerationRepository, configGenerationStore)
	mailRateLimitService := ratelimitpolicy.NewService(mailRateLimitRepository)
	mailRuntimeStore := messagemail.NewRuntimeStore(mailStores, redisClient)
	mailRuntimeStore.SetGenerations(configGenerationRepository, configGenerationStore)
	mailRuntimeStore.SetLogger(logger)
	mailRateLimitService.SetRuntimeCoordinator(mailRuntimeStore)
	mailStores.Config.SetGenerations(configGenerationRepository, messagemail.CacheGenerationScope())
	mailStores.Template.SetGenerations(configGenerationRepository, messagemail.CacheGenerationScope())
	mailStores.RecipientRule.SetGenerations(configGenerationRepository, messagemail.CacheGenerationScope())
	mailRecipientRuleRepository := mailStores.RecipientRule
	mailRecipientRuleService := recipientrule.NewService(mailRecipientRuleRepository)
	mailService := messagemail.NewService(mailStores, keys, storagemail.NewTencentSESClient(nil), mailRecipientRuleService, mailLimiter, mailRateLimitStore)
	mailService.SetVerifyCodeReadinessStore(mailRuntimeStore)
	mailService.SetRuntimeStore(mailRuntimeStore)
	mailConfigService := mailconfig.NewService(mailStores.Config, keys)
	mailConfigService.SetRuntimeCoordinator(mailRuntimeStore)
	mailTemplateService := mailtemplate.NewService(mailStores.Template)
	mailTemplateService.SetRuntimeCoordinator(mailRuntimeStore)
	mailLogService := maillog.NewService(mailStores.Log, mailStores.LogVerification, keys)
	mailRecipientRuleService.SetRuntimeCoordinator(mailRuntimeStore)
	smsRuntimeCache := messagesms.NewRuntimeCache(redisClient)
	smsRuntimeCache.SetGenerations(configGenerationRepository, configGenerationStore)
	smsRuntimeCache.SetLogger(logger)
	smsConfigRepository := smsconfig.NewRepository(postgres.GORM)
	smsConfigRepository.SetGenerations(configGenerationRepository, messagesms.CacheGenerationScope())
	smsConfigService := smsconfig.NewService(smsConfigRepository, keys, smsRuntimeCache)
	smsTemplateRepository := smstemplate.NewRepository(postgres.GORM)
	smsTemplateRepository.SetGenerations(configGenerationRepository, messagesms.CacheGenerationScope())
	smsTemplateService := smstemplate.NewService(smsTemplateRepository)
	smsTemplateService.SetRuntimeCoordinator(smsRuntimeCache)
	smsRecipientRuleRepository := smsrecipientrule.NewRepository(postgres.GORM)
	smsRecipientRuleRepository.SetGenerations(configGenerationRepository, messagesms.CacheGenerationScope())
	smsRecipientRuleService := smsrecipientrule.NewService(smsRecipientRuleRepository, keys)
	smsRecipientRuleService.SetRuntimeCoordinator(smsRuntimeCache)
	smsRateLimitRepository := smsratelimitpolicy.NewRepository(postgres.GORM)
	smsRateLimitRepository.SetGenerations(configGenerationRepository, messagesms.CacheGenerationScope())
	smsRateLimitStore := smsratelimitpolicy.NewStore(redisClient)
	smsRateLimitStore.SetGenerations(configGenerationRepository, configGenerationStore)
	smsRateLimitService := smsratelimitpolicy.NewService(smsRateLimitRepository, smsRateLimitStore)
	smsRateLimitService.SetRuntimeCoordinator(smsRuntimeCache)
	smsLogRepository := smslog.NewRepository(postgres.GORM)
	smsLogVerificationRepository := smslogverification.NewRepository(postgres.GORM)
	smsLogService := smslog.NewService(smsLogRepository, smsLogVerificationRepository, keys)
	smsService := messagesms.NewService(messagesms.Stores{
		Runtime:      smsRuntimeCache,
		Config:       smsConfigRepository,
		Template:     smsTemplateRepository,
		Rule:         smsRecipientRuleRepository,
		Log:          smsLogRepository,
		Verification: smsLogVerificationRepository,
		Policy:       smsRateLimitService,
		Limiter:      messagesms.NewRedisLimiter(redisClient.UniversalClient()),
		Sender:       storagesms.NewTencentSMSClient(nil),
	}, keys)
	verificationStore := auth.NewVerificationCodeStore(redisClient, keys.VerificationCodeHMACKey())
	authService.SetVerifyCodeSender(mailService)
	authService.SetPhoneVerifyCodeSender(smsService)
	authService.SetVerificationCodeStore(verificationStore)
	authService.SetLoginLogRecorder(loginLogService)
	phoneService := userphone.NewService(userphone.NewRepository(postgres.GORM), smsService, verificationStore, keys, userphone.NewAuthorityCoordinator(authStateStore, authInvalidator))
	emailService := useremail.NewService(useremail.NewRepository(postgres.GORM), mailService, verificationStore, keys, useremail.NewAuthorityCoordinator(authStateStore, authInvalidator))
	permissionRepository := permission.NewRepository(postgres.GORM)
	permissionService := permission.NewService(permissionRepository, accessStateStore, permission.NewSnapshotCache(redisClient), permission.NewLocalSnapshotCache(1024), logger, menuStateStore)
	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
	dictionaryRepository := dictionary.NewRepository(postgres.GORM)
	dictionaryRepository.SetGenerations(configGenerationRepository, dictionary.CacheGenerationScope())
	dictionaryCache := dictionary.NewOptionsCache(redisClient)
	dictionaryCache.SetStateStore(configGenerationStore)
	dictionaryService := dictionary.NewService(dictionaryRepository)
	dictionaryService.SetCache(dictionaryCache)
	dictionaryService.SetGenerations(configGenerationRepository, configGenerationStore)
	dictionaryService.SetLogger(logger)
	realtimeRepository := realtime.NewRepository(postgres.GORM)
	realtimeTickets := realtime.NewTicketStore(redisClient)
	realtimeConnections := realtime.NewConnectionSet(settings.Realtime.MaxConnections, settings.Realtime.MaxConnectionsPerUser)
	realtimeSubscriber := realtime.NewSubscriber(redisClient, realtimeConnections, func(errorClass string) {
		logger.Error("realtime subscriber error", "errorClass", errorClass)
	})
	realtimeService := realtime.NewService(realtimeTickets, authService, realtimeRepository, settings.Realtime.ResumeConcurrency)
	realtimeHandler := realtime.NewHandler(realtimeService, realtimeConnections, settings.CORSOrigin)
	notificationRepository := notification.NewRepository(postgres.GORM, realtimeRepository)
	notificationService := notification.NewService(notificationRepository, settingService)
	schedulerRepository := scheduler.NewRepository(postgres.GORM)
	notificationDefinition := scheduler.NotificationBatchDefinition(nil)
	batchJobWriter, err := scheduler.NewBatchJobWriter(schedulerRepository, notificationDefinition)
	if err != nil {
		return err
	}
	notificationTaskRepository := notificationtask.NewRepositoryWithPermissions(postgres.GORM, batchJobWriter, func(db *gorm.DB) permissionnotification.Reader {
		return permissionnotification.NewRepository(db)
	})
	notificationTaskService := notificationtask.NewService(notificationTaskRepository)
	schedulerCatalog, err := scheduler.NewTaskCatalog(append(scheduler.BuiltinDefinitions(nil, nil, nil), notificationDefinition)...)
	if err != nil {
		return err
	}
	schedulerService := scheduler.NewService(schedulerRepository, schedulerCatalog)
	if err := validateRuntimeDependencies(
		runtimeDependency{name: "system.setting/global", validate: settingService.ValidateDependencies},
		runtimeDependency{name: "system.dictionary/global", validate: dictionaryService.ValidateDependencies},
		runtimeDependency{name: "message.mail/global runtime", validate: mailRuntimeStore.ValidateDependencies},
		runtimeDependency{name: "message.mail/global rate-limit", validate: mailRateLimitStore.ValidateDependencies},
		runtimeDependency{name: "message.sms/global runtime", validate: smsRuntimeCache.ValidateDependencies},
		runtimeDependency{name: "message.sms/global rate-limit", validate: smsRateLimitStore.ValidateDependencies},
		runtimeDependency{name: "permission.authPlatform mail/sms generation", validate: authPlatformService.ValidateRateLimitCacheDependencies},
		runtimeDependency{name: "storage.cosconfig/<positive config id>", validate: cosConfigService.ValidateDependencies},
		runtimeDependency{name: "storage.object-route/v2", validate: uploadRuleService.ValidateDependencies},
		runtimeDependency{name: "realtime.ticket Redis", validate: func() error {
			if realtimeTickets == nil {
				return errors.New("ticket store is unavailable")
			}
			return nil
		}},
		runtimeDependency{name: "realtime repository", validate: func() error {
			if realtimeRepository == nil {
				return errors.New("repository is unavailable")
			}
			return nil
		}},
		runtimeDependency{name: "message.notification retention settings", validate: func() error {
			if settingService == nil {
				return errors.New("setting reader is unavailable")
			}
			return nil
		}},
	); err != nil {
		return err
	}
	operationLogEnqueuer := operationlog.NewQueueEnqueuer(queueClient)
	authenticate := auth.Authenticate(authService)
	router := buildRouter(routerDependencies{
		CORSOrigin:     settings.CORSOrigin,
		TrustedProxies: settings.TrustedProxies,
		Logger:         logger,
		Health:         health.NewHandler(healthService),
		Auth:           auth.NewHandler(authService, settings.Auth.CookieSecure),
		Captcha:        authcaptcha.NewHandler(captchaService),
		AuthPlatform:   authplatform.NewHandler(authPlatformService),
		Permission:     permission.NewHandler(permissionService),
		Menu:           menu.NewHandler(menuService),
		Role:           role.NewHandler(roleService),
		User: account.NewHandler(userService, func(context *gin.Context) (int64, bool) {
			identity, ok := auth.IdentityFromContext(context)
			return identity.UserID, ok
		}),
		Account: profile.NewHandler(profileService, authService, func(context *gin.Context) (int64, bool) {
			identity, ok := auth.IdentityFromContext(context)
			return identity.UserID, ok
		}),
		Phone: userphone.NewHandler(phoneService, func(context *gin.Context) (userphone.Actor, bool) {
			identity, ok := auth.IdentityFromContext(context)
			client, clientOK := authclient.FromContext(context)
			return userphone.Actor{UserID: identity.UserID, SessionID: identity.SessionID, PlatformID: identity.PlatformID, Platform: identity.Platform, ClientIP: client.ClientIP}, ok && clientOK
		}),
		Email: useremail.NewHandler(emailService, func(context *gin.Context) (useremail.Actor, bool) {
			identity, ok := auth.IdentityFromContext(context)
			client, clientOK := authclient.FromContext(context)
			return useremail.Actor{UserID: identity.UserID, SessionID: identity.SessionID, PlatformID: identity.PlatformID, Platform: identity.Platform, ClientIP: client.ClientIP}, ok && clientOK
		}),
		COSConfig:         cosconfig.NewHandler(cosConfigService),
		UploadRule:        uploadrule.NewHandler(uploadRuleService),
		OperationLog:      operationlog.NewHandler(operationLogService),
		Dictionary:        dictionary.NewHandler(dictionaryService),
		Setting:           systemsetting.NewHandler(settingService),
		CacheGeneration:   systemcachegeneration.NewHandler(cacheGenerationService),
		QueueMonitor:      queuemonitor.NewHandler(queueMonitorService, settings.Auth.CookieSecure, queuemonitor.SubjectFromContext),
		QueueMonitorUI:    queuemonitor.NewGateway(queueMonitorService, queueMonitorUI),
		LoginLog:          loginlog.NewHandler(loginLogService),
		Mail:              messagemail.NewHandler(mailService),
		MailConfig:        mailconfig.NewHandler(mailConfigService),
		MailTemplate:      mailtemplate.NewHandler(mailTemplateService),
		MailLog:           maillog.NewHandler(mailLogService),
		MailRateLimit:     ratelimitpolicy.NewHandler(mailRateLimitService),
		MailRecipientRule: recipientrule.NewHandler(mailRecipientRuleService),
		SMS:               messagesms.NewHandler(smsService),
		SMSConfig:         smsconfig.NewHandler(smsConfigService),
		SMSTemplate:       smstemplate.NewHandler(smsTemplateService),
		SMSLog:            smslog.NewHandler(smsLogService),
		SMSRateLimit:      smsratelimitpolicy.NewHandler(smsRateLimitService),
		SMSRecipientRule:  smsrecipientrule.NewHandler(smsRecipientRuleService),
		Realtime:          realtimeHandler,
		Notification:      notification.NewHandler(notificationService),
		NotificationTask:  notificationtask.NewHandler(notificationTaskService),
		Scheduler:         scheduler.NewHandler(schedulerService),
		OperationEnqueuer: operationLogEnqueuer,
		SessionAdmin: usersession.NewSessionAdminHandler(sessionService, func(context *gin.Context) (usersession.Actor, bool) {
			identity, ok := auth.IdentityFromContext(context)
			return usersession.Actor{UserID: identity.UserID, SessionID: identity.SessionID}, ok
		}),
		AuthOrigin:   auth.RequireOrigin(settings.CORSOrigin),
		Authenticate: authenticate,
		RequirePermission: func(code string) gin.HandlerFunc {
			return permission.RequirePermission(permissionService, code)
		},
	})

	server := &http.Server{Addr: settings.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	return runAPIRuntime(processContext, realtimeSubscriber, realtimeConnections, server)
}

func runAPIRuntime(processContext context.Context, subscriber realtimeSubscriber, connections realtimeConnections, server httpRuntime) error {
	if processContext == nil || subscriber == nil || connections == nil || server == nil {
		return errors.New("API runtime dependencies are required")
	}
	subscriberContext, cancelSubscriber := context.WithCancel(processContext)
	subscriberDone := make(chan error, 1)
	go func() { subscriberDone <- subscriber.Run(subscriberContext) }()
	select {
	case <-subscriber.Ready():
	case err := <-subscriberDone:
		cancelSubscriber()
		if err == nil {
			err = errors.New("subscriber stopped before ready")
		}
		return fmt.Errorf("start realtime subscriber: %w", err)
	case <-processContext.Done():
		cancelSubscriber()
		<-subscriberDone
		return nil
	}

	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ListenAndServe() }()
	var serveErr error
	subscriberStopped := false
	select {
	case <-processContext.Done():
	case err := <-subscriberDone:
		subscriberStopped = true
		if err == nil {
			err = errors.New("subscriber stopped unexpectedly")
		}
		serveErr = fmt.Errorf("realtime subscriber stopped: %w", err)
	case err := <-serveDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}

	connections.BeginDrain()
	cancelSubscriber()
	if !subscriberStopped {
		select {
		case <-subscriberDone:
		case <-time.After(5 * time.Second):
		}
	}
	connections.CloseAll()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil && serveErr == nil {
		serveErr = fmt.Errorf("shutdown HTTP: %w", err)
	}
	return serveErr
}

func buildRouter(dependencies routerDependencies) *gin.Engine {
	router := gin.New()
	if err := router.SetTrustedProxies(dependencies.TrustedProxies); err != nil {
		panic(fmt.Sprintf("set trusted proxies: %v", err))
	}
	router.Use(
		projectmiddleware.RequestID(),
		projectmiddleware.CORS(dependencies.CORSOrigin),
		projectmiddleware.AccessLog(dependencies.Logger),
		operationlog.Middleware(dependencies.Logger, dependencies.OperationEnqueuer),
		projectmiddleware.Recovery(dependencies.Logger),
		projectmiddleware.Language(),
	)
	health.RegisterRoutes(router, dependencies.Health)
	if dependencies.QueueMonitorUI != nil {
		queuemonitor.RegisterUIRoutes(router, dependencies.QueueMonitorUI)
	}
	sharedRoutes := router.Group("/api/v1")
	sharedRoutes.Use(authclient.Require())
	auth.RegisterRoutes(sharedRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	if dependencies.Captcha != nil {
		authcaptcha.RegisterRoutes(sharedRoutes, dependencies.Captcha, dependencies.AuthOrigin)
	}
	authplatform.RegisterPublicRoutes(sharedRoutes, dependencies.AuthPlatform)
	permission.RegisterRoutes(sharedRoutes, dependencies.Permission, dependencies.Authenticate)
	if dependencies.Realtime != nil {
		realtime.RegisterRoutes(sharedRoutes, router, dependencies.Realtime, dependencies.AuthOrigin, dependencies.Authenticate)
	}
	if dependencies.Notification != nil {
		notification.RegisterRoutes(sharedRoutes, dependencies.Notification, dependencies.Authenticate, dependencies.RequirePermission)
	}
	dictionary.RegisterOptionRoute(sharedRoutes, dependencies.Dictionary, dependencies.Authenticate)

	adminRoutes := router.Group("/api/admin/v1")
	adminRoutes.Use(authclient.Require(), authclient.RequireAdminPlatform())
	authplatform.RegisterManagementRoutes(adminRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
	menu.RegisterRoutes(adminRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
	role.RegisterRoutes(adminRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
	account.RegisterRoutes(adminRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
	profile.RegisterRoutes(adminRoutes, dependencies.Account, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Phone != nil {
		userphone.RegisterRoutes(adminRoutes, dependencies.Phone, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Email != nil {
		useremail.RegisterRoutes(adminRoutes, dependencies.Email, dependencies.Authenticate, dependencies.RequirePermission)
	}
	cosconfig.RegisterRoutes(adminRoutes, dependencies.COSConfig, dependencies.Authenticate, dependencies.RequirePermission)
	uploadrule.RegisterRoutes(adminRoutes, dependencies.UploadRule, dependencies.Authenticate, dependencies.RequirePermission)
	uploadrule.RegisterCredentialRoute(sharedRoutes, dependencies.UploadRule, dependencies.Authenticate, dependencies.RequirePermission)
	loginlog.RegisterRoutes(adminRoutes, dependencies.LoginLog, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Mail != nil {
		mailRoutes := adminRoutes.Group("/message/mail")
		messagemail.RegisterRoutes(mailRoutes, dependencies.Mail, dependencies.Authenticate, dependencies.RequirePermission)
		mailconfig.RegisterRoutes(mailRoutes, dependencies.MailConfig, dependencies.Authenticate, dependencies.RequirePermission)
		mailtemplate.RegisterRoutes(mailRoutes, dependencies.MailTemplate, dependencies.Authenticate, dependencies.RequirePermission)
		maillog.RegisterRoutes(mailRoutes, dependencies.MailLog, dependencies.Authenticate, dependencies.RequirePermission)
		ratelimitpolicy.RegisterRoutes(mailRoutes, dependencies.MailRateLimit, dependencies.Authenticate, dependencies.RequirePermission)
		recipientrule.RegisterRoutes(mailRoutes, dependencies.MailRecipientRule, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.SMS != nil {
		smsRoutes := adminRoutes.Group("/message/sms")
		messagesms.RegisterRoutes(smsRoutes, dependencies.SMS, dependencies.Authenticate, dependencies.RequirePermission)
		smsconfig.RegisterRoutes(smsRoutes, dependencies.SMSConfig, dependencies.Authenticate, dependencies.RequirePermission)
		smstemplate.RegisterRoutes(smsRoutes, dependencies.SMSTemplate, dependencies.Authenticate, dependencies.RequirePermission)
		smslog.RegisterRoutes(smsRoutes, dependencies.SMSLog, dependencies.Authenticate, dependencies.RequirePermission)
		smsratelimitpolicy.RegisterRoutes(smsRoutes, dependencies.SMSRateLimit, dependencies.Authenticate, dependencies.RequirePermission)
		smsrecipientrule.RegisterRoutes(smsRoutes, dependencies.SMSRecipientRule, dependencies.Authenticate, dependencies.RequirePermission)
	}
	operationlog.RegisterRoutes(adminRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)
	dictionary.RegisterRoutes(adminRoutes, dependencies.Dictionary, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Setting != nil {
		systemsetting.RegisterRoutes(adminRoutes, dependencies.Setting, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.CacheGeneration != nil {
		systemcachegeneration.RegisterRoutes(adminRoutes, dependencies.CacheGeneration, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.QueueMonitor != nil {
		queuemonitor.RegisterGrantRoute(adminRoutes, dependencies.QueueMonitor, dependencies.AuthOrigin, dependencies.Authenticate, dependencies.RequirePermission)
	}
	usersession.RegisterSessionAdminRoutes(adminRoutes, dependencies.SessionAdmin, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.NotificationTask != nil {
		notificationtask.RegisterRoutes(adminRoutes, dependencies.NotificationTask, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Scheduler != nil {
		scheduler.RegisterRoutes(adminRoutes, dependencies.Scheduler, dependencies.Authenticate, dependencies.RequirePermission)
	}
	return router
}
