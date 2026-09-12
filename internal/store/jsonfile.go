// Package store provides simple persisted-JSON-file storage, used for the
// entity registry, scene/schedule stores, and whitelist — no database
// needed for the volumes of data a single-household bridge deals with.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type JSONFile[T any] struct {
	path string
	mu   sync.Mutex
}

func NewJSONFile[T any](path string) *JSONFile[T] {
	return &JSONFile[T]{path: path}
}

// Load reads and decodes the file, returning defaultValue (without error)
// if the file doesn't exist yet.
func (f *JSONFile[T]) Load(defaultValue T) (T, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return defaultValue, nil
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("read %s: %w", f.path, err)
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		var zero T
		return zero, fmt.Errorf("decode %s: %w", f.path, err)
	}
	return v, nil
}

// Save writes v to the file, creating parent directories as needed, and
// replacing the previous contents atomically.
func (f *JSONFile[T]) Save(v T) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", f.path, err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", f.path, err)
	}

	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	return os.Rename(tmp, f.path)
}
