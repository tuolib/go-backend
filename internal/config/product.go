package config

import "github.com/knadh/koanf/v2"

// ProductServiceConfig holds configuration for the Product service.
type ProductServiceConfig struct {
	Common
	Postgres Postgres `koanf:"postgres"`
	Redis    Redis    `koanf:"redis"`
	Internal Internal `koanf:"internal"`

	Port string `koanf:"product_service_port"`
}

// LoadProduct loads product-service-specific configuration from environment.
func LoadProduct(k *koanf.Koanf) ProductServiceConfig {
	var cfg ProductServiceConfig
	cfg.Port = k.String("product_service_port")
	if cfg.Port == "" {
		cfg.Port = "3002"
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Postgres.URL = k.String("database_url")
	cfg.Redis.URL = k.String("redis_url")
	cfg.Internal.Secret = k.String("internal_secret")

	return cfg
}
