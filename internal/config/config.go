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

// Load reads environment variables into a koanf instance.
// All env vars are lowercased and mapped to koanf keys.
// Example: DATABASE_URL -> database_url
func Load() (*koanf.Koanf, error) {
	k := koanf.New(".")

	err := k.Load(env.Provider("", ".", func(s string) string {
		return strings.ToLower(s)
	}), nil)
	if err != nil {
		return nil, err
	}

	return k, nil
}
