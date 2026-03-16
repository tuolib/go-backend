package config

import "github.com/knadh/koanf/v2"

// CartServiceConfig 购物车服务配置。购物车只用 Redis（不需要 PostgreSQL），因为购物车数据是临时性的。
// CartServiceConfig holds configuration for the Cart service. Cart uses Redis only (no PostgreSQL) since cart data is ephemeral.
type CartServiceConfig struct {
	Common                               // 嵌入公共配置 / Embed common config
	Redis    Redis    `koanf:"redis"`    // 购物车数据存储 / Cart data storage
	Internal Internal `koanf:"internal"` // 内部服务通信鉴权 / Internal service auth

	Port              string `koanf:"cart_service_port"`  // 购物车服务监听端口 / Cart service listen port
	ProductServiceURL string `koanf:"product_service_url"` // 商品服务地址，用于查询 SKU 信息和验证库存 / Product service URL for SKU info and stock validation
}

// LoadCart 从 koanf 加载购物车服务配置。
// LoadCart loads cart-service-specific configuration from koanf.
func LoadCart(k *koanf.Koanf) CartServiceConfig {
	var cfg CartServiceConfig
	cfg.Port = k.String("cart_service_port")
	if cfg.Port == "" {
		cfg.Port = "3003" // 默认端口 / Default port
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Redis.URL = k.String("redis_url")
	cfg.Internal.Secret = k.String("internal_secret")
	cfg.ProductServiceURL = k.String("product_service_url")

	return cfg
}
