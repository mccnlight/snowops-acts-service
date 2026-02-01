package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type HTTPConfig struct {
	Host string
	Port int
}

type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime string
}

type AuthConfig struct {
	AccessSecret string
}

type ActsConfig struct {
	ValidStatuses   []string
}

type Config struct {
	Environment string
	HTTP        HTTPConfig
	DB          DBConfig
	Auth        AuthConfig
	Acts        ActsConfig
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("app")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("./deploy")
	v.AddConfigPath("./internal/config")
	v.AutomaticEnv()

	_ = v.ReadInConfig()

	cfg := &Config{
		Environment: v.GetString("APP_ENV"),
		HTTP: HTTPConfig{
			Host: v.GetString("HTTP_HOST"),
			Port: v.GetInt("HTTP_PORT"),
		},
		DB: DBConfig{
			DSN:             v.GetString("DB_DSN"),
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetString("DB_CONN_MAX_LIFETIME"),
		},
		Auth: AuthConfig{
			AccessSecret: v.GetString("JWT_ACCESS_SECRET"),
		},
		Acts: ActsConfig{
			ValidStatuses:   parseList(v.GetString("ACTS_VALID_STATUSES")),
		},
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = "0.0.0.0"
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 7089
	}
	if len(cfg.Acts.ValidStatuses) == 0 {
		cfg.Acts.ValidStatuses = []string{"OK"}
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validate(cfg *Config) error {
	if cfg.DB.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if cfg.Auth.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	return nil
}

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
