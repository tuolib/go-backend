package auth

import "testing"

// TestArgon2Hasher_HashAndVerify 测试密码哈希和验证的完整流程。
// TestArgon2Hasher_HashAndVerify tests the full flow of hashing and verifying a password.
func TestArgon2Hasher_HashAndVerify(t *testing.T) {
	h := NewArgon2Hasher()
	password := "SecureP@ssw0rd!"

	hash, err := h.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	// 哈希值不应该等于原始密码（基本安全检查）/ Hash should not equal the plaintext (basic safety check)
	if hash == password {
		t.Fatal("hash should not equal plaintext password")
	}

	// 用正确密码验证应该通过 / Verification with the correct password should pass
	match, err := h.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if !match {
		t.Fatal("expected password to match hash")
	}
}

// TestArgon2Hasher_WrongPassword 测试错误密码验证失败。
// TestArgon2Hasher_WrongPassword tests that a wrong password fails verification.
func TestArgon2Hasher_WrongPassword(t *testing.T) {
	h := NewArgon2Hasher()

	hash, _ := h.HashPassword("correct-password")

	match, err := h.VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if match {
		t.Fatal("expected wrong password to not match")
	}
}

// TestArgon2Hasher_UniqueHashes 测试同一密码每次哈希结果不同（因为随机盐）。
// TestArgon2Hasher_UniqueHashes tests that the same password produces different hashes each time (due to random salt).
func TestArgon2Hasher_UniqueHashes(t *testing.T) {
	h := NewArgon2Hasher()
	password := "same-password"

	hash1, _ := h.HashPassword(password)
	hash2, _ := h.HashPassword(password)

	// 两次哈希结果不同，说明使用了不同的随机盐 / Different hashes confirm different random salts were used
	if hash1 == hash2 {
		t.Fatal("same password should produce different hashes (different salts)")
	}
}
