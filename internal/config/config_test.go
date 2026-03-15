package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgresql://localhost/test")
	os.Setenv("APP_ENV", "testing")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("APP_ENV")

	k, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if got := k.String("database_url"); got != "postgresql://localhost/test" {
		t.Errorf("database_url = %q, want %q", got, "postgresql://localhost/test")
	}
	if got := k.String("app_env"); got != "testing" {
		t.Errorf("app_env = %q, want %q", got, "testing")
	}
}

func TestLoadUser(t *testing.T) {
	os.Setenv("USER_SERVICE_PORT", "4001")
	os.Setenv("DATABASE_URL", "postgresql://localhost/test")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("JWT_ACCESS_SECRET", "test-access")
	defer func() {
		os.Unsetenv("USER_SERVICE_PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("JWT_ACCESS_SECRET")
	}()

	k, _ := Load()
	cfg := LoadUser(k)

	if cfg.Port != "4001" {
		t.Errorf("Port = %q, want %q", cfg.Port, "4001")
	}
	if cfg.Postgres.URL != "postgresql://localhost/test" {
		t.Errorf("Postgres.URL = %q", cfg.Postgres.URL)
	}
	if cfg.Redis.URL != "redis://localhost:6379" {
		t.Errorf("Redis.URL = %q", cfg.Redis.URL)
	}
	if cfg.JWT.AccessSecret != "test-access" {
		t.Errorf("JWT.AccessSecret = %q", cfg.JWT.AccessSecret)
	}
}

func TestLoadUser_DefaultPort(t *testing.T) {
	os.Unsetenv("USER_SERVICE_PORT")

	k, _ := Load()
	cfg := LoadUser(k)

	if cfg.Port != "3001" {
		t.Errorf("default port = %q, want %q", cfg.Port, "3001")
	}
}
