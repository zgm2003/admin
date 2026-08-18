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
	"admin/server/internal/module/auth"
	"admin/server/internal/module/health"
	"admin/server/internal/module/menu"
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
	CORSOrigin   string
	Logger       *slog.Logger
	Health       *health.Handler
	Task         *taskdemo.Handler
	Auth         *auth.Handler
	Access       *access.Handler
	AuthOrigin   gin.HandlerFunc
	Authenticate gin.HandlerFunc
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
	if err := database.AutoMigrate(
		processContext,
		postgres.GORM,
		&taskdemo.Task{},
		&user.User{},
		&role.Role{},
		&role.UserRole{},
		&menu.Menu{},
		&menu.RoleMenu{},
		&auth.Session{},
	); err != nil {
		return err
	}
	if err := auth.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure authentication schema: %w", err)
	}
	if err := menu.EnsureSchema(processContext, postgres.GORM); err != nil {
		return fmt.Errorf("ensure menu schema: %w", err)
	}

	roleRepository := role.NewRepository(postgres.GORM)
	if err := roleRepository.EnsureSystemRoles(processContext); err != nil {
		return fmt.Errorf("ensure system roles: %w", err)
	}
	keys, err := secretkey.New(settings.AppSecret)
	if err != nil {
		return fmt.Errorf("derive application keys: %w", err)
	}

	queueClient, err := queue.NewClient(settings.RedisURL)
	if err != nil {
		return err
	}
	defer queueClient.Close()

	repository := taskdemo.NewRepository(postgres.GORM)
	taskService := taskdemo.NewService(repository, taskdemo.NewQueueEnqueuer(queueClient), logger)
	healthService := health.NewService(postgres, redisClient)
	userRepository := user.NewRepository(postgres.GORM)
	sessionRepository := auth.NewSessionRepository(postgres.GORM)
	authService := auth.NewService(
		userRepository,
		roleRepository,
		sessionRepository,
		redisClient,
		auth.NewJWT(keys.JWTSigningKey()),
		keys.RefreshTokenHMACKey(),
	)
	accessRepository := access.NewRepository(postgres.GORM)
	accessService := access.NewService(accessRepository)
	authenticate := auth.Authenticate(authService)
	router := buildRouter(routerDependencies{
		CORSOrigin:   settings.CORSOrigin,
		Logger:       logger,
		Health:       health.NewHandler(healthService),
		Task:         taskdemo.NewHandler(taskService),
		Auth:         auth.NewHandler(authService, settings.Auth.CookieSecure),
		Access:       access.NewHandler(accessService),
		AuthOrigin:   auth.RequireOrigin(settings.CORSOrigin),
		Authenticate: authenticate,
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
	router.Use(
		projectmiddleware.RequestID(),
		projectmiddleware.CORS(dependencies.CORSOrigin),
		projectmiddleware.AccessLog(dependencies.Logger),
		projectmiddleware.Recovery(dependencies.Logger),
		projectmiddleware.Language(),
	)
	health.RegisterRoutes(router, dependencies.Health)
	apiRoutes := router.Group("/api/v1")
	auth.RegisterRoutes(apiRoutes, dependencies.Auth, dependencies.AuthOrigin, dependencies.Authenticate)
	access.RegisterRoutes(apiRoutes, dependencies.Access, dependencies.Authenticate)
	taskdemo.RegisterRoutes(apiRoutes, dependencies.Task, dependencies.Authenticate)
	return router
}
