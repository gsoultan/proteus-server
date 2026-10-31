package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := FromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.MaxBodyBytes != 20<<20 || c.AllowRemote || c.Timeout != 30*time.Second {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestParse(t *testing.T) {
	c, err := FromEnv(env(map[string]string{
		"PROTEUS_MAX_BODY_BYTES": "5MiB", "PROTEUS_TIMEOUT": "5s", "PROTEUS_ALLOW_REMOTE": "true",
		"PROTEUS_ALLOWED_HOSTS": "a.com, .b.com", "PROTEUS_WORKERS": "3",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxBodyBytes != 5<<20 || c.Timeout != 5*time.Second || !c.AllowRemote || len(c.AllowedHosts) != 2 || c.Workers != 3 {
		t.Fatalf("parsed: %+v", c)
	}
}

func TestErrors(t *testing.T) {
	for k, v := range map[string]string{
		"PROTEUS_TIMEOUT": "soon", "PROTEUS_WORKERS": "-1", "PROTEUS_MAX_BODY_BYTES": "big",
		"PROTEUS_ALLOW_REMOTE": "maybe", "PROTEUS_WEBHOOK_SECRET": "short",
	} {
		if _, err := FromEnv(env(map[string]string{k: v})); err == nil {
			t.Errorf("%s=%q accepted", k, v)
		}
	}
}
