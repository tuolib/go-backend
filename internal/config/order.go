package config

import "github.com/knadh/koanf/v2"

// OrderServiceConfig 订单服务配置。订单服务依赖最多——需要调用用户、商品、购物车三个服务。
// OrderServiceConfig holds configuration for the Order service. Order has the most dependencies — it calls User, Product, and Cart services.
type OrderServiceConfig struct {
	Common                                // 嵌入公共配置 / Embed common config
	Postgres Postgres `koanf:"postgres"`  // 订单数据持久化 / Order data persistence
	Redis    Redis    `koanf:"redis"`     // 分布式锁、幂等控制 / Distributed locks, idempotency control
	Internal Internal `koanf:"internal"`  // 内部服务通信鉴权 / Internal service auth

	Port              string `koanf:"order_service_port"`  // 订单服务监听端口 / Order service listen port
	UserServiceURL    string `koanf:"user_service_url"`    // 用户服务地址（查收货地址）/ User service URL (fetch delivery address)
	ProductServiceURL string `koanf:"product_service_url"` // 商品服务地址（查 SKU、扣库存）/ Product service URL (query SKU, deduct stock)
	CartServiceURL    string `koanf:"cart_service_url"`    // 购物车服务地址（清空购物车）/ Cart service URL (clear cart after order)
}

// LoadOrder 从 koanf 加载订单服务配置。
// LoadOrder loads order-service-specific configuration from koanf.
func LoadOrder(k *koanf.Koanf) OrderServiceConfig {
	var cfg OrderServiceConfig
	cfg.Port = k.String("order_service_port")
	if cfg.Port == "" {
		cfg.Port = "3004" // 默认端口 / Default port
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
