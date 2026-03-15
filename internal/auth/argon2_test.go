package auth

import "testing"

func TestArgon2Hasher_HashAndVerify(t *testing.T) {
	h := NewArgon2Hasher()
	password := "SecureP@ssw0rd!"

	hash, err := h.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	if hash == password {
		t.Fatal("hash should not equal plaintext password")
	}

	match, err := h.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if !match {
		t.Fatal("expected password to match hash")
	}
}

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

func TestArgon2Hasher_UniqueHashes(t *testing.T) {
	h := NewArgon2Hasher()
	password := "same-password"

	hash1, _ := h.HashPassword(password)
	hash2, _ := h.HashPassword(password)

	if hash1 == hash2 {
		t.Fatal("same password should produce different hashes (different salts)")
	}
}
