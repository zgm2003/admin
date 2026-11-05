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
	callbackevent "admin/server/internal/module/payment/callbackEvent"
	paymentconfig "admin/server/internal/module/payment/config"
	paymentorder "admin/server/internal/module/payment/order"
	rechargepackage "admin/server/internal/module/payment/rechargePackage"
	redeemcode "admin/server/internal/module/payment/redeemCode"
	paymenttransaction "admin/server/internal/module/payment/transaction"
	paymentwallet "admin/server/internal/module/payment/wallet"
	"admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/module/permission/menu"
	permissionnotification "admin/server/internal/module/permission/notification"
	"admin/server/internal/module/permission/role"
	"admin/server/internal/module/permission/state"
	"admin/server/internal/module/realtime"
	"admin/server/internal/module/storage/cosConfig"
	storagefile "admin/server/internal/module/storage/file"
	"admin/server/internal/module/storage/uploadRule"
	systemconfigcachestatus "admin/server/internal/module/system/configCacheStatus"
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
	"admin/server/internal/payment/alipay"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/configCache"
	"admin/server/internal/shared/i18n"
	storagecos "admin/server/internal/storage/cos"
	storagemail "admin/server/internal/storage/mail"
	storagesms "admin/server/internal/storage/sms"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type routerDependencies struct {
	PaymentConfig      *paymentconfig.Handler
	PaymentPackage     *rechargepackage.Handler
	PaymentOrder       *paymentorder.Handler
	PaymentWallet      *paymentwallet.Handler
	PaymentTransaction *paymenttransaction.Handler
	PaymentCallback    *callbackevent.Handler
	PaymentRedeemCode  *redeemcode.Handler
	CORSOrigin         string
	TrustedProxies     []string
	Logger             *slog.Logger
	Health             *health.Handler
	Auth               *auth.Handler
	Captcha            *authcaptcha.Handler
	AuthPlatform       *authplatform.Handler
	Permission         *permission.Handler
	Menu               *menu.Handler
	Role               *role.Handler
	User               *account.Handler
	Account            *profile.Handler
	Phone              *userphone.Handler
	Email              *useremail.Handler
	COSConfig          *cosconfig.Handler
	UploadRule         *uploadrule.Handler
	File               *storagefile.Handler
	OperationLog       *operationlog.Handler
	Setting            *systemsetting.Handler
	ConfigCacheStatus  *systemconfigcachestatus.Handler
	QueueMonitor       *queuemonitor.Handler
	QueueMonitorUI     http.Handler
	LoginLog           *loginlog.Handler
	Mail               *messagemail.Handler
	MailConfig         *mailconfig.Handler
	MailTemplate       *mailtemplate.Handler
	MailLog            *maillog.Handler
	MailRateLimit      *ratelimitpolicy.Handler
	MailRecipientRule  *recipientrule.Handler
	SMS                *messagesms.Handler
	SMSConfig          *smsconfig.Handler
	SMSTemplate        *smstemplate.Handler
	SMSLog             *smslog.Handler
	SMSRateLimit       *smsratelimitpolicy.Handler
	SMSRecipientRule   *smsrecipientrule.Handler
	Realtime           *realtime.Handler
	Notification       *notification.Handler
	NotificationTask   *notificationtask.Handler
	Scheduler          *scheduler.Handler
	OperationEnqueuer  operationlog.Enqueuer
	SessionAdmin       *usersession.SessionAdminHandler
	AuthOrigin         gin.HandlerFunc
	Authenticate       gin.HandlerFunc
	RequirePermission  func(string) gin.HandlerFunc
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
	configGenerationRepository := configcache.NewRepository(postgres.GORM)
	configGenerationStore := configcache.NewStore(redisClient)
	paymentConfigRepository := paymentconfig.NewRepository(postgres.GORM)
	paymentConfigRepository.SetGenerations(configGenerationRepository)
	paymentConfigService := paymentconfig.NewService(paymentConfigRepository, keys)
	paymentConfigService.SetGenerations(configGenerationRepository, configGenerationStore)
	paymentConfigService.SetCache(paymentconfig.NewCache(redisClient, configGenerationStore))
	paymentPackageRepository := rechargepackage.NewRepository(postgres.GORM)
	paymentPackageService := rechargepackage.NewService(paymentPackageRepository)
	paymentWalletRepository := paymentwallet.NewRepository(postgres.GORM)
	paymentWalletService := paymentwallet.NewService(paymentWalletRepository)
	paymentTransactionRepository := paymenttransaction.NewRepository(postgres.GORM)
	paymentTransactionService := paymenttransaction.NewService(paymentTransactionRepository)
	paymentOrderRepository := paymentorder.NewRepository(postgres.GORM)
	paymentOrderService := paymentorder.NewService(paymentOrderRepository)
	paymentOrderService.SetRuntime(paymentConfigService, paymentPackageService)
	paymentOrderService.SetCheckout(func(ctx context.Context, credentials alipay.Credentials, input alipay.CheckoutInput) (string, error) {
		return alipay.Checkout(ctx, credentials, input)
	})
	paymentCallbackRepository := callbackevent.NewRepository(postgres.GORM)
	paymentCallbackService := callbackevent.NewService(paymentCallbackRepository)
	paymentCallbackService.SetRuntime(paymentConfigService)
	paymentCallbackService.SetConfirmer(paymentOrderService)
	paymentRedeemRepository := redeemcode.NewRepository(postgres.GORM)
	paymentRedeemService := redeemcode.NewService(paymentRedeemRepository, redeemcode.NewRedisLimiter(redisClient.UniversalClient()))
	settingRepository := systemsetting.NewRepository(postgres.GORM)
	settingRepository.SetGenerations(configGenerationRepository)
	settingService := systemsetting.NewService(settingRepository)
	settingCache := systemsetting.NewCache(redisClient)
	settingCache.SetStateStore(configGenerationStore)
	settingService.SetCache(settingCache)
	configCacheService := systemconfigcachestatus.NewService(systemconfigcachestatus.NewRepository(postgres.GORM), configGenerationStore)
	settingService.SetGenerations(configGenerationRepository, configGenerationStore)
	settingService.SetLogger(logger)
	paymentOrderService.SetSettings(settingService)
	profileRepository := profile.NewRepository(postgres.GORM)
	sessionRepository := usersession.NewRepository(postgres.GORM)
	authPlatformRepository := authplatform.NewRepository(postgres.GORM)
	mailRateLimitRepository := ratelimitpolicy.NewRepository(postgres.GORM)
	mailGenerationScope := messagemail.ConfigCacheScope()
	smsGenerationScope := messagesms.ConfigCacheScope()
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
	authPlatformService.SetRateLimitConfigCaches(configGenerationRepository, configGenerationStore, mailGenerationScope, smsGenerationScope)
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
	mailRateLimitRepository.SetGenerations(configGenerationRepository, messagemail.ConfigCacheScope())
	mailRateLimitStore := ratelimitpolicy.NewStore(mailRateLimitRepository, redisClient)
	mailRateLimitStore.SetGenerations(configGenerationRepository, configGenerationStore)
	mailRateLimitService := ratelimitpolicy.NewService(mailRateLimitRepository)
	mailRuntimeStore := messagemail.NewRuntimeStore(mailStores, redisClient)
	mailRuntimeStore.SetGenerations(configGenerationRepository, configGenerationStore)
	mailRuntimeStore.SetLogger(logger)
	mailRateLimitService.SetRuntimeCoordinator(mailRuntimeStore)
	mailStores.Config.SetGenerations(configGenerationRepository, messagemail.ConfigCacheScope())
	mailStores.Template.SetGenerations(configGenerationRepository, messagemail.ConfigCacheScope())
	mailStores.RecipientRule.SetGenerations(configGenerationRepository, messagemail.ConfigCacheScope())
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
	mailRecipientRuleService.SetSettings(settingService)
	smsRuntimeCache := messagesms.NewRuntimeCache(redisClient)
	smsRuntimeCache.SetGenerations(configGenerationRepository, configGenerationStore)
	smsRuntimeCache.SetLogger(logger)
	smsConfigRepository := smsconfig.NewRepository(postgres.GORM)
	smsConfigRepository.SetGenerations(configGenerationRepository, messagesms.ConfigCacheScope())
	smsConfigService := smsconfig.NewService(smsConfigRepository, keys, smsRuntimeCache)
	smsTemplateRepository := smstemplate.NewRepository(postgres.GORM)
	smsTemplateRepository.SetGenerations(configGenerationRepository, messagesms.ConfigCacheScope())
	smsTemplateService := smstemplate.NewService(smsTemplateRepository)
	smsTemplateService.SetRuntimeCoordinator(smsRuntimeCache)
	smsRecipientRuleRepository := smsrecipientrule.NewRepository(postgres.GORM)
	smsRecipientRuleRepository.SetGenerations(configGenerationRepository, messagesms.ConfigCacheScope())
	smsRecipientRuleService := smsrecipientrule.NewService(smsRecipientRuleRepository)
	smsRecipientRuleService.SetRuntimeCoordinator(smsRuntimeCache)
	smsRateLimitRepository := smsratelimitpolicy.NewRepository(postgres.GORM)
	smsRateLimitRepository.SetGenerations(configGenerationRepository, messagesms.ConfigCacheScope())
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
	phoneService := userphone.NewService(userphone.NewRepository(postgres.GORM), smsService, verificationStore, userphone.NewAuthorityCoordinator(authStateStore, authInvalidator))
	emailService := useremail.NewService(useremail.NewRepository(postgres.GORM), mailService, verificationStore, useremail.NewAuthorityCoordinator(authStateStore, authInvalidator))
	permissionRepository := permission.NewRepository(postgres.GORM)
	permissionService := permission.NewService(permissionRepository, accessStateStore, permission.NewSnapshotCache(redisClient), permission.NewLocalSnapshotCache(1024), logger, menuStateStore)
	profileService.SetAvatarPermissions(permissionService)
	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
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
	fileService := storagefile.NewService(storagefile.NewRepository(postgres.GORM, schedulerRepository), keys, cosClient, cosConfigService, uploadRuleService)
	uploadRuleService.SetFileRegistrar(fileService)
	profileService.SetAvatarCopier(fileService)
	notificationDefinition := scheduler.NotificationBatchDefinition(nil)
	batchJobWriter, err := scheduler.NewBatchJobWriter(schedulerRepository, notificationDefinition)
	if err != nil {
		return err
	}
	notificationTaskRepository := notificationtask.NewRepositoryWithPermissions(postgres.GORM, batchJobWriter, func(db *gorm.DB) permissionnotification.Reader {
		return permissionnotification.NewRepository(db)
	})
	notificationTaskService := notificationtask.NewService(notificationTaskRepository)
	schedulerCatalog, err := scheduler.NewTaskCatalog(append(scheduler.BuiltinDefinitions(nil, nil, nil), notificationDefinition, scheduler.PaymentReconcileDefinition(nil), storagefile.CleanupDefinition(fileService), storagefile.ScanDefinition(fileService))...)
	if err != nil {
		return err
	}
	schedulerService := scheduler.NewService(schedulerRepository, schedulerCatalog)
	if err := validateRuntimeDependencies(
		runtimeDependency{name: "system.setting/global", validate: settingService.ValidateDependencies},
		runtimeDependency{name: "message.mail/global runtime", validate: mailRuntimeStore.ValidateDependencies},
		runtimeDependency{name: "message.mail/global rate-limit", validate: mailRateLimitStore.ValidateDependencies},
		runtimeDependency{name: "message.sms/global runtime", validate: smsRuntimeCache.ValidateDependencies},
		runtimeDependency{name: "message.sms/global rate-limit", validate: smsRateLimitStore.ValidateDependencies},
		runtimeDependency{name: "permission.authPlatform mail/sms generation", validate: authPlatformService.ValidateRateLimitCacheDependencies},
		runtimeDependency{name: "storage.cosconfig/<positive config id>", validate: cosConfigService.ValidateDependencies},
		runtimeDependency{name: "payment.config/global", validate: paymentConfigService.ValidateDependencies},
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
		COSConfig:          cosconfig.NewHandler(cosConfigService),
		PaymentConfig:      paymentconfig.NewHandler(paymentConfigService),
		PaymentPackage:     rechargepackage.NewHandler(paymentPackageService),
		PaymentOrder:       paymentorder.NewHandler(paymentOrderService),
		PaymentWallet:      paymentwallet.NewHandler(paymentWalletService),
		PaymentTransaction: paymenttransaction.NewHandler(paymentTransactionService),
		PaymentCallback:    callbackevent.NewHandler(paymentCallbackService),
		PaymentRedeemCode:  redeemcode.NewHandler(paymentRedeemService),
		UploadRule:         uploadrule.NewHandler(uploadRuleService),
		File:               storagefile.NewHandler(fileService),
		OperationLog:       operationlog.NewHandler(operationLogService),
		Setting:            systemsetting.NewHandler(settingService),
		ConfigCacheStatus:  systemconfigcachestatus.NewHandler(configCacheService),
		QueueMonitor:       queuemonitor.NewHandler(queueMonitorService, settings.Auth.CookieSecure, queuemonitor.SubjectFromContext),
		QueueMonitorUI:     queuemonitor.NewGateway(queueMonitorService, queueMonitorUI),
		LoginLog:           loginlog.NewHandler(loginLogService),
		Mail:               messagemail.NewHandler(mailService),
		MailConfig:         mailconfig.NewHandler(mailConfigService),
		MailTemplate:       mailtemplate.NewHandler(mailTemplateService),
		MailLog:            maillog.NewHandler(mailLogService),
		MailRateLimit:      ratelimitpolicy.NewHandler(mailRateLimitService),
		MailRecipientRule:  recipientrule.NewHandler(mailRecipientRuleService),
		SMS:                messagesms.NewHandler(smsService),
		SMSConfig:          smsconfig.NewHandler(smsConfigService),
		SMSTemplate:        smstemplate.NewHandler(smsTemplateService),
		SMSLog:             smslog.NewHandler(smsLogService),
		SMSRateLimit:       smsratelimitpolicy.NewHandler(smsRateLimitService),
		SMSRecipientRule:   smsrecipientrule.NewHandler(smsRecipientRuleService),
		Realtime:           realtimeHandler,
		Notification:       notification.NewHandler(notificationService),
		NotificationTask:   notificationtask.NewHandler(notificationTaskService),
		Scheduler:          scheduler.NewHandler(schedulerService),
		OperationEnqueuer:  operationLogEnqueuer,
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
	// Provider callbacks and browser WebSockets authenticate by signature/ticket,
	// not by client headers. They still belong to one explicit platform.
	adminTransports := router.Group("/api/admin/v1")
	if dependencies.PaymentCallback != nil {
		callbackevent.RegisterAdminProviderRoutes(adminTransports, dependencies.PaymentCallback)
	}
	adminRoutes := router.Group("/api/admin/v1", authclient.Require(), authclient.RequirePlatform("admin"))
	auth.RegisterAdminRoutes(adminRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	if dependencies.Captcha != nil {
		authcaptcha.RegisterAdminRoutes(adminRoutes, dependencies.Captcha, dependencies.AuthOrigin)
	}
	authplatform.RegisterAdminRoutes(adminRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
	permission.RegisterAdminRoutes(adminRoutes, dependencies.Permission, dependencies.Authenticate)
	menu.RegisterAdminRoutes(adminRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
	role.RegisterAdminRoutes(adminRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
	account.RegisterAdminRoutes(adminRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Account != nil {
		profile.RegisterAdminRoutes(adminRoutes, dependencies.Account, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Phone != nil {
		userphone.RegisterAdminRoutes(adminRoutes, dependencies.Phone, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Email != nil {
		useremail.RegisterAdminRoutes(adminRoutes, dependencies.Email, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Realtime != nil {
		realtime.RegisterAdminRoutes(adminRoutes, router, dependencies.Realtime, dependencies.AuthOrigin, dependencies.Authenticate)
	}
	if dependencies.Notification != nil {
		notification.RegisterAdminRoutes(adminRoutes, dependencies.Notification, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentConfig != nil {
		paymentconfig.RegisterAdminRoutes(adminRoutes, dependencies.PaymentConfig, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentPackage != nil {
		rechargepackage.RegisterAdminRoutes(adminRoutes, dependencies.PaymentPackage, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentOrder != nil {
		paymentorder.RegisterAdminRoutes(adminRoutes, dependencies.PaymentOrder, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentWallet != nil {
		paymentwallet.RegisterAdminRoutes(adminRoutes, dependencies.PaymentWallet, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentTransaction != nil {
		paymenttransaction.RegisterAdminRoutes(adminRoutes, dependencies.PaymentTransaction, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentCallback != nil {
		callbackevent.RegisterAdminRoutes(adminRoutes, dependencies.PaymentCallback, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.PaymentRedeemCode != nil {
		redeemcode.RegisterAdminRoutes(adminRoutes, dependencies.PaymentRedeemCode, dependencies.Authenticate, dependencies.RequirePermission)
	}
	cosconfig.RegisterAdminRoutes(adminRoutes, dependencies.COSConfig, dependencies.Authenticate, dependencies.RequirePermission)
	uploadrule.RegisterAdminRoutes(adminRoutes, dependencies.UploadRule, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.File != nil {
		storagefile.RegisterAdminRoutes(adminRoutes, dependencies.File, dependencies.Authenticate, dependencies.RequirePermission)
	}
	loginlog.RegisterAdminRoutes(adminRoutes, dependencies.LoginLog, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Mail != nil {
		mailRoutes := adminRoutes.Group("/message/mail")
		messagemail.RegisterAdminRoutes(mailRoutes, dependencies.Mail, dependencies.Authenticate, dependencies.RequirePermission)
		mailconfig.RegisterAdminRoutes(mailRoutes, dependencies.MailConfig, dependencies.Authenticate, dependencies.RequirePermission)
		mailtemplate.RegisterAdminRoutes(mailRoutes, dependencies.MailTemplate, dependencies.Authenticate, dependencies.RequirePermission)
		maillog.RegisterAdminRoutes(mailRoutes, dependencies.MailLog, dependencies.Authenticate, dependencies.RequirePermission)
		ratelimitpolicy.RegisterAdminRoutes(mailRoutes, dependencies.MailRateLimit, dependencies.Authenticate, dependencies.RequirePermission)
		recipientrule.RegisterAdminRoutes(mailRoutes, dependencies.MailRecipientRule, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.SMS != nil {
		smsRoutes := adminRoutes.Group("/message/sms")
		messagesms.RegisterAdminRoutes(smsRoutes, dependencies.SMS, dependencies.Authenticate, dependencies.RequirePermission)
		smsconfig.RegisterAdminRoutes(smsRoutes, dependencies.SMSConfig, dependencies.Authenticate, dependencies.RequirePermission)
		smstemplate.RegisterAdminRoutes(smsRoutes, dependencies.SMSTemplate, dependencies.Authenticate, dependencies.RequirePermission)
		smslog.RegisterAdminRoutes(smsRoutes, dependencies.SMSLog, dependencies.Authenticate, dependencies.RequirePermission)
		smsratelimitpolicy.RegisterAdminRoutes(smsRoutes, dependencies.SMSRateLimit, dependencies.Authenticate, dependencies.RequirePermission)
		smsrecipientrule.RegisterAdminRoutes(smsRoutes, dependencies.SMSRecipientRule, dependencies.Authenticate, dependencies.RequirePermission)
	}
	operationlog.RegisterAdminRoutes(adminRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.Setting != nil {
		systemsetting.RegisterAdminRoutes(adminRoutes, dependencies.Setting, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.ConfigCacheStatus != nil {
		systemconfigcachestatus.RegisterAdminRoutes(adminRoutes, dependencies.ConfigCacheStatus, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.QueueMonitor != nil {
		queuemonitor.RegisterAdminRoutes(adminRoutes, dependencies.QueueMonitor, dependencies.AuthOrigin, dependencies.Authenticate, dependencies.RequirePermission)
	}
	usersession.RegisterAdminRoutes(adminRoutes, dependencies.SessionAdmin, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.NotificationTask != nil {
		notificationtask.RegisterAdminRoutes(adminRoutes, dependencies.NotificationTask, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Scheduler != nil {
		scheduler.RegisterAdminRoutes(adminRoutes, dependencies.Scheduler, dependencies.Authenticate, dependencies.RequirePermission)
	}

	// Canvas is an independent allow-list, never a replay of the Admin list.
	canvasRoutes := router.Group("/api/canvas/v1", authclient.Require(), authclient.RequirePlatform("canvas"))
	auth.RegisterCanvasRoutes(canvasRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	if dependencies.Captcha != nil {
		authcaptcha.RegisterCanvasRoutes(canvasRoutes, dependencies.Captcha, dependencies.AuthOrigin)
	}
	authplatform.RegisterCanvasRoutes(canvasRoutes, dependencies.AuthPlatform)
	permission.RegisterCanvasRoutes(canvasRoutes, dependencies.Permission, dependencies.Authenticate)
	if dependencies.Account != nil {
		profile.RegisterCanvasRoutes(canvasRoutes, dependencies.Account, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Email != nil {
		useremail.RegisterCanvasRoutes(canvasRoutes, dependencies.Email, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Phone != nil {
		userphone.RegisterCanvasRoutes(canvasRoutes, dependencies.Phone, dependencies.Authenticate, dependencies.RequirePermission)
	}
	uploadrule.RegisterCanvasRoutes(canvasRoutes, dependencies.UploadRule, dependencies.Authenticate, dependencies.RequirePermission)
	if dependencies.File != nil {
		storagefile.RegisterCanvasRoutes(canvasRoutes, dependencies.File, dependencies.Authenticate, dependencies.RequirePermission)
	}
	if dependencies.Setting != nil {
		systemsetting.RegisterCanvasRoutes(canvasRoutes, dependencies.Setting)
	}
	return router
}
