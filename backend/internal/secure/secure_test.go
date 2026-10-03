package secure

import (
	"bytes"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestSealOpen(t *testing.T) {
	k, err := New(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	a, b := k.Seal("04 11234 5678901"), k.Seal("04 11234 5678901")
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same value must differ (random nonce)")
	}
	got, err := k.Open(a)
	if err != nil || got != "04 11234 5678901" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	a[len(a)-1] ^= 1
	if _, err := k.Open(a); err == nil {
		t.Fatal("tampered ciphertext must not open")
	}
	other, _ := New("another-secret-another-secret-xx")
	if _, err := other.Open(b); err == nil {
		t.Fatal("a different key must not open the value")
	}
}

func TestHashIsDeterministicAndKeyed(t *testing.T) {
	k, _ := New(testSecret)
	other, _ := New("another-secret-another-secret-xx")
	if !bytes.Equal(k.Hash("x"), k.Hash("x")) {
		t.Fatal("hash must be deterministic")
	}
	if bytes.Equal(k.Hash("x"), other.Hash("x")) {
		t.Fatal("hash must depend on the key")
	}
}

func TestSignVerify(t *testing.T) {
	k, _ := New(testSecret)
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	tok := k.Sign("PUT|key", now.Add(5*time.Minute))
	if !k.Verify(tok, "PUT|key", now) {
		t.Fatal("valid token rejected")
	}
	if k.Verify(tok, "GET|key", now) {
		t.Fatal("token accepted for a different payload")
	}
	if k.Verify(tok, "PUT|key", now.Add(6*time.Minute)) {
		t.Fatal("expired token accepted")
	}
	if k.Verify("garbage", "PUT|key", now) {
		t.Fatal("garbage accepted")
	}
}

func TestShortSecretRejected(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"04112345678901": "•••• 8901", "123": "••••", "": "••••"} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
