package config

import "github.com/knadh/koanf/v2"

// UserServiceConfig 只包含用户服务需要的配置项。
// UserServiceConfig holds only the configuration needed by the User service.
//
// 每个服务有独立的 Config struct，避免加载不需要的配置——比如 Cart 服务不需要 JWT 配置。
// Each service has its own Config struct to avoid loading unneeded config — e.g. Cart doesn't need JWT.
//
// Common 通过结构体嵌入获得公共字段（AppEnv、LogLevel），这是 Go 的"组合优于继承"。
// Common is embedded to get shared fields (AppEnv, LogLevel) — this is Go's "composition over inheritance".
type UserServiceConfig struct {
	Common                                // 嵌入公共配置 / Embed common config
	Postgres Postgres `koanf:"postgres"`  // 用户数据持久化 / User data persistence
	Redis    Redis    `koanf:"redis"`     // 缓存、session 等 / Cache, sessions, etc.
	JWT      JWT      `koanf:"jwt"`       // 用户服务负责签发 token / User service is responsible for issuing tokens
	Internal Internal `koanf:"internal"`  // 内部服务通信鉴权 / Internal service auth

	Port string `koanf:"user_service_port"` // 用户服务监听端口 / User service listen port
}

// LoadUser 从 koanf 加载用户服务配置。
// LoadUser loads user-service-specific configuration from koanf.
func LoadUser(k *koanf.Koanf) UserServiceConfig {
	var cfg UserServiceConfig
	cfg.Port = k.String("user_service_port")
	if cfg.Port == "" {
		cfg.Port = "3001" // 默认端口 / Default port
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
