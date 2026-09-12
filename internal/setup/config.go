// Package setup handles standalone huebridge's own configuration: the Home
// Assistant URL and long-lived access token, and the admin password
// gating the setup wizard and entity-picker UI, all normally supplied by
// Supervisor when running as an add-on.
package setup

import (
	"fmt"
	"os"

	"huebridge/internal/store"
)

// Config is standalone huebridge's persisted configuration.
type Config struct {
	HAURL             string `json:"ha_url"`
	HAToken           string `json:"ha_token"`
	AdminPasswordHash []byte `json:"admin_password_hash"`
}

// Complete reports whether every field the setup wizard collects has been
// filled in, i.e. whether the bridge can start without running the wizard.
func (c Config) Complete() bool {
	return c.HAURL != "" && c.HAToken != "" && len(c.AdminPasswordHash) > 0
}

// Store persists Config as a single JSON file.
type Store struct {
	file *store.JSONFile[Config]
	path string
}

// NewStore builds a Store backed by the JSON file at path.
func NewStore(path string) *Store {
	return &Store{file: store.NewJSONFile[Config](path), path: path}
}

// Load reads the config, returning a zero (incomplete) Config if the file
// doesn't exist yet — the normal state before the setup wizard has run.
func (s *Store) Load() (Config, error) {
	return s.file.Load(Config{})
}

// Save persists cfg, creating the parent directory if needed. Unlike
// store.JSONFile's other consumers (registry, scenes, schedules,
// whitelist), Config holds secrets — the HA long-lived access token and
// the admin bcrypt hash — so the file is chmod'd to 0o600 after writing,
// scoped to this package rather than changing store.JSONFile's default
// permissions for everyone.
func (s *Store) Save(cfg Config) error {
	if err := s.file.Save(cfg); err != nil {
		return err
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("restrict permissions on %s: %w", s.path, err)
	}
	return nil
}
