// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// FileStore keeps settings and keys in one JSON file (mode 0600) in the data
// directory, for servers without a database. It is for a single instance.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// SettingsFile is the FileStore's file name in the data directory.
const SettingsFile = "settings.json"

// NewFileStore uses dir/settings.json; dir is created on first save.
func NewFileStore(dir string) *FileStore {
	return &FileStore{path: filepath.Join(dir, SettingsFile)}
}

type fileKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Hash      []byte    `json:"sha256"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

type fileDoc struct {
	Settings Doc       `json:"settings"`
	Keys     []fileKey `json:"keys,omitempty"`
}

func (s *FileStore) read() (fileDoc, error) {
	var d fileDoc
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, fmt.Errorf("settings: %w", err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return d, fmt.Errorf("settings: %s: %w", s.path, err)
	}
	return d, nil
}

func (s *FileStore) write(d fileDoc) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(s.path, b)
}

// writeFile replaces path atomically with a 0600 file.
func writeFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("settings: data directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("settings: data directory not writable: %w", err)
	}
	tmp := f.Name()
	_, werr := f.Write(append(b, '\n'))
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("settings: write: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("settings: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("settings: write: %w", err)
	}
	return nil
}

// Load implements Store.
func (s *FileStore) Load(context.Context) (Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	return d.Settings, err
}

// Save implements Store.
func (s *FileStore) Save(_ context.Context, doc Doc, expected int64) (Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return Doc{}, err
	}
	if d.Settings.Revision != expected {
		return Doc{}, ErrRevisionMismatch
	}
	doc.Revision = expected + 1
	d.Settings = doc
	if err := s.write(d); err != nil {
		return Doc{}, err
	}
	return doc, nil
}

// ListKeys implements Store.
func (s *FileStore) ListKeys(context.Context) ([]APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return nil, err
	}
	out := make([]APIKey, 0, len(d.Keys))
	for _, k := range d.Keys {
		if len(k.Hash) != 32 {
			return nil, fmt.Errorf("settings: key %q: bad hash", k.ID)
		}
		a := APIKey{ID: k.ID, Name: k.Name, Role: k.Role, Prefix: k.Prefix, CreatedAt: k.CreatedAt}
		copy(a.Hash[:], k.Hash)
		out = append(out, a)
	}
	return out, nil
}

// CreateKey implements Store.
func (s *FileStore) CreateKey(_ context.Context, k APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(d.Keys, func(f fileKey) bool { return f.ID == k.ID }) {
		return ErrKeyExists
	}
	d.Keys = append(d.Keys, fileKey{ID: k.ID, Name: k.Name, Role: k.Role, Hash: k.Hash[:], Prefix: k.Prefix, CreatedAt: k.CreatedAt})
	return s.write(d)
}

// DeleteKey implements Store.
func (s *FileStore) DeleteKey(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(d.Keys, func(f fileKey) bool { return f.ID == id })
	if i < 0 {
		return ErrNotFound
	}
	d.Keys = slices.Delete(d.Keys, i, i+1)
	return s.write(d)
}

var _ Store = (*FileStore)(nil)

// BootstrapFile holds the database settings chosen in the /setup wizard.
const BootstrapFile = "bootstrap.json"

// ReadBootstrap returns the PROTEUS_* values in dir/bootstrap.json, or nil
// when there is none.
func ReadBootstrap(dir string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, BootstrapFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("settings: %s: %w", BootstrapFile, err)
	}
	return m, nil
}

// WriteBootstrap replaces dir/bootstrap.json (mode 0600: it can hold a
// database password).
func WriteBootstrap(dir string, m map[string]string) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, BootstrapFile), b)
}
