package auth

import (
	"bytes"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "correct horse battery"); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
	if err := VerifyPassword(hash, "wrong password"); err != ErrMismatch {
		t.Fatalf("expected ErrMismatch, got %v", err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same password")
	b, _ := HashPassword("same password")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestNewTokenHashMatches(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 {
		t.Fatalf("token too short: %d", len(token))
	}
	if !bytes.Equal(HashToken(token), hash) {
		t.Fatal("HashToken must reproduce the stored hash")
	}
}
