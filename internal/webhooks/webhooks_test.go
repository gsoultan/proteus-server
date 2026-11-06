// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package webhooks_test

import (
	"strings"
	"testing"

	"github.com/gsoultan/proteus-server/internal/webhooks"
	"github.com/gsoultan/proteus-server/internal/webhooks/storetest"
)

func TestMemoryStore(t *testing.T) { storetest.Run(t, webhooks.NewMemoryStore(0)) }

func TestSealer(t *testing.T) {
	plain, err := webhooks.NewSealer(nil)
	if err != nil || plain.Encrypted() {
		t.Fatal(err)
	}
	enc, err := webhooks.NewSealer([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil || !enc.Encrypted() {
		t.Fatal(err)
	}
	secret := webhooks.NewSecret()
	if !strings.HasPrefix(secret, webhooks.SecretPrefix) || len(secret) != len(webhooks.SecretPrefix)+43 {
		t.Fatalf("secret %q", secret)
	}

	sealed := enc.Seal(secret)
	if strings.Contains(string(sealed), secret) {
		t.Fatal("sealed secret is readable")
	}
	if got, err := enc.Open(sealed); err != nil || got != secret {
		t.Fatalf("Open = %q, %v", got, err)
	}
	// A plain secret stored before a key was set still opens.
	if got, err := enc.Open(plain.Seal(secret)); err != nil || got != secret {
		t.Fatalf("Open plain = %q, %v", got, err)
	}
	// An encrypted secret needs the key, and the same key.
	if _, err := plain.Open(sealed); err == nil {
		t.Fatal("opened without a key")
	}
	other, _ := webhooks.NewSealer([]byte("another key another key another key"))
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("opened with the wrong key")
	}
}

func TestSign(t *testing.T) {
	// Same construction as the per-job webhooks: hex HMAC-SHA256 of "<ts>.<body>".
	got := webhooks.Sign([]byte("key"), "1700000000", []byte(`{"a":1}`))
	if len(got) != 64 || got != webhooks.Sign([]byte("key"), "1700000000", []byte(`{"a":1}`)) ||
		got == webhooks.Sign([]byte("key"), "1700000001", []byte(`{"a":1}`)) {
		t.Fatalf("Sign = %q", got)
	}
}
