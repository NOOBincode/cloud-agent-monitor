//go:build wireinject

package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"gorm.io/gorm"

	"cloud-agent-monitor/internal/alerting/application"
	"cloud-agent-monitor/internal/alerting/infrastructure"
	alerthttp "cloud-agent-monitor/internal/alerting/interfaces/http"
	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/platform"
	"cloud-agent-monitor/internal/promclient"
	sloapp "cloud-agent-monitor/internal/slo/application"
	sloinfra "cloud-agent-monitor/internal/slo/infrastructure"
	slohttp "cloud-agent-monitor/internal/slo/interfaces/http"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/internal/user"
	"cloud-agent-monitor/pkg/config"
	"cloud-agent-monitor/pkg/infra"
	"cloud-agent-monitor/pkg/version"
)

func InitializeApp() (*App, error) {
	wire.Build(
		ProvideConfig,
		ProvideDatabase,
		ProvideServiceRepository,
		ProvideUserRepository,
		ProvideAPIKeyRepository,
		ProvideRoleRepository,
		ProvideJWTService,
		ProvideAPIKeyService,
		ProvideUserService,
		ProvideUserHandler,
		ProvideAuthMiddleware,
		ProvideCasbinEnforcer,
		ProvidePrometheusClient,
		ProvideHealthCheckService,
		ProvideServiceHandler,
		ProvideAlertmanagerClient,
		ProvideAlertOperationRepository,
		ProvideAlertNoiseRepository,
		ProvideAlertNotificationRepository,
		ProvideAlertRecordRepository,
		ProvideCache,
		ProvideQueue,
		ProvideAlertRecordBuffer,
		ProvideAlertService,
		ProvideAlertHandler,
		ProvideSLORepository,
		ProvideSLIRepository,
		ProvideSLICollector,
		ProvideSLOService,
		ProvideSLOHandler,
		ProvideHTTPServer,
		ProvideApp,
	)
	return nil, nil
}

func ProvideConfig() (*config.Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath != "" {
		return config.LoadWithPath(configPath)
	}
	return config.Load()
}

func ProvideDatabase(cfg *config.Config) (*gorm.DB, error) {
	db, err := storage.NewMySQLDB(cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.UserRole{},
		&models.APIKey{},
		&models.Service{},
		&models.ServiceLabel{},
		&models.ServiceDependency{},
		&models.LoginLog{},
		&models.AlertOperation{},
		&models.AlertNoiseRecord{},
		&models.AlertNotification{},
		&models.AlertRecord{},
		&models.SLO{},
		&models.SLI{},
		&models.ErrorBudgetHistory{},
		&models.BurnRateAlert{},
	); err != nil {
		return nil, err
	}
	return db, nil
}

func ProvideServiceRepository(db *gorm.DB) storage.ServiceRepositoryInterface {
	return storage.NewServiceRepository(db)
}

func ProvideUserRepository(db *gorm.DB) storage.UserRepositoryInterface {
	return storage.NewUserRepository(db)
}

func ProvideAPIKeyRepository(db *gorm.DB) storage.APIKeyRepositoryInterface {
	return storage.NewAPIKeyRepository(db)
}

func ProvideRoleRepository(db *gorm.DB) storage.RoleRepositoryInterface {
	return storage.NewRoleRepository(db)
}

func ProvideJWTService(cfg *config.Config) *auth.JWTService {
	jwtConfig := auth.JWTConfig{
		SecretKey:       cfg.JWT.SecretKey,
		AccessTokenTTL:  time.Duration(cfg.JWT.AccessTokenExpiry) * time.Minute,
		RefreshTokenTTL: time.Duration(cfg.JWT.RefreshTokenExpiry) * time.Hour * 24,
		Issuer:          cfg.JWT.Issuer,
	}
	return auth.NewJWTService(jwtConfig)
}

func ProvideAPIKeyService(repo storage.APIKeyRepositoryInterface) *auth.APIKeyService {
	return auth.NewAPIKeyService(repo)
}

func ProvideUserService(
	userRepo storage.UserRepositoryInterface,
	apiKeyRepo storage.APIKeyRepositoryInterface,
	jwtService *auth.JWTService,
) *user.Service {
	return user.NewService(userRepo, apiKeyRepo, jwtService)
}

func ProvideUserHandler(
	userService *user.Service,
	apiKeyService *auth.APIKeyService,
	roleRepo storage.RoleRepositoryInterface,
) *user.Handler {
	return user.NewHandlerWithRole(userService, apiKeyService, roleRepo)
}

func ProvideAuthMiddleware(apiKeyService *auth.APIKeyService, jwtService *auth.JWTService) *auth.AuthMiddleware {
	adapter := auth.NewAPIKeyValidatorAdapter(apiKeyService)
	return auth.NewAuthMiddlewareWithJWT(adapter, jwtService)
}

func ProvideCasbinEnforcer(cfg *config.Config) (*auth.CasbinEnforcer, error) {
	return auth.NewCasbinEnforcer(cfg)
}

func ProvidePrometheusClient(cfg *config.Config) *promclient.Client {
	return promclient.NewClient(cfg.Prometheus)
}

func ProvideHealthCheckService(
	repo storage.ServiceRepositoryInterface,
	promClient *promclient.Client,
	cfg *config.Config,
) *platform.HealthCheckService {
	return platform.NewHealthCheckService(repo, promClient, cfg.HealthCheck)
}

func ProvideServiceHandler(
	repo storage.ServiceRepositoryInterface,
	healthCheckService *platform.HealthCheckService,
) *platform.ServiceHandler {
	return platform.NewServiceHandlerWithHealthCheck(repo, healthCheckService)
}

func ProvideAlertmanagerClient(cfg *config.Config) *infrastructure.AlertmanagerClient {
	return infrastructure.NewAlertmanagerClient(cfg.Alertmanager)
}

