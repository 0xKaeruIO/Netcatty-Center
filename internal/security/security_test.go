package security

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	stored, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "scrypt$") {
		t.Fatalf("unexpected hash format: %s", stored)
	}
	if !VerifyPassword("correct-horse", stored) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong-battery", stored) {
		t.Fatal("expected wrong password to fail")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()
	if !ValidAPIKeyFormat(key.Plaintext) {
		t.Fatalf("invalid key: %s", key.Plaintext)
	}
	if !strings.HasPrefix(key.Plaintext, "ncc_") {
		t.Fatal("missing ncc_ prefix")
	}
	if key.Prefix != key.Plaintext[:12] {
		t.Fatal("prefix mismatch")
	}
	if key.Hash != SHA256Hex(key.Plaintext) {
		t.Fatal("hash should be sha256 of plaintext")
	}
	if key.Hash == key.Plaintext {
		t.Fatal("plaintext must not be stored as hash")
	}
}
