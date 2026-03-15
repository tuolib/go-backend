package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken 对 refresh token 做 SHA-256 哈希后再存数据库。
// 为什么不存原始 token？如果数据库泄露，攻击者拿到的是哈希值，无法还原出原始 token。
// 为什么用 SHA-256 而不是 Argon2？refresh token 本身是高熵随机字符串，不需要防暴力破解，
// SHA-256 速度快且不可逆，足够安全。密码才需要慢哈希（Argon2）因为用户密码熵低。
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