func ProvideAlertOperationRepository(db *gorm.DB) storage.AlertOperationRepositoryInterface {
	return storage.NewAlertOperationRepository(db)
}

func ProvideAlertNoiseRepository(db *gorm.DB) storage.AlertNoiseRepositoryInterface {
	return storage.NewAlertNoiseRepository(db)
}

func ProvideAlertNotificationRepository(db *gorm.DB) storage.AlertNotificationRepositoryInterface {
	return storage.NewAlertNotificationRepository(db)
}

func ProvideAlertRecordRepository(db *gorm.DB) storage.AlertRecordRepositoryInterface {
	return storage.NewAlertRecordRepository(db)
}

func ProvideCache() *infra.Cache {
	return infra.NewCache(100)
}

func ProvideQueue(cfg *config.Config) *infra.Queue {
	return infra.NewQueue(infra.QueueConfig{
		RedisAddr:     cfg.Redis.Addr,
		RedisPassword: cfg.Redis.Password,
		RedisDB:       cfg.Redis.DB,
		Concurrency:   4,
	})
}

func ProvideAlertRecordBuffer(
	repo storage.AlertRecordRepositoryInterface,
	queue *infra.Queue,
) *infrastructure.AlertRecordBuffer {
	return infrastructure.NewAlertRecordBuffer(repo, queue, infrastructure.DefaultAlertRecordBufferConfig())
}

func ProvideAlertService(
	amClient *infrastructure.AlertmanagerClient,
	opRepo storage.AlertOperationRepositoryInterface,
	noiseRepo storage.AlertNoiseRepositoryInterface,
	notifyRepo storage.AlertNotificationRepositoryInterface,
	recordRepo storage.AlertRecordRepositoryInterface,
	cache *infra.Cache,
	queue *infra.Queue,
	recordBuffer *infrastructure.AlertRecordBuffer,
) *application.AlertService {
	return application.NewAlertService(amClient, opRepo, noiseRepo, notifyRepo, recordRepo, cache, queue, recordBuffer)
}

func ProvideAlertHandler(service *application.AlertService) *alerthttp.Handler {
	return alerthttp.NewHandler(service)
}

func ProvideSLORepository(db *gorm.DB) *storage.SLORepository {
	return storage.NewSLORepository(db)
}

func ProvideSLIRepository(db *gorm.DB) *storage.SLIRepository {
	return storage.NewSLIRepository(db)
}

func ProvideSLICollector(promClient *promclient.Client) *sloinfra.PrometheusSLICollector {
	return sloinfra.NewPrometheusSLICollector(promClient)
}

func ProvideSLOService(
	sloRepo *storage.SLORepository,
	sliRepo *storage.SLIRepository,
	collector *sloinfra.PrometheusSLICollector,
	cache *infra.Cache,
) *sloapp.SLOService {
	return sloapp.NewSLOService(sloRepo, sliRepo, collector, cache)
}

func ProvideSLOHandler(service *sloapp.SLOService) *slohttp.Handler {
	return slohttp.NewHandler(service)
}

func ProvideHTTPServer(
	db *gorm.DB,
	repo storage.ServiceRepositoryInterface,
	userHandler *user.Handler,
	authMiddleware *auth.AuthMiddleware,
	casbinEnforcer *auth.CasbinEnforcer,
	serviceHandler *platform.ServiceHandler,
	healthCheckService *platform.HealthCheckService,
	alertHandler *alerthttp.Handler,
	sloHandler *slohttp.Handler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		dbStatus := "ok"
		if err := storage.Ping(ctx, db); err != nil {
			dbStatus = "error: " + err.Error()
		}

		status := http.StatusOK
		if dbStatus != "ok" {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"status":   "ok",
			"service":  version.Name,
			"version":  version.Version,
			"database": dbStatus,
		})
	})

	v1 := r.Group("/api/v1")

	alerthttp.RegisterPublicRoutes(v1, alertHandler)

	user.RegisterRoutes(v1, userHandler, authMiddleware)

	protected := v1.Group("")
	protected.Use(authMiddleware.RequireAPIKey())
	protected.Use(casbinEnforcer.CasbinMiddleware())
	platform.RegisterRoutesWithHealthCheck(protected, repo, healthCheckService)
	alerthttp.RegisterRoutes(protected, alertHandler)
	slohttp.RegisterRoutes(protected, sloHandler)

	healthCheckService.Start()

	return r
}

func ProvideApp(cfg *config.Config, db *gorm.DB, server *gin.Engine) *App {
	return &App{
		config: cfg,
		server: server,
		db:     db,
	}
}

var ProviderSet = wire.NewSet(
	ProvideConfig,
	ProvideDatabase,
	ProvideServiceRepository,
	ProvideUserRepository,
	ProvideAPIKeyRepository,
	ProvideRoleRepository,
	ProvideJWTService,
	ProvideAPIKeyService,
	ProvideUserService,
	ProvideUserHandler,
	ProvideAuthMiddleware,
	ProvideCasbinEnforcer,
	ProvidePrometheusClient,
	ProvideHealthCheckService,
	ProvideServiceHandler,
	ProvideAlertmanagerClient,
	ProvideAlertOperationRepository,
	ProvideAlertNoiseRepository,
	ProvideAlertNotificationRepository,
	ProvideAlertRecordRepository,
	ProvideCache,
	ProvideQueue,
	ProvideAlertRecordBuffer,
	ProvideAlertService,
	ProvideAlertHandler,
	ProvideSLORepository,
	ProvideSLIRepository,
	ProvideSLICollector,
	ProvideSLOService,
	ProvideSLOHandler,
	ProvideHTTPServer,
	ProvideApp,
)
