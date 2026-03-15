package config

import (
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// Common holds fields shared by all services.
type Common struct {
	AppEnv   string `koanf:"app_env"`
	LogLevel string `koanf:"log_level"`
}

// Postgres holds PostgreSQL connection settings.
type Postgres struct {
	URL string `koanf:"database_url"`
}

// Redis holds Redis connection settings.
type Redis struct {
	URL string `koanf:"redis_url"`
}

// JWT holds JWT signing configuration.
type JWT struct {
	AccessSecret     string        `koanf:"jwt_access_secret"`
	RefreshSecret    string        `koanf:"jwt_refresh_secret"`
	AccessExpiresIn  time.Duration `koanf:"jwt_access_expires_in"`
	RefreshExpiresIn time.Duration `koanf:"jwt_refresh_expires_in"`
}

// Internal holds service-to-service auth settings.
type Internal struct {
	Secret string `koanf:"internal_secret"`
}

// CORS holds cross-origin settings.
type CORS struct {
	Origins []string `koanf:"cors_origins"`
}

// Load 从环境变量加载配置到 koanf。
// 为什么用 koanf 而不是直接 os.Getenv？koanf 提供类型安全的访问（String/Duration/Int），
// 且禁止在业务代码中散落 os.Getenv 调用，所有配置读取集中在 config 包。
// 分隔符用 "." 而不是 "_"，是为了避免 koanf 把 DATABASE_URL 拆成嵌套结构 database.url。
func Load() (*koanf.Koanf, error) {
	k := koanf.New(".")

	// 前缀 "" 表示不过滤，读取所有环境变量。
	// 分隔符 "." 表示用点号分隔嵌套键（但我们用扁平键所以实际不触发）。
	// 转换函数把 DATABASE_URL → database_url，统一用小写键名。
	err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.ToLower(s)
	}), nil)
	if err != nil {
		return nil, err
	}

	return k, nil
}
