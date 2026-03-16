package config

import "github.com/knadh/koanf/v2"

// GatewayConfig API 网关配置。网关负责路由分发、限流、鉴权，不直接连数据库。
// GatewayConfig holds configuration for the API Gateway service. The gateway handles routing, rate-limiting, and auth — it doesn't connect to the database directly.
type GatewayConfig struct {
	Common                            // 嵌入公共字段（AppEnv、LogLevel）/ Embed common fields (AppEnv, LogLevel)
	Redis    Redis    `koanf:"redis"` // 限流、token 黑名单等需要 Redis / Redis for rate-limiting, token blacklist, etc.
	JWT      JWT      `koanf:"jwt"`
	Internal Internal `koanf:"internal"`
	CORS     CORS     `koanf:"cors"`

	Port              string `koanf:"api_gateway_port"`   // 网关监听端口 / Gateway listen port
	UserServiceURL    string `koanf:"user_service_url"`   // 用户服务地址（微服务模式）/ User service URL (microservice mode)
	ProductServiceURL string `koanf:"product_service_url"`
	CartServiceURL    string `koanf:"cart_service_url"`
	OrderServiceURL   string `koanf:"order_service_url"`
}

// LoadGateway 从 koanf 加载网关配置，未设置端口时使用默认值 3000。
// LoadGateway loads gateway-specific configuration from koanf, defaulting to port 3000.
func LoadGateway(k *koanf.Koanf) GatewayConfig {
	var cfg GatewayConfig
	cfg.Port = k.String("api_gateway_port")
	if cfg.Port == "" {
		cfg.Port = "3000" // 默认端口 / Default port
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

	// 微服务模式下，网关需要知道各服务的地址才能转发请求。
	// In microservice mode, the gateway needs each service's URL to proxy requests.
	cfg.UserServiceURL = k.String("user_service_url")
	cfg.ProductServiceURL = k.String("product_service_url")
	cfg.CartServiceURL = k.String("cart_service_url")
	cfg.OrderServiceURL = k.String("order_service_url")

	return cfg
}
