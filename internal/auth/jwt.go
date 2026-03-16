package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 通过嵌入 jwt.RegisteredClaims 获得标准字段（Subject/ID/ExpiresAt 等），
// 再加上自定义的 Email 字段。这样解析 token 后可以直接拿到用户邮箱，减少一次数据库查询。
// Claims embeds jwt.RegisteredClaims to get standard fields (Subject/ID/ExpiresAt, etc.),
// plus a custom Email field. This way we get the user's email directly from the token, saving a DB query.
type Claims struct {
	jwt.RegisteredClaims        // 嵌入标准声明 / Embed standard JWT claims
	Email                string `json:"email"` // 自定义字段：用户邮箱 / Custom field: user email
}

// JWTManager 管理双令牌体系：短命的 access token + 长命的 refresh token。
// JWTManager manages a dual-token system: short-lived access token + long-lived refresh token.
//
// 为什么用双令牌？access token 过期快（15分钟），即使泄露影响有限；
// refresh token 过期慢（7天），但只用于换取新 access token，不直接访问资源。
// Why dual tokens? Access token expires quickly (15min), limiting damage if leaked;
// refresh token expires slowly (7 days) but is only used to obtain new access tokens, never to access resources directly.
//
// 两个 token 用不同密钥签名，互相独立——泄露一个不影响另一个。
// The two tokens use different signing keys, so they're independent — leaking one doesn't compromise the other.
//
// 字段小写开头（未导出），外部无法直接读取密钥，保证封装安全。
// Fields start lowercase (unexported), so external code can't read the secrets directly — enforcing encapsulation.
type JWTManager struct {
	accessSecret     []byte        // Access token 签名密钥 / Access token signing key
	refreshSecret    []byte        // Refresh token 签名密钥 / Refresh token signing key
	accessExpiresIn  time.Duration // Access token 有效期 / Access token TTL
	refreshExpiresIn time.Duration // Refresh token 有效期 / Refresh token TTL
}

// NewJWTManager 创建 JWTManager，传入密钥和过期时间。零值 duration 使用默认值。
// NewJWTManager creates a JWTManager with the given secrets and durations. Zero durations use defaults.
func NewJWTManager(accessSecret, refreshSecret string, accessExp, refreshExp time.Duration) *JWTManager {
	// 如果未指定过期时间，使用合理的默认值。
	// If durations are zero, apply sensible defaults.
	if accessExp == 0 {
		accessExp = 15 * time.Minute // 默认 15 分钟 / Default 15 minutes
	}
	if refreshExp == 0 {
		refreshExp = 7 * 24 * time.Hour // 默认 7 天 / Default 7 days
	}

	return &JWTManager{
		accessSecret:     []byte(accessSecret),  // string → []byte：jwt 库要求字节切片 / jwt library requires byte slice
		refreshSecret:    []byte(refreshSecret),
		accessExpiresIn:  accessExp,
		refreshExpiresIn: refreshExp,
	}
}

// SignAccessToken 为指定用户签发 access token。
// SignAccessToken creates a signed access token for the given user.
func (m *JWTManager) SignAccessToken(userID, email, jti string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,                                     // sub: 用户唯一标识 / sub: user unique identifier
			ID:        jti,                                        // jti: token 唯一 ID，用于吊销——把 jti 加入黑名单即可废掉特定 token / jti: token unique ID for revocation — add jti to blacklist to invalidate a specific token
			IssuedAt:  jwt.NewNumericDate(now),                    // iat: 签发时间 / iat: issued at
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiresIn)), // exp: 过期时间 / exp: expiration time
		},
		Email: email,
	}
	// jwt.SigningMethodHS256 = HMAC-SHA256，对称加密签名算法。
	// jwt.SigningMethodHS256 = HMAC-SHA256, a symmetric signing algorithm.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.accessSecret)
}

// SignRefreshToken 用独立密钥签发 refresh token，和 access token 互不影响。
// SignRefreshToken signs a refresh token with a separate key, independent of the access token.
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

// VerifyAccessToken 解析并验证 access token 字符串，返回解码后的 Claims。
// VerifyAccessToken parses and validates an access token string, returning the decoded Claims.
func (m *JWTManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	return m.verify(tokenStr, m.accessSecret)
}

// VerifyRefreshToken 解析并验证 refresh token 字符串。
// VerifyRefreshToken parses and validates a refresh token string.
func (m *JWTManager) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	return m.verify(tokenStr, m.refreshSecret)
}

// AccessExpiresIn 返回 access token 的有效期。
// AccessExpiresIn returns the access token TTL.
func (m *JWTManager) AccessExpiresIn() time.Duration {
	return m.accessExpiresIn
}

// RefreshExpiresIn 返回 refresh token 的有效期。
// RefreshExpiresIn returns the refresh token TTL.
func (m *JWTManager) RefreshExpiresIn() time.Duration {
	return m.refreshExpiresIn
}

// verify 是内部方法（小写开头，未导出），解析 JWT 并验证签名。
// verify is an internal method (lowercase, unexported) that parses a JWT and validates its signature.
func (m *JWTManager) verify(tokenStr string, secret []byte) (*Claims, error) {
	// ParseWithClaims 第二个参数 &Claims{} 是模板，告诉解析器把 payload 反序列化到 Claims 结构体。
	// ParseWithClaims 2nd arg &Claims{} is a template, telling the parser to deserialize the payload into Claims.
	//
	// 第三个参数是 keyFunc，jwt 库在验证签名前调用它来获取密钥。
	// The 3rd arg is a keyFunc — the jwt library calls it to get the signing key before verifying.
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		// 安全检查：必须验证签名算法是 HMAC 系列。
		// Security check: ensure the signing algorithm is from the HMAC family.
		//
		// 防止攻击者将 header 中的 alg 改为 "none" 绕过签名验证（CVE-2015-9235）。
		// Prevents attackers from changing the header's alg to "none" to bypass signature verification (CVE-2015-9235).
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	// 类型断言：将 token.Claims（接口类型）转回具体的 *Claims 类型。
	// Type assertion: convert token.Claims (interface type) back to the concrete *Claims type.
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
