// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package config

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Fields must list exactly what FromEnv reads, with the same defaults.
func TestFieldsMatchFromEnv(t *testing.T) {
	var read []string
	base, err := FromEnv(func(k string) string { read = append(read, k); return "" })
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range read {
		if k == "PROTEUS_API_KEYS_FILE" {
			continue
		}
		if _, ok := Lookup(k); !ok {
			t.Errorf("FromEnv reads %s but Fields does not list it", k)
		}
	}
	for _, f := range Fields {
		if !slices.Contains(read, f.Name) {
			t.Errorf("Fields lists %s but FromEnv does not read it", f.Name)
		}
		if f.Editable && f.Bootstrap {
			t.Errorf("%s is both editable and bootstrap", f.Name)
		}
		if !slices.Contains(Groups, f.Group) {
			t.Errorf("%s: unknown group %q", f.Name, f.Group)
		}
	}
	defaults := map[string]string{}
	for _, f := range Fields {
		defaults[f.Name] = f.Default
	}
	got, err := FromEnv(env(defaults))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("Field defaults differ from FromEnv defaults:\n%+v\n%+v", got, base)
	}
}

func TestLayered(t *testing.T) {
	get := Layered(env(map[string]string{"A": "env", "E": " "}), map[string]string{"A": "boot", "B": "boot", "E": "boot"}, map[string]string{"B": "saved", "C": "saved"})
	for k, want := range map[string]string{"A": "env", "B": "boot", "C": "saved", "D": "", "E": "boot"} {
		if got := get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestValidateSaved(t *testing.T) {
	clean, err := ValidateSaved(env(nil), nil, map[string]string{"PROTEUS_TIMEOUT": " 5s ", "PROTEUS_ALLOWED_HOSTS": ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 1 || clean["PROTEUS_TIMEOUT"] != "5s" {
		t.Fatalf("clean = %v", clean)
	}
	for name, m := range map[string]map[string]string{
		"unknown":   {"PROTEUS_NOPE": "1"},
		"bootstrap": {"PROTEUS_ADDR": ":1"},
		"bad value": {"PROTEUS_TIMEOUT": "soon"},
		"short":     {"PROTEUS_WEBHOOK_SECRET": "short"},
	} {
		if _, err := ValidateSaved(env(nil), nil, m); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A bad environment value is the operator's problem, not a reason to
	// refuse a good saved value.
	if _, err := ValidateSaved(env(map[string]string{"PROTEUS_WORKERS": "x"}), nil, map[string]string{"PROTEUS_TIMEOUT": "5s"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBootstrap(t *testing.T) {
	if _, err := ValidateBootstrap(map[string]string{"PROTEUS_TIMEOUT": "5s"}); err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("err = %v", err)
	}
	b, err := ValidateBootstrap(map[string]string{"PROTEUS_DATABASE_URL": "postgres://x", "PROTEUS_AUTO_MIGRATE": "false"})
	if err != nil || len(b) != 2 {
		t.Fatal(b, err)
	}
	if _, err := ValidateBootstrap(map[string]string{"PROTEUS_AUTO_MIGRATE": "perhaps"}); err == nil {
		t.Fatal("bad bool accepted")
	}
}
