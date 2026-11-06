// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package webhooks

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// SecretPrefix starts every endpoint signing secret.
const SecretPrefix = "whsec_"

// NewSecret returns a fresh signing secret: the prefix and 32 random bytes
// in unpadded base64url.
func NewSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return SecretPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// Sign returns the hex HMAC-SHA256 of "<ts>.<body>" under secret.
func Sign(secret []byte, ts string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Sealer protects stored secrets. With a key it seals them with
// AES-256-GCM; without one it stores them as they are (and says so in the
// leading byte, so adding a key later still opens the old ones).
type Sealer struct{ aead cipher.AEAD }

const (
	sealPlain byte = 0
	sealGCM   byte = 1
)

// NewSealer derives the AES key from keyMaterial with HKDF-SHA256; empty
// keyMaterial stores secrets unencrypted.
func NewSealer(keyMaterial []byte) (*Sealer, error) {
	if len(keyMaterial) == 0 {
		return &Sealer{}, nil
	}
	key, err := hkdf.Key(sha256.New, keyMaterial, nil, "proteus webhook secrets v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Encrypted reports whether Seal encrypts.
func (s *Sealer) Encrypted() bool { return s.aead != nil }

// Seal returns the stored form of secret.
func (s *Sealer) Seal(secret string) []byte {
	if s.aead == nil {
		return append([]byte{sealPlain}, secret...)
	}
	nonce := make([]byte, s.aead.NonceSize())
	rand.Read(nonce)
	out := append([]byte{sealGCM}, nonce...)
	return s.aead.Seal(out, nonce, []byte(secret), []byte{sealGCM})
}

var errSealed = errors.New("webhook secret is encrypted and PROTEUS_WEBHOOK_KEY is not set or differs")

// Open returns the secret from its stored form.
func (s *Sealer) Open(stored []byte) (string, error) {
	if len(stored) == 0 {
		return "", errors.New("webhook secret missing")
	}
	switch stored[0] {
	case sealPlain:
		return string(stored[1:]), nil
	case sealGCM:
		if s.aead == nil {
			return "", errSealed
		}
		n := s.aead.NonceSize()
		if len(stored) < 1+n {
			return "", errSealed
		}
		plain, err := s.aead.Open(nil, stored[1:1+n], stored[1+n:], []byte{sealGCM})
		if err != nil {
			return "", errSealed
		}
		return string(plain), nil
	}
	return "", errors.New("webhook secret has an unknown format")
}
