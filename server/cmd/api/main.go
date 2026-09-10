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
	"admin/server/internal/module/permission/access"
	"admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/module/permission/menu"
	"admin/server/internal/module/permission/role"
	"admin/server/internal/module/permission/state"
	"admin/server/internal/module/storage/cosConfig"
	"admin/server/internal/module/storage/uploadRule"
	"admin/server/internal/module/system/dictionary"
	"admin/server/internal/module/system/operationLog"
	account "admin/server/internal/module/user/account"
	"admin/server/internal/module/user/loginLog"
	profile "admin/server/internal/module/user/profile"
	usersession "admin/server/internal/module/user/session"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/i18n"
	storagecos "admin/server/internal/storage/cos"
	storagemail "admin/server/internal/storage/mail"

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
	AuthPlatform      *authplatform.Handler
	Permission        *permission.Handler
	Menu              *menu.Handler
	Role              *role.Handler
	User              *account.Handler
	Account           *profile.Handler
	COSConfig         *cosconfig.Handler
	UploadRule        *uploadrule.Handler
	OperationLog      *operationlog.Handler
	Dictionary        *dictionary.Handler
	LoginLog          *loginlog.Handler
	Mail              *messagemail.Handler
	MailConfig        *mailconfig.Handler
	MailTemplate      *mailtemplate.Handler
	MailLog           *maillog.Handler
	MailRateLimit     *ratelimitpolicy.Handler
	MailRecipientRule *recipientrule.Handler
	OperationEnqueuer operationlog.Enqueuer
	SessionAdmin      *usersession.SessionAdminHandler
	AuthOrigin        gin.HandlerFunc
	Authenticate      gin.HandlerFunc
	RequirePermission func(string) gin.HandlerFunc
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
	accessStateStore := permissionstate.NewStore(redisClient)
	accessInvalidator := permissionstate.NewInvalidator(accessStateStore)
	menuRepository := menu.NewRepository(postgres.GORM)
	menuStateStore := permissionstate.NewMenuStore(redisClient)
	menuService := menu.NewService(menuRepository, menuStateStore)

	roleRepository := role.NewRepository(postgres.GORM)
	roleService := role.NewService(roleRepository, accessInvalidator)
	keys, err := secretkey.New(settings.AppSecret)
	if err != nil {
		return fmt.Errorf("derive application keys: %w", err)
	}

	healthService := health.NewService(postgres, redisClient)
	userRepository := account.NewRepository(postgres.GORM)
	profileRepository := profile.NewRepository(postgres.GORM)
	sessionRepository := usersession.NewRepository(postgres.GORM)
	authPlatformRepository := authplatform.NewRepository(postgres.GORM)
	mailRateLimitRepository := ratelimitpolicy.NewRepository(postgres.GORM)
	authPlatformRepository.SetRateLimitPolicyLifecycle(
		func(ctx context.Context, tx *gorm.DB, platformID int64) error {
			return ratelimitpolicy.NewRepository(tx).ProvisionDefaults(ctx, platformID)
		},
		func(ctx context.Context, tx *gorm.DB, platformID int64) error {
			return ratelimitpolicy.NewRepository(tx).DeleteForPlatform(ctx, platformID)
		},
	)
	policyStore := authplatform.NewPolicyStore(redisClient)
	authStateStore := authstate.NewStore(redisClient)
	authInvalidator := authstate.NewInvalidator(authStateStore)
	authPlatformService := authplatform.NewService(authPlatformRepository, policyStore, redisClient, authStateStore, authInvalidator, auth.NewSessionCache(redisClient).Delete, logger, authplatform.Deployment{
		CookieSecure: settings.Auth.CookieSecure, CORSOrigin: settings.CORSOrigin,
		TrustedProxyMode: settings.TrustedProxyMode, TrustedProxyCount: len(settings.TrustedProxies),
	})
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
	authService.SetPasswordStore(userRepository)
	sessionService := usersession.NewService(sessionRepository, authStateStore, authInvalidator, auth.NewSessionCache(redisClient))
	userService := account.NewService(userRepository, authStateStore, authInvalidator, accessStateStore, accessInvalidator)
	profileService := profile.NewService(profileRepository)
	cosClient := storagecos.NewClient(nil)
	cosConfigService := cosconfig.NewService(cosconfig.NewRepository(postgres.GORM), keys, cosClient)
	uploadRuleService := uploadrule.NewService(uploadrule.NewRepository(postgres.GORM), keys, cosClient)
	loginLogService := loginlog.NewService(loginlog.NewRepository(postgres.GORM))
	mailStores := messagemail.NewStores(postgres.GORM)
	mailLimiter := messagemail.NewRedisLimiter(redisClient.UniversalClient())
	mailRateLimitStore := ratelimitpolicy.NewStore(mailRateLimitRepository, redisClient)
	mailRateLimitService := ratelimitpolicy.NewService(mailRateLimitRepository, mailRateLimitStore)
	mailReadinessStore := messagemail.NewVerifyCodeReadinessStore(mailStores, redisClient)
	mailReadinessCoordinator := messagemail.NewReadinessCoordinator(mailReadinessStore)
	mailRuntimeStore := messagemail.NewRuntimeStore(mailStores, redisClient)
	mailRecipientRuleRepository := mailStores.RecipientRule
	mailRecipientRuleService := recipientrule.NewService(mailRecipientRuleRepository)
	mailService := messagemail.NewService(mailStores, keys, storagemail.NewTencentSESClient(nil), mailRecipientRuleService, mailLimiter, mailRateLimitStore)
	mailService.SetVerifyCodeReadinessStore(mailReadinessStore)
	mailService.SetRuntimeStore(mailRuntimeStore)
	mailConfigService := mailconfig.NewService(mailStores.Config, keys, mailReadinessCoordinator)
	mailConfigService.SetRuntimeCoordinator(mailRuntimeStore)
	mailTemplateService := mailtemplate.NewService(mailStores.Template, mailReadinessCoordinator)
	mailTemplateService.SetRuntimeCoordinator(mailRuntimeStore)
	mailLogService := maillog.NewService(mailStores.Log, mailStores.LogVerification, keys)
	mailRecipientRuleService.SetRuntimeCoordinator(mailRuntimeStore)
	verificationStore := auth.NewVerificationCodeStore(redisClient, keys.VerificationCodeHMACKey())
	authService.SetVerifyCodeSender(mailService)
	authService.SetVerificationCodeStore(verificationStore)
	authService.SetLoginLogRecorder(loginLogService)
	permissionRepository := permission.NewRepository(postgres.GORM)
	permissionService := permission.NewService(permissionRepository, accessStateStore, permission.NewSnapshotCache(redisClient), permission.NewLocalSnapshotCache(1024), logger, menuStateStore)
	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
	dictionaryService := dictionary.NewService(dictionary.NewRepository(postgres.GORM))
	dictionaryService.SetCache(dictionary.NewOptionsCache(redisClient))
	operationLogEnqueuer := operationlog.NewQueueEnqueuer(queueClient)
	authenticate := auth.Authenticate(authService)
	router := buildRouter(routerDependencies{
		CORSOrigin:     settings.CORSOrigin,
		TrustedProxies: settings.TrustedProxies,
		Logger:         logger,
		Health:         health.NewHandler(healthService),
		Auth:           auth.NewHandler(authService, settings.Auth.CookieSecure),
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
		COSConfig:         cosconfig.NewHandler(cosConfigService),
		UploadRule:        uploadrule.NewHandler(uploadRuleService),
		OperationLog:      operationlog.NewHandler(operationLogService),
		Dictionary:        dictionary.NewHandler(dictionaryService),
		LoginLog:          loginlog.NewHandler(loginLogService),
		Mail:              messagemail.NewHandler(mailService),
		MailConfig:        mailconfig.NewHandler(mailConfigService),
		MailTemplate:      mailtemplate.NewHandler(mailTemplateService),
		MailLog:           maillog.NewHandler(mailLogService),
		MailRateLimit:     ratelimitpolicy.NewHandler(mailRateLimitService),
		MailRecipientRule: recipientrule.NewHandler(mailRecipientRuleService),
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
	serveErrors := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
		}
		close(serveErrors)
	}()

	select {
	case <-processContext.Done():
	case err := <-serveErrors:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP: %w", err)
	}
	return nil
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
	sharedRoutes := router.Group("/api/v1")
	sharedRoutes.Use(authclient.Require())
	auth.RegisterRoutes(sharedRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	authplatform.RegisterPublicRoutes(sharedRoutes, dependencies.AuthPlatform)
	permission.RegisterRoutes(sharedRoutes, dependencies.Permission, dependencies.Authenticate)

	adminRoutes := router.Group("/api/admin/v1")
	adminRoutes.Use(authclient.Require(), authclient.RequireAdminPlatform())
	authplatform.RegisterManagementRoutes(adminRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
	menu.RegisterRoutes(adminRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
	role.RegisterRoutes(adminRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
	account.RegisterRoutes(adminRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
	profile.RegisterRoutes(adminRoutes, dependencies.Account, dependencies.Authenticate, dependencies.RequirePermission)
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
	operationlog.RegisterRoutes(adminRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)
	dictionary.RegisterRoutes(adminRoutes, dependencies.Dictionary, dependencies.Authenticate, dependencies.RequirePermission)
	usersession.RegisterSessionAdminRoutes(adminRoutes, dependencies.SessionAdmin, dependencies.Authenticate, dependencies.RequirePermission)
	return router
}
