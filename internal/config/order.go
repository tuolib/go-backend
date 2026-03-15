package config

import "github.com/knadh/koanf/v2"

// OrderServiceConfig holds configuration for the Order service.
type OrderServiceConfig struct {
	Common
	Postgres Postgres `koanf:"postgres"`
	Redis    Redis    `koanf:"redis"`
	Internal Internal `koanf:"internal"`

	Port              string `koanf:"order_service_port"`
	UserServiceURL    string `koanf:"user_service_url"`
	ProductServiceURL string `koanf:"product_service_url"`
	CartServiceURL    string `koanf:"cart_service_url"`
}

// LoadOrder loads order-service-specific configuration from environment.
func LoadOrder(k *koanf.Koanf) OrderServiceConfig {
	var cfg OrderServiceConfig
	cfg.Port = k.String("order_service_port")
	if cfg.Port == "" {
		cfg.Port = "3004"
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Postgres.URL = k.String("database_url")
	cfg.Redis.URL = k.String("redis_url")
	cfg.Internal.Secret = k.String("internal_secret")
	cfg.UserServiceURL = k.String("user_service_url")
	cfg.ProductServiceURL = k.String("product_service_url")
	cfg.CartServiceURL = k.String("cart_service_url")

	return cfg
}
