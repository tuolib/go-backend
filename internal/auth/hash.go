package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken returns the SHA-256 hex digest of a token string.
// Used for storing refresh tokens in the database (never store raw tokens).
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
