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
	"admin/server/internal/module/access"
	"admin/server/internal/module/accessstate"
	"admin/server/internal/module/auth"
	"admin/server/internal/module/authclient"
	"admin/server/internal/module/authplatform"
	"admin/server/internal/module/authstate"
	"admin/server/internal/module/health"
	"admin/server/internal/module/menu"
	"admin/server/internal/module/operationlog"
	"admin/server/internal/module/role"
	"admin/server/internal/module/taskdemo"
	"admin/server/internal/module/user"
	"admin/server/internal/queue"
	projectredis "admin/server/internal/redis"
	"admin/server/internal/secretkey"
	"admin/server/internal/shared/i18n"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type routerDependencies struct {
	CORSOrigin        string
	TrustedProxies    []string
	Logger            *slog.Logger
	Health            *health.Handler
	Task              *taskdemo.Handler
	Auth              *auth.Handler
	AuthPlatform      *authplatform.Handler
	Access            *access.Handler
	Menu              *menu.Handler
	Role              *role.Handler
	User              *user.Handler
	OperationLog      *operationlog.Handler
	OperationEnqueuer operationlog.Enqueuer
	SessionAdmin      *auth.SessionAdminHandler
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
	if err := database.PrepareDomainNames(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("prepare domain database names: %w", err)
	}
	if err := auth.PrepareSessionSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("prepare authentication schema: %w", err)
	}
	if err := operationlog.PrepareSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("prepare operation log schema: %w", err)
	}
	if err := menu.PrepareSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("prepare menu schema: %w", err)
	}
	if err := database.AutoMigrate(
		processContext,
		postgres.GORM,
		&taskdemo.Task{},
		&user.User{},
		&role.Role{},
		&role.UserRole{},
		&menu.Menu{},
		&menu.RoleMenu{},
		&authplatform.Platform{},
		&auth.Session{},
		&operationlog.OperationLog{},
		&access.Version{},
	); err != nil {
		return err
	}
	if err := role.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure role schema: %w", err)
	}
	if err := authplatform.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure authentication platform schema: %w", err)
	}
	if err := auth.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure authentication schema: %w", err)
	}
	if err := menu.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure menu schema: %w", err)
	}
	if err := access.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure access schema: %w", err)
	}
	if err := operationlog.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure operation log schema: %w", err)
	}

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
	if err := auth.CleanupLegacySessionPointers(processContext, redisClient); err != nil {
		return fmt.Errorf("remove legacy current session keys: %w", err)
	}
	accessStateStore := accessstate.NewStore(redisClient)
	accessInvalidator := accessstate.NewInvalidator(accessStateStore)
	menuRepository := menu.NewRepository(postgres.GORM)
	menuService := menu.NewService(menuRepository, accessInvalidator)
	if err := menuService.EnsureFoundation(processContext, menuFoundation()); err != nil {
		return fmt.Errorf("ensure menu foundation: %w", err)
	}

	roleRepository := role.NewRepository(postgres.GORM)
	roleService := role.NewService(roleRepository, accessInvalidator)
	if err := roleService.EnsureSystemRoles(processContext); err != nil {
		return fmt.Errorf("ensure system roles: %w", err)
	}
	keys, err := secretkey.New(settings.AppSecret)
	if err != nil {
		return fmt.Errorf("derive application keys: %w", err)
	}

	repository := taskdemo.NewRepository(postgres.GORM)
	taskService := taskdemo.NewService(repository, taskdemo.NewQueueEnqueuer(queueClient), logger)
	healthService := health.NewService(postgres, redisClient)
	userRepository := user.NewRepository(postgres.GORM)
	sessionRepository := auth.NewSessionRepository(postgres.GORM)
	authPlatformRepository := authplatform.NewRepository(postgres.GORM)
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
	authService.SetSessionAdminRepository(sessionRepository)
	userService := user.NewService(userRepository, authStateStore, authInvalidator, accessStateStore, accessInvalidator)
	accessRepository := access.NewRepository(postgres.GORM)
	accessService := access.NewService(accessRepository, accessStateStore, access.NewSnapshotCache(redisClient), logger)
	operationLogRepository := operationlog.NewRepository(postgres.GORM)
	operationLogService := operationlog.NewService(operationLogRepository)
	operationLogEnqueuer := operationlog.NewQueueEnqueuer(queueClient)
	authenticate := auth.Authenticate(authService)
	router := buildRouter(routerDependencies{
		CORSOrigin:     settings.CORSOrigin,
		TrustedProxies: settings.TrustedProxies,
		Logger:         logger,
		Health:         health.NewHandler(healthService),
		Task:           taskdemo.NewHandler(taskService),
		Auth:           auth.NewHandler(authService, settings.Auth.CookieSecure),
		AuthPlatform:   authplatform.NewHandler(authPlatformService),
		Access:         access.NewHandler(accessService),
		Menu:           menu.NewHandler(menuService),
		Role:           role.NewHandler(roleService),
		User: user.NewHandler(userService, func(context *gin.Context) (int64, bool) {
			identity, ok := auth.IdentityFromContext(context)
			return identity.UserID, ok
		}),
		OperationLog:      operationlog.NewHandler(operationLogService),
		OperationEnqueuer: operationLogEnqueuer,
		SessionAdmin:      auth.NewSessionAdminHandler(authService),
		AuthOrigin:        auth.RequireOrigin(settings.CORSOrigin),
		Authenticate:      authenticate,
		RequirePermission: func(code string) gin.HandlerFunc {
			return access.RequirePermission(accessService, code)
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
	apiRoutes := router.Group("/api/v1")
	apiRoutes.Use(authclient.Require())
	auth.RegisterRoutes(apiRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	authplatform.RegisterPublicRoutes(apiRoutes, dependencies.AuthPlatform)
	authplatform.RegisterManagementRoutes(apiRoutes, dependencies.AuthPlatform, dependencies.Authenticate, dependencies.RequirePermission)
	access.RegisterRoutes(apiRoutes, dependencies.Access, dependencies.Authenticate)
	menu.RegisterRoutes(apiRoutes, dependencies.Menu, dependencies.Authenticate, dependencies.RequirePermission)
	role.RegisterRoutes(apiRoutes, dependencies.Role, dependencies.Authenticate, dependencies.RequirePermission)
	user.RegisterRoutes(apiRoutes, dependencies.User, dependencies.Authenticate, dependencies.RequirePermission)
	operationlog.RegisterRoutes(apiRoutes, dependencies.OperationLog, dependencies.Authenticate, dependencies.RequirePermission)
	auth.RegisterSessionAdminRoutes(apiRoutes, dependencies.SessionAdmin, dependencies.Authenticate, dependencies.RequirePermission)
	taskdemo.RegisterRoutes(apiRoutes, dependencies.Task, dependencies.Authenticate)
	return router
}
