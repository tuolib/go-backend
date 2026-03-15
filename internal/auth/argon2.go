package auth

import "github.com/alexedwards/argon2id"

// Argon2Hasher 用 Argon2id 算法做密码哈希。
// 为什么选 Argon2id 而不是 bcrypt？Argon2id 是 2015 年密码哈希竞赛的冠军，
// 同时抵抗 GPU 攻击（高内存消耗）和侧信道攻击，是目前最推荐的密码哈希算法。
type Argon2Hasher struct {
	params *argon2id.Params
}

// NewArgon2Hasher 使用库的默认参数（内存 64MB、3 次迭代、2 线程）。
// 每次哈希会自动生成随机盐，所以同一密码每次哈希结果都不同。
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{
		params: argon2id.DefaultParams,
	}
}

// HashPassword creates an Argon2id hash of the password.
func (h *Argon2Hasher) HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, h.params)
}

// VerifyPassword checks whether the password matches the hash.
func (h *Argon2Hasher) VerifyPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}
