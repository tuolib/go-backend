package config

import (
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Common 所有服务共享的配置字段。
// Common holds fields shared by all services.
type Common struct {
	AppEnv   string `koanf:"app_env"`   // 运行环境：development / staging / production
	LogLevel string `koanf:"log_level"` // 日志级别：debug / info / warn / error
}

// Postgres 数据库连接配置。
// Postgres holds PostgreSQL connection settings.
type Postgres struct {
	URL string `koanf:"database_url"` // 连接字符串，如 postgresql://user:pass@host:5432/dbname
}

// Redis 缓存连接配置。
// Redis holds Redis connection settings.
type Redis struct {
	URL string `koanf:"redis_url"` // 连接字符串，如 redis://localhost:6379
}

// JWT 签名配置，管理双令牌体系。
// JWT holds JWT signing configuration for the dual-token system.
type JWT struct {
	AccessSecret     string        `koanf:"jwt_access_secret"`      // Access token 签名密钥 / Access token signing secret
	RefreshSecret    string        `koanf:"jwt_refresh_secret"`     // Refresh token 签名密钥（独立于 access）/ Refresh token signing secret (independent)
	AccessExpiresIn  time.Duration `koanf:"jwt_access_expires_in"`  // Access token 有效期（如 15m）/ Access token TTL (e.g. 15m)
	RefreshExpiresIn time.Duration `koanf:"jwt_refresh_expires_in"` // Refresh token 有效期（如 168h = 7天）/ Refresh token TTL (e.g. 168h = 7 days)
}

// Internal 服务间内部鉴权配置。微服务之间的调用通过共享密钥认证。
// Internal holds service-to-service auth settings. Inter-service calls are authenticated with a shared secret.
type Internal struct {
	Secret string `koanf:"internal_secret"` // 内部通信共享密钥 / Shared secret for internal communication
}

// CORS 跨域配置。浏览器安全策略要求后端显式声明允许哪些域名访问。
// CORS holds cross-origin settings. Browser security policy requires the backend to explicitly declare allowed origins.
type CORS struct {
	Origins []string `koanf:"cors_origins"` // 允许的源列表，如 ["http://localhost:3000"] / Allowed origins list
}

// Load 从环境变量加载配置到 koanf 实例。
// Load reads all environment variables into a koanf instance.
//
// 为什么用 koanf 而不是直接 os.Getenv？koanf 提供类型安全的访问（String/Duration/Int），
// 且禁止在业务代码中散落 os.Getenv 调用，所有配置读取集中在 config 包。
// Why koanf instead of os.Getenv? Koanf provides type-safe accessors (String/Duration/Int),
// and keeps all config reads centralized in the config package instead of scattered os.Getenv calls.
//
// 分隔符用 "." 而不是 "_"，是为了避免 koanf 把 DATABASE_URL 拆成嵌套结构 database.url。
// Separator is "." not "_", to prevent koanf from splitting DATABASE_URL into nested structure database.url.
func Load() (*koanf.Koanf, error) {
	k := koanf.New(".")

	// 前缀 "" 表示不过滤，读取所有环境变量。
	// Prefix "" means no filtering — read all environment variables.
	//
	// 转换函数把 DATABASE_URL → database_url，统一用小写键名。
	// The transform function converts DATABASE_URL → database_url, normalizing to lowercase keys.
	err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.ToLower(s)
	}), nil)
	if err != nil {
		return nil, err
	}

	return k, nil
}
