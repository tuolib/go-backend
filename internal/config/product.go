package config

import "github.com/knadh/koanf/v2"

// ProductServiceConfig 商品服务配置。商品服务不需要 JWT（鉴权由网关统一处理）。
// ProductServiceConfig holds configuration for the Product service. No JWT needed — auth is handled by the gateway.
type ProductServiceConfig struct {
	Common                                // 嵌入公共配置 / Embed common config
	Postgres Postgres `koanf:"postgres"`  // 商品/SKU/库存数据 / Product/SKU/stock data
	Redis    Redis    `koanf:"redis"`     // 库存缓存、Lua 脚本操作 / Stock cache, Lua script operations
	Internal Internal `koanf:"internal"`  // 内部服务通信鉴权 / Internal service auth

	Port string `koanf:"product_service_port"` // 商品服务监听端口 / Product service listen port
}

// LoadProduct 从 koanf 加载商品服务配置。
// LoadProduct loads product-service-specific configuration from koanf.
func LoadProduct(k *koanf.Koanf) ProductServiceConfig {
	var cfg ProductServiceConfig
	cfg.Port = k.String("product_service_port")
	if cfg.Port == "" {
		cfg.Port = "3002" // 默认端口 / Default port
	}

	cfg.AppEnv = k.String("app_env")
	cfg.LogLevel = k.String("log_level")
	cfg.Postgres.URL = k.String("database_url")
	cfg.Redis.URL = k.String("redis_url")
	cfg.Internal.Secret = k.String("internal_secret")

	return cfg
}
