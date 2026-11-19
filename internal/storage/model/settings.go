// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package model

import "github.com/gsoultan/storm"

// ServerSetting holds the settings saved from the web UI. There is one row,
// Scope "server"; Body is the JSON settings.Doc (values and setup state).
type ServerSetting struct {
	storm.Model

	Scope    string
	Revision int64
	Body     []byte
}

func (s *ServerSetting) Schema(t *storm.Table) {
	t.Col(&s.Scope).Size(32)
	t.Col(&s.Revision).Default("0")
	t.Unique(&s.Scope)
}

// APIKey is a key created in the web UI. Only its SHA-256 is stored.
type APIKey struct {
	storm.Model

	KeyID  string // owner id of the key's jobs and templates
	Name   string
	Role   string // admin or client
	Hash   []byte
	Prefix string
}

func (k *APIKey) Schema(t *storm.Table) {
	t.Col(&k.KeyID).Size(64)
	t.Col(&k.Name).Size(100)
	t.Col(&k.Role)
	t.Col(&k.Prefix).Size(16)
	t.Check(storm.RawSQL(`role IN ('admin', 'client')`))
	t.Unique(&k.KeyID)
}
