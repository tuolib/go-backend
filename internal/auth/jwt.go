package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT token claims.
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// JWTManager handles signing and verifying JWT tokens.
type JWTManager struct {
	accessSecret     []byte
	refreshSecret    []byte
	accessExpiresIn  time.Duration
	refreshExpiresIn time.Duration
}

// NewJWTManager creates a JWTManager with the given secrets and durations.
func NewJWTManager(accessSecret, refreshSecret string, accessExp, refreshExp time.Duration) *JWTManager {
	// Apply defaults if durations are zero
	if accessExp == 0 {
		accessExp = 15 * time.Minute
	}
	if refreshExp == 0 {
		refreshExp = 7 * 24 * time.Hour
	}

	return &JWTManager{
		accessSecret:     []byte(accessSecret),
		refreshSecret:    []byte(refreshSecret),
		accessExpiresIn:  accessExp,
		refreshExpiresIn: refreshExp,
	}
}

// SignAccessToken creates a signed access token for the given user.
func (m *JWTManager) SignAccessToken(userID, email, jti string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiresIn)),
		},
		Email: email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.accessSecret)
}

// SignRefreshToken creates a signed refresh token for the given user.
func (m *JWTManager) SignRefreshToken(userID, email, jti string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiresIn)),
		},
		Email: email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.refreshSecret)
}

// VerifyAccessToken parses and validates an access token string.
func (m *JWTManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	return m.verify(tokenStr, m.accessSecret)
}

// VerifyRefreshToken parses and validates a refresh token string.
func (m *JWTManager) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	return m.verify(tokenStr, m.refreshSecret)
}

// AccessExpiresIn returns the access token TTL.
func (m *JWTManager) AccessExpiresIn() time.Duration {
	return m.accessExpiresIn
}

// RefreshExpiresIn returns the refresh token TTL.
func (m *JWTManager) RefreshExpiresIn() time.Duration {
	return m.refreshExpiresIn
}

func (m *JWTManager) verify(tokenStr string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
