package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 通过嵌入 jwt.RegisteredClaims 获得标准字段（Subject/ID/ExpiresAt 等），
// 再加上自定义的 Email 字段。这样解析 token 后可以直接拿到用户邮箱，减少一次数据库查询。
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// JWTManager 管理双令牌体系：短命的 access token + 长命的 refresh token。
// 为什么用双令牌？access token 过期快（15分钟），即使泄露影响有限；
// refresh token 过期慢（7天），但只用于换取新 access token，不直接访问资源。
// 两个 token 用不同密钥签名，互相独立——泄露一个不影响另一个。
// 字段小写开头（未导出），外部无法直接读取密钥，保证封装安全。
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
			Subject:   userID,                                     // sub: 用户唯一标识
			ID:        jti,                                        // jti: token 唯一 ID，用于吊销——把 jti 加入黑名单即可废掉特定 token
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiresIn)),
		},
		Email: email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.accessSecret)
}

// SignRefreshToken 用独立密钥签发 refresh token，和 access token 互不影响。
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
		// 安全检查：必须验证签名算法是 HMAC 系列。
		// 防止攻击者将 header 中的 alg 改为 "none" 绕过签名验证（CVE-2015-9235）。
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
