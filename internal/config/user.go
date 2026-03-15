package config

import "github.com/knadh/koanf/v2"

// UserServiceConfig holds configuration for the User service.
type UserServiceConfig struct {
	Common
	Postgres Postgres `koanf:"postgres"`
	Redis    Redis    `koanf:"redis"`
	JWT      JWT      `koanf:"jwt"`
	Internal Internal `koanf:"internal"`

	Port string `koanf:"user_service_port"`
}

// LoadUser loads user-service-specific configuration from environment.
func LoadUser(k *koanf.Koanf) UserServiceConfig {
	var cfg UserServiceConfig
	cfg.Port = k.String("user_service_port")
	if cfg.Port == "" {
		cfg.Port = "3001"
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Postgres.URL = k.String("database_url")
	cfg.Redis.URL = k.String("redis_url")
	cfg.JWT.AccessSecret = k.String("jwt_access_secret")
	cfg.JWT.RefreshSecret = k.String("jwt_refresh_secret")
	cfg.JWT.AccessExpiresIn = k.Duration("jwt_access_expires_in")
	cfg.JWT.RefreshExpiresIn = k.Duration("jwt_refresh_expires_in")
	cfg.Internal.Secret = k.String("internal_secret")

	return cfg
}
