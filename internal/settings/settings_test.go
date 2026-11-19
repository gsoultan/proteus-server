// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gsoultan/proteus-server/internal/settings"
	"github.com/gsoultan/proteus-server/internal/settings/storetest"
)

func TestMemoryStore(t *testing.T) { storetest.Run(t, settings.NewMemoryStore()) }

func TestFileStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	storetest.Run(t, settings.NewFileStore(dir))
	fi, err := os.Stat(filepath.Join(dir, settings.SettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	// A second store over the same file sees the same data.
	keys, err := settings.NewFileStore(dir).ListKeys(t.Context())
	if err != nil || len(keys) != 1 {
		t.Fatalf("reopen: %v %v", keys, err)
	}
}

func TestBootstrap(t *testing.T) {
	dir := t.TempDir()
	if m, err := settings.ReadBootstrap(dir); err != nil || m != nil {
		t.Fatalf("missing file: %v %v", m, err)
	}
	if err := settings.WriteBootstrap(dir, map[string]string{"PROTEUS_DATABASE_URL": "postgres://x"}); err != nil {
		t.Fatal(err)
	}
	m, err := settings.ReadBootstrap(dir)
	if err != nil || m["PROTEUS_DATABASE_URL"] != "postgres://x" {
		t.Fatalf("read back: %v %v", m, err)
	}
}

func TestCleanName(t *testing.T) {
	if n, err := settings.CleanName("  Billing app "); err != nil || n != "Billing app" {
		t.Fatal(n, err)
	}
	for _, bad := range []string{"", "  ", "a<b", string(make([]rune, 101))} {
		if _, err := settings.CleanName(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
