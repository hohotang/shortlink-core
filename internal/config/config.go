package config

import (
	"strings"
	"time"

	"github.com/hohotang/shortlink-core/internal/logger"
	"github.com/hohotang/shortlink-core/internal/models"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

// ServerConfig holds the server configuration
type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

// StorageConfig holds the storage configuration
type StorageConfig struct {
	Type        models.StorageType `mapstructure:"type"`
	RedisURL    string             `mapstructure:"redis_url"`
	PostgresURL string             `mapstructure:"postgres_url"`
	CacheTTL    int                `mapstructure:"cache_ttl"`
	Postgres    PostgresConfig     `mapstructure:"postgres"`
}

// PostgresConfig holds detailed PostgreSQL configuration
type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// SnowflakeConfig holds the Snowflake ID generator configuration
type SnowflakeConfig struct {
	MachineID int64 `mapstructure:"machine_id"`
}

// TelemetryConfig holds the OpenTelemetry configuration
type TelemetryConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`
	ServiceName  string `mapstructure:"service_name"`
	Environment  string `mapstructure:"environment"`
}

// Load reads the configuration from config.yaml or environment variables
func Load() (*Config, error) {
	// Initialize viper
	v := viper.New()

	// Set default values
	v.SetDefault("server.port", 50051)
	v.SetDefault("server.base_url", "http://localhost:8080/")
	v.SetDefault("storage.type", "memory")
	v.SetDefault("storage.redis_url", "redis://localhost:6379")
	v.SetDefault("storage.postgres_url", "postgres://postgres:postgres@localhost:5432/shortlink?sslmode=disable")
	v.SetDefault("storage.cache_ttl", 3600)
	v.SetDefault("storage.postgres.host", "localhost")
	v.SetDefault("storage.postgres.port", 5432)
	v.SetDefault("storage.postgres.user", "postgres")
	v.SetDefault("storage.postgres.password", "postgres")
	v.SetDefault("storage.postgres.dbname", "shortlink")
	v.SetDefault("storage.postgres.sslmode", "disable")
	v.SetDefault("storage.postgres.max_open_conns", 25)
	v.SetDefault("storage.postgres.max_idle_conns", 5)
	v.SetDefault("storage.postgres.conn_max_lifetime", 5*time.Minute)
	v.SetDefault("snowflake.machine_id", 1)
	v.SetDefault("telemetry.enabled", false)
	v.SetDefault("telemetry.otlp_endpoint", "localhost:4318")
	v.SetDefault("telemetry.service_name", "shortlink-core")
	v.SetDefault("telemetry.environment", "development")

	// Set config file specifics
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	// Configure environment variable support
	v.AutomaticEnv()
	v.SetEnvPrefix("SHORTLINK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Create a logger (note: proper initialization happens later, this is just for config load)
	log, _ := zap.NewProduction()
	if logger.L() != nil {
		log = logger.L()
	}
	defer log.Sync()

	// Read config file if exists
	err := v.ReadInConfig()
	if err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		} else {
			log.Info("Config file not found, using default values")
		}
	} else {
		log.Info("Using config file", zap.String("file", v.ConfigFileUsed()))
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	log.Info("Configuration loaded",
		zap.String("storageType", string(cfg.Storage.Type)),
		zap.Int("serverPort", cfg.Server.Port),
		zap.Bool("telemetryEnabled", cfg.Telemetry.Enabled))

	return cfg, nil
}
