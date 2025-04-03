package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

// StorageConfig represents the storage configuration
type StorageConfig struct {
	Type        string `mapstructure:"type"` // "memory", "redis", "postgres", "both"
	RedisURL    string `mapstructure:"redis_url"`
	PostgresURL string `mapstructure:"postgres_url"`
	CacheTTL    int    `mapstructure:"cache_ttl"` // Redis cache TTL in seconds
}

// SnowflakeConfig represents the configuration for snowflake ID generation
type SnowflakeConfig struct {
	MachineID int64 `mapstructure:"machine_id"`
}

// Load loads the configuration using Viper
func Load() (*Config, error) {
	v := viper.New()

	// Set default values
	v.SetDefault("server.port", 50051)
	v.SetDefault("storage.type", "memory")
	v.SetDefault("storage.redis_url", "redis://localhost:6379")
	v.SetDefault("storage.postgres_url", "postgres://postgres:postgres@localhost:5432/shortlink?sslmode=disable")
	v.SetDefault("storage.cache_ttl", 3600) // 1 hour
	v.SetDefault("snowflake.machine_id", 1)

	// Add multiple search paths for config file
	v.SetConfigName("config") // config.yaml
	v.SetConfigType("yaml")
	v.AddConfigPath(".")        // 專案根目錄
	v.AddConfigPath("./config") // 也支援 config/ 資料夾

	// Read environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("SHORTLINK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file if exists
	err := v.ReadInConfig()
	if err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		} else {
			log.Printf("Config file not found, using default values")
		}
	} else {
		log.Printf("Using config file: %s", v.ConfigFileUsed())
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
