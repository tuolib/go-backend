package auth

import "github.com/alexedwards/argon2id"

// Argon2Hasher handles password hashing with Argon2id.
type Argon2Hasher struct {
	params *argon2id.Params
}

// NewArgon2Hasher creates an Argon2Hasher with default parameters.
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
