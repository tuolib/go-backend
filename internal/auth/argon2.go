package auth

import "github.com/alexedwards/argon2id"

// Argon2Hasher 用 Argon2id 算法做密码哈希。
// Argon2Hasher hashes passwords using the Argon2id algorithm.
//
// 为什么选 Argon2id 而不是 bcrypt？Argon2id 是 2015 年密码哈希竞赛的冠军，
// 同时抵抗 GPU 攻击（高内存消耗）和侧信道攻击，是目前最推荐的密码哈希算法。
// Why Argon2id over bcrypt? Argon2id won the 2015 Password Hashing Competition,
// resisting both GPU attacks (high memory usage) and side-channel attacks — currently the most recommended algorithm.
type Argon2Hasher struct {
	params *argon2id.Params // 哈希参数（内存、迭代次数、并行度）/ Hash parameters (memory, iterations, parallelism)
}

// NewArgon2Hasher 使用库的默认参数（内存 64MB、3 次迭代、2 线程）。
// NewArgon2Hasher uses the library's default parameters (64MB memory, 3 iterations, 2 threads).
//
// 每次哈希会自动生成随机盐，所以同一密码每次哈希结果都不同。
// Each hash automatically generates a random salt, so the same password produces a different hash every time.
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{
		params: argon2id.DefaultParams,
	}
}

// HashPassword 对密码做 Argon2id 哈希，返回包含盐和参数的完整哈希字符串。
// HashPassword creates an Argon2id hash of the password, returning a full hash string with embedded salt and parameters.
func (h *Argon2Hasher) HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, h.params)
}

// VerifyPassword 验证密码是否与已存储的哈希匹配。内部自动从哈希字符串中提取盐和参数。
// VerifyPassword checks whether the password matches the stored hash. It auto-extracts salt and params from the hash string.
func (h *Argon2Hasher) VerifyPassword(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}
