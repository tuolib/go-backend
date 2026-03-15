package auth

import (
	"testing"
	"time"
)

func TestJWTManager_SignAndVerifyAccessToken(t *testing.T) {
	m := NewJWTManager("access-secret", "refresh-secret", 15*time.Minute, 7*24*time.Hour)

	token, err := m.SignAccessToken("user123", "test@example.com", "jti-001")
	if err != nil {
		t.Fatalf("SignAccessToken error: %v", err)
	}

	claims, err := m.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken error: %v", err)
	}

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

func TestJWTManager_WrongSecret(t *testing.T) {
	m := NewJWTManager("access-secret", "refresh-secret", 15*time.Minute, 7*24*time.Hour)

	// Sign with access, verify with refresh → should fail
	token, _ := m.SignAccessToken("user1", "a@b.com", "jti-003")
	_, err := m.VerifyRefreshToken(token)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret")
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	// Token expires immediately
	m := NewJWTManager("secret", "secret", -1*time.Second, -1*time.Second)

	token, _ := m.SignAccessToken("user1", "a@b.com", "jti-004")
	_, err := m.VerifyAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWTManager_DefaultDurations(t *testing.T) {
	m := NewJWTManager("a", "b", 0, 0)
	if m.AccessExpiresIn() != 15*time.Minute {
		t.Errorf("default access = %v, want 15m", m.AccessExpiresIn())
	}
	if m.RefreshExpiresIn() != 7*24*time.Hour {
		t.Errorf("default refresh = %v, want 168h", m.RefreshExpiresIn())
	}
}
