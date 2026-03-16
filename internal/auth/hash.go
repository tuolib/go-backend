package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken 对 refresh token 做 SHA-256 哈希后再存数据库。
// HashToken applies SHA-256 to a refresh token before storing it in the database.
//
// 为什么不存原始 token？如果数据库泄露，攻击者拿到的是哈希值，无法还原出原始 token。
// Why not store the raw token? If the database leaks, attackers only get the hash — they can't reverse it to the original token.
//
// 为什么用 SHA-256 而不是 Argon2？refresh token 本身是高熵随机字符串，不需要防暴力破解，
// SHA-256 速度快且不可逆，足够安全。密码才需要慢哈希（Argon2）因为用户密码熵低。
// Why SHA-256 instead of Argon2? Refresh tokens are high-entropy random strings — no need for brute-force resistance.
// SHA-256 is fast and irreversible, which is sufficient. Passwords need slow hashes (Argon2) because user passwords have low entropy.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token)) // 返回 [32]byte 固定长度数组 / Returns a [32]byte fixed-size array
	return hex.EncodeToString(h[:])   // h[:] 将数组转为切片，hex 编码后变成 64 字符的十六进制字符串 / h[:] converts array to slice; hex encoding produces a 64-char hex string
}
