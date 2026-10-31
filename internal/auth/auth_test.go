package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestKeys(t *testing.T) {
	key := NewKey()
	if !strings.HasPrefix(key, "pk_") || len(key) < 40 {
		t.Fatalf("weak key %q", key)
	}
	keys, err := ParseKeys("# comment\nci:" + HashKey(key) + ", other:" + HashKey("x"))
	if err != nil || len(keys) != 2 {
		t.Fatalf("ParseKeys: %v %v", keys, err)
	}
	a := New(keys, 0, 0)
	if id, err := a.Check(key); err != nil || id != "ci" {
		t.Fatalf("Check(valid) = %q, %v", id, err)
	}
	for _, bad := range []string{"", "nope", HashKey(key)} {
		if _, err := a.Check(bad); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("Check(%q) = %v, want ErrUnauthorized", bad, err)
		}
	}
}

func TestParseKeysErrors(t *testing.T) {
	for _, spec := range []string{"nohash", "id:zz", "id:abcd", "a:" + HashKey("1") + ",a:" + HashKey("2")} {
		if _, err := ParseKeys(spec); err == nil {
			t.Errorf("ParseKeys(%q) accepted", spec)
		}
	}
	_, err := ParseKeys("secretid:plaintextkey")
	if err == nil || strings.Contains(err.Error(), "plaintextkey") {
		t.Errorf("error leaks the value: %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	keys, _ := ParseKeys("a:" + HashKey("ka") + ",b:" + HashKey("kb"))
	a := New(keys, 0.001, 2)
	for range 2 {
		if _, err := a.Check("ka"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Check("ka"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("third call = %v, want ErrRateLimited", err)
	}
	if _, err := a.Check("kb"); err != nil {
		t.Fatalf("other key limited: %v", err)
	}
}

func TestDisabled(t *testing.T) {
	a := New(nil, 0, 0)
	if id, err := a.Check(""); err != nil || id != Anonymous {
		t.Fatalf("Check = %q, %v", id, err)
	}
}
