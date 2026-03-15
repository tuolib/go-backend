package config

import "github.com/knadh/koanf/v2"

// CartServiceConfig holds configuration for the Cart service.
// Cart uses Redis only (no PostgreSQL).
type CartServiceConfig struct {
	Common
	Redis    Redis    `koanf:"redis"`
	Internal Internal `koanf:"internal"`

	Port              string `koanf:"cart_service_port"`
	ProductServiceURL string `koanf:"product_service_url"`
}

// LoadCart loads cart-service-specific configuration from environment.
func LoadCart(k *koanf.Koanf) CartServiceConfig {
	var cfg CartServiceConfig
	cfg.Port = k.String("cart_service_port")
	if cfg.Port == "" {
		cfg.Port = "3003"
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Redis.URL = k.String("redis_url")
	cfg.Internal.Secret = k.String("internal_secret")
	cfg.ProductServiceURL = k.String("product_service_url")

	return cfg
}
