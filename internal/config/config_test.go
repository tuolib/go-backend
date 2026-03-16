package config

import (
	"os"
	"testing"
)

// TestLoad 测试 koanf 能正确从环境变量加载配置。
// TestLoad tests that koanf correctly loads configuration from environment variables.
func TestLoad(t *testing.T) {
	// 设置测试用的环境变量 / Set environment variables for testing
	os.Setenv("DATABASE_URL", "postgresql://localhost/test")
	os.Setenv("APP_ENV", "testing")
	// defer 确保测试结束后清理环境变量，不影响其他测试。
	// defer ensures environment variables are cleaned up after the test, so they don't affect other tests.
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("APP_ENV")

	k, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// 验证 Load 把 DATABASE_URL 转成了小写 database_url / Verify Load converted DATABASE_URL to lowercase database_url
	if got := k.String("database_url"); got != "postgresql://localhost/test" {
		t.Errorf("database_url = %q, want %q", got, "postgresql://localhost/test")
	}
	if got := k.String("app_env"); got != "testing" {
		t.Errorf("app_env = %q, want %q", got, "testing")
	}
}

// TestLoadUser 测试用户服务配置的完整加载流程。
// TestLoadUser tests the complete loading flow for user service configuration.
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

	// 验证各字段正确映射 / Verify each field is correctly mapped
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

// TestLoadUser_DefaultPort 测试未设置端口环境变量时使用默认值。
// TestLoadUser_DefaultPort tests that the default port is used when the env var is unset.
func TestLoadUser_DefaultPort(t *testing.T) {
	os.Unsetenv("USER_SERVICE_PORT")

	k, _ := Load()
	cfg := LoadUser(k)

	if cfg.Port != "3001" {
		t.Errorf("default port = %q, want %q", cfg.Port, "3001")
	}
}
