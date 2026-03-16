package auth

import "testing"

// TestHashToken 测试 SHA-256 token 哈希的基本属性。
// TestHashToken tests the basic properties of SHA-256 token hashing.
func TestHashToken(t *testing.T) {
	token := "my-refresh-token-123"
	hash := HashToken(token)

	// 哈希值不应等于原始 token / Hash should not equal the raw token
	if hash == token {
		t.Fatal("hash should not equal raw token")
	}
	// SHA-256 输出 32 字节，hex 编码后 64 字符 / SHA-256 outputs 32 bytes, which is 64 hex characters
	if len(hash) != 64 {
		t.Errorf("SHA256 hex length = %d, want 64", len(hash))
	}

	// 确定性：同一输入总是产生相同输出（与 Argon2 不同，因为没有随机盐）。
	// Deterministic: same input always produces the same output (unlike Argon2, because there's no random salt).
	if HashToken(token) != hash {
		t.Fatal("HashToken should be deterministic")
	}

	// 不同输入产生不同输出 / Different inputs produce different outputs
	if HashToken("other-token") == hash {
		t.Fatal("different tokens should produce different hashes")
	}
}
