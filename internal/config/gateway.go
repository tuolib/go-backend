package config

import "github.com/knadh/koanf/v2"

// GatewayConfig holds configuration for the API Gateway service.
type GatewayConfig struct {
	Common
	Redis    Redis    `koanf:"redis"`
	JWT      JWT      `koanf:"jwt"`
	Internal Internal `koanf:"internal"`
	CORS     CORS     `koanf:"cors"`

	Port              string `koanf:"api_gateway_port"`
	UserServiceURL    string `koanf:"user_service_url"`
	ProductServiceURL string `koanf:"product_service_url"`
	CartServiceURL    string `koanf:"cart_service_url"`
	OrderServiceURL   string `koanf:"order_service_url"`
}

// LoadGateway loads gateway-specific configuration from environment.
func LoadGateway(k *koanf.Koanf) GatewayConfig {
	var cfg GatewayConfig
	cfg.Port = k.String("api_gateway_port")
	if cfg.Port == "" {
		cfg.Port = "3000"
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Redis.URL = k.String("redis_url")
	cfg.JWT.AccessSecret = k.String("jwt_access_secret")
	cfg.JWT.RefreshSecret = k.String("jwt_refresh_secret")
	cfg.JWT.AccessExpiresIn = k.Duration("jwt_access_expires_in")
	cfg.JWT.RefreshExpiresIn = k.Duration("jwt_refresh_expires_in")
	cfg.Internal.Secret = k.String("internal_secret")
	cfg.CORS.Origins = k.Strings("cors_origins")

	cfg.UserServiceURL = k.String("user_service_url")
	cfg.ProductServiceURL = k.String("product_service_url")
	cfg.CartServiceURL = k.String("cart_service_url")
	cfg.OrderServiceURL = k.String("order_service_url")

	return cfg
}
