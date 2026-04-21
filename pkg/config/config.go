package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var globalViper *viper.Viper

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Log          LogConfig
	JWT          JWTConfig
	Prometheus   PrometheusConfig
	HealthCheck  HealthCheckConfig
	Alertmanager AlertmanagerConfig
	Redis        RedisConfig
	Topology     TopologyConfig
}

type TopologyConfig struct {
	RefreshInterval time.Duration
	CacheTTL        time.Duration
	MaxDepth        int
	LocalCacheSize  int
	Kubernetes      K8sTopologyConfig
}

type K8sTopologyConfig struct {
	Enabled    bool
	MasterURL  string
	Kubeconfig string
	Namespaces []string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AlertmanagerConfig struct {
	URL        string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

type ServerConfig struct {
	Addr            string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Driver          string
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	Charset         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (c DatabaseConfig) DSN() string {
	sslmode := "disable"
	if c.Driver == "postgres" || c.Driver == "postgresql" {
		return fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=obs_platform",
			c.Host, c.Port, c.User, c.Password, c.Database, sslmode,
		)
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database, c.Charset,
	)
}

type LogConfig struct {
	Level  string
	Format string
}

type JWTConfig struct {
	SecretKey          string
	AccessTokenExpiry  int
	RefreshTokenExpiry int
	Issuer             string
}

type PrometheusConfig struct {
	URL        string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

type HealthCheckConfig struct {
	Enabled            bool
	CheckInterval      time.Duration
	UnhealthyThreshold int
	HealthyThreshold   int
}

func Load() (*Config, error) {
	return LoadWithPath("")
}

func LoadWithPath(configPath string) (*Config, error) {
	globalViper = viper.New()

	if configPath != "" {
		globalViper.SetConfigFile(configPath)
	} else {
		globalViper.SetConfigName("config")
		globalViper.SetConfigType("yaml")

		globalViper.AddConfigPath(".")
		globalViper.AddConfigPath("./configs")
		globalViper.AddConfigPath("./deploy/configs")

		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			globalViper.AddConfigPath(exeDir)
			globalViper.AddConfigPath(filepath.Join(exeDir, "configs"))
		}
	}

	if err := globalViper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	globalViper.SetEnvPrefix("PLATFORM")
	globalViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	globalViper.AutomaticEnv()

	cfg := &Config{
		Server: ServerConfig{
			Addr:            globalViper.GetString("server.addr"),
			ShutdownTimeout: globalViper.GetDuration("server.shutdown_timeout"),
		},
		Database: DatabaseConfig{
			Driver:          globalViper.GetString("database.driver"),
			Host:            globalViper.GetString("database.host"),
			Port:            globalViper.GetInt("database.port"),
			User:            globalViper.GetString("database.user"),
			Password:        globalViper.GetString("database.password"),
			Database:        globalViper.GetString("database.database"),
			Charset:         globalViper.GetString("database.charset"),
			MaxOpenConns:    globalViper.GetInt("database.max_open_conns"),
			MaxIdleConns:    globalViper.GetInt("database.max_idle_conns"),
			ConnMaxLifetime: globalViper.GetDuration("database.conn_max_lifetime"),
			ConnMaxIdleTime: globalViper.GetDuration("database.conn_max_idle_time"),
		},
		Log: LogConfig{
			Level:  globalViper.GetString("log.level"),
			Format: globalViper.GetString("log.format"),
		},
		JWT: JWTConfig{
			SecretKey:          globalViper.GetString("jwt.secret_key"),
			AccessTokenExpiry:  globalViper.GetInt("jwt.access_token_expiry"),
			RefreshTokenExpiry: globalViper.GetInt("jwt.refresh_token_expiry"),
			Issuer:             globalViper.GetString("jwt.issuer"),
		},
		Prometheus: PrometheusConfig{
			URL:        globalViper.GetString("prometheus.url"),
			Timeout:    globalViper.GetDuration("prometheus.timeout"),
			MaxRetries: globalViper.GetInt("prometheus.max_retries"),
			RetryDelay: globalViper.GetDuration("prometheus.retry_delay"),
		},
		HealthCheck: HealthCheckConfig{
			Enabled:            globalViper.GetBool("health_check.enabled"),
			CheckInterval:      globalViper.GetDuration("health_check.check_interval"),
			UnhealthyThreshold: globalViper.GetInt("health_check.unhealthy_threshold"),
			HealthyThreshold:   globalViper.GetInt("health_check.healthy_threshold"),
		},
		Alertmanager: AlertmanagerConfig{
			URL:        globalViper.GetString("alertmanager.url"),
			Timeout:    globalViper.GetDuration("alertmanager.timeout"),
			MaxRetries: globalViper.GetInt("alertmanager.max_retries"),
			RetryDelay: globalViper.GetDuration("alertmanager.retry_delay"),
		},
		Redis: RedisConfig{
			Addr:     globalViper.GetString("redis.addr"),
			Password: globalViper.GetString("redis.password"),
			DB:       globalViper.GetInt("redis.db"),
		},
		Topology: TopologyConfig{
			RefreshInterval: globalViper.GetDuration("topology.refresh_interval"),
			CacheTTL:        globalViper.GetDuration("topology.cache_ttl"),
			MaxDepth:        globalViper.GetInt("topology.max_depth"),
			LocalCacheSize:  globalViper.GetInt("topology.local_cache_size"),
			Kubernetes: K8sTopologyConfig{
				Enabled:    globalViper.GetBool("topology.kubernetes.enabled"),
				MasterURL:  globalViper.GetString("topology.kubernetes.master_url"),
				Kubeconfig: globalViper.GetString("topology.kubernetes.kubeconfig"),
				Namespaces: globalViper.GetStringSlice("topology.kubernetes.namespaces"),
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []error

	if c.Server.Addr == "" {
		errs = append(errs, errors.New("server.addr is required"))
	}

	if c.Database.Database == "" {
		errs = append(errs, errors.New("database.database is required"))
	}
	if c.Database.User == "" {
		errs = append(errs, errors.New("database.user is required"))
	}

	if c.JWT.SecretKey == "" {
		errs = append(errs, errors.New("jwt.secret_key is required"))
	}
	if len(c.JWT.SecretKey) < 32 {
		errs = append(errs, errors.New("jwt.secret_key must be at least 32 characters for security"))
	}

	if c.JWT.AccessTokenExpiry <= 0 {
		errs = append(errs, errors.New("jwt.access_token_expiry must be positive"))
	}
	if c.JWT.RefreshTokenExpiry <= 0 {
		errs = append(errs, errors.New("jwt.refresh_token_expiry must be positive"))
	}

	if c.Database.MaxOpenConns < c.Database.MaxIdleConns {
		errs = append(errs, errors.New("database.max_open_conns must be >= database.max_idle_conns"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %v", errs)
	}

	return nil
}

func (c *Config) IsDevelopment() bool {
	return c.Log.Level == "debug" || strings.HasPrefix(c.Server.Addr, "127.0.0.1") || strings.HasPrefix(c.Server.Addr, "localhost")
}

func (c *Config) IsProduction() bool {
	return !c.IsDevelopment()
}
