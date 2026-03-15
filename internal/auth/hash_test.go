package auth

import "testing"

func TestHashToken(t *testing.T) {
	token := "my-refresh-token-123"
	hash := HashToken(token)

	if hash == token {
		t.Fatal("hash should not equal raw token")
	}
	if len(hash) != 64 {
		t.Errorf("SHA256 hex length = %d, want 64", len(hash))
	}

	// Same input → same output (deterministic)
	if HashToken(token) != hash {
		t.Fatal("HashToken should be deterministic")
	}

	// Different input → different output
	if HashToken("other-token") == hash {
		t.Fatal("different tokens should produce different hashes")
	}
}
