// Package secure provides field encryption, keyed hashing and signed tokens,
// all derived from one deployment secret (DATA_ENCRYPTION_KEY).
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const version byte = 1

// Keys holds the per-purpose keys derived from the deployment secret.
type Keys struct {
	aead    cipher.AEAD
	hashKey []byte
	signKey []byte
}

// New derives independent keys for encryption, hashing and signing from secret.
func New(secret string) (*Keys, error) {
	if len(secret) < 32 {
		return nil, errors.New("secret must be at least 32 characters")
	}
	derive := func(info string) []byte {
		k, err := hkdf.Key(sha256.New, []byte(secret), nil, info, 32)
		if err != nil {
			panic(err) // only fails for invalid lengths, which are constant here
		}
		return k
	}
	block, err := aes.NewCipher(derive("bill-management/field-encryption/v1"))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Keys{
		aead:    aead,
		hashKey: derive("bill-management/lookup-hash/v1"),
		signKey: derive("bill-management/signed-tokens/v1"),
	}, nil
}

// Seal encrypts plaintext with AES-GCM. Output: version || nonce || ciphertext.
func (k *Keys) Seal(plaintext string) []byte {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	out := append([]byte{version}, nonce...)
	return k.aead.Seal(out, nonce, []byte(plaintext), nil)
}

// Open decrypts a value produced by Seal.
func (k *Keys) Open(sealed []byte) (string, error) {
	n := k.aead.NonceSize()
	if len(sealed) < 1+n || sealed[0] != version {
		return "", errors.New("secure: unknown ciphertext format")
	}
	plain, err := k.aead.Open(nil, sealed[1:1+n], sealed[1+n:], nil)
	if err != nil {
		return "", fmt.Errorf("secure: decrypt: %w", err)
	}
	return string(plain), nil
}

// Hash is a keyed, deterministic hash for equality lookups on encrypted values.
func (k *Keys) Hash(value string) []byte {
	m := hmac.New(sha256.New, k.hashKey)
	m.Write([]byte(value))
	return m.Sum(nil)
}

// Sign returns a URL-safe token binding payload to an expiry time.
func (k *Keys) Sign(payload string, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + base64.RawURLEncoding.EncodeToString(k.mac(exp+"|"+payload))
}

// Verify checks a token from Sign against payload and the current time.
func (k *Keys) Verify(token, payload string, now time.Time) bool {
	exp, sig, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || now.Unix() > unix {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, k.mac(exp+"|"+payload)) == 1
}

func (k *Keys) mac(s string) []byte {
	m := hmac.New(sha256.New, k.signKey)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// Mask shows only the last four characters, e.g. "•••• 6789".
func Mask(s string) string {
	r := []rune(s)
	if len(r) <= 4 {
		return "••••"
	}
	return "•••• " + string(r[len(r)-4:])
}
