package auth

import (
	"testing"
	"time"
)

// TestJWTManager_SignAndVerifyAccessToken 测试签发和验证 access token 的完整流程。
// TestJWTManager_SignAndVerifyAccessToken tests the full flow of signing and verifying an access token.
func TestJWTManager_SignAndVerifyAccessToken(t *testing.T) {
	m := NewJWTManager("access-secret", "refresh-secret", 15*time.Minute, 7*24*time.Hour)

	token, err := m.SignAccessToken("user123", "test@example.com", "jti-001")
	if err != nil {
		t.Fatalf("SignAccessToken error: %v", err)
	}

	// 验证 token 并提取 claims / Verify token and extract claims
	claims, err := m.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken error: %v", err)
	}

	// 检查 claims 中的各字段是否与签发时一致 / Verify claims fields match what was signed
	if claims.Subject != "user123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user123")
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "test@example.com")
	}
	if claims.ID != "jti-001" {
		t.Errorf("ID = %q, want %q", claims.ID, "jti-001")
	}
}

// TestJWTManager_SignAndVerifyRefreshToken 测试 refresh token 的签发和验证。
// TestJWTManager_SignAndVerifyRefreshToken tests signing and verifying a refresh token.
func TestJWTManager_SignAndVerifyRefreshToken(t *testing.T) {
	m := NewJWTManager("access-secret", "refresh-secret", 15*time.Minute, 7*24*time.Hour)

	token, err := m.SignRefreshToken("user456", "user@test.com", "jti-002")
	if err != nil {
		t.Fatalf("SignRefreshToken error: %v", err)
	}

	claims, err := m.VerifyRefreshToken(token)
	if err != nil {
		t.Fatalf("VerifyRefreshToken error: %v", err)
	}

	if claims.Subject != "user456" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user456")
	}
}

// TestJWTManager_WrongSecret 测试用错误密钥验证 token 会失败。
// TestJWTManager_WrongSecret tests that verifying a token with the wrong key fails.
//
// 用 access 密钥签发的 token，用 refresh 密钥验证必须报错。
// A token signed with the access key must fail verification with the refresh key.
func TestJWTManager_WrongSecret(t *testing.T) {
	m := NewJWTManager("access-secret", "refresh-secret", 15*time.Minute, 7*24*time.Hour)

	// 用 access 密钥签发，尝试用 refresh 密钥验证 / Sign with access key, try to verify with refresh key
	token, _ := m.SignAccessToken("user1", "a@b.com", "jti-003")
	_, err := m.VerifyRefreshToken(token)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret")
	}
}

// TestJWTManager_ExpiredToken 测试过期 token 验证会失败。
// TestJWTManager_ExpiredToken tests that an expired token fails verification.
func TestJWTManager_ExpiredToken(t *testing.T) {
	// 负数过期时间让 token 立刻过期 / Negative duration makes the token expire immediately
	m := NewJWTManager("secret", "secret", -1*time.Second, -1*time.Second)

	token, _ := m.SignAccessToken("user1", "a@b.com", "jti-004")
	_, err := m.VerifyAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

// TestJWTManager_DefaultDurations 测试零值 duration 时使用默认值。
// TestJWTManager_DefaultDurations tests that zero durations fall back to defaults.
func TestJWTManager_DefaultDurations(t *testing.T) {
	m := NewJWTManager("a", "b", 0, 0) // 传入 0，应使用默认值 / Pass 0, should use defaults
	if m.AccessExpiresIn() != 15*time.Minute {
		t.Errorf("default access = %v, want 15m", m.AccessExpiresIn())
	}
	if m.RefreshExpiresIn() != 7*24*time.Hour {
		t.Errorf("default refresh = %v, want 168h", m.RefreshExpiresIn())
	}
}
