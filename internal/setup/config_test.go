package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigComplete(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"zero value", Config{}, false},
		{"missing token", Config{HAURL: "http://ha.local:8123", AdminPasswordHash: []byte("x")}, false},
		{"missing url", Config{HAToken: "tok", AdminPasswordHash: []byte("x")}, false},
		{"missing password hash", Config{HAURL: "http://ha.local:8123", HAToken: "tok"}, false},
		{"complete", Config{HAURL: "http://ha.local:8123", HAToken: "tok", AdminPasswordHash: []byte("x")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Complete(); got != tt.want {
				t.Errorf("Complete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreLoadMissingFileReturnsIncompleteConfig(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "standalone.json"))
	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Complete() {
		t.Fatalf("expected incomplete config for missing file, got %+v", cfg)
	}
}

func TestStoreSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "standalone.json")
	s := NewStore(path)
	want := Config{HAURL: "http://ha.local:8123", HAToken: "tok", AdminPasswordHash: []byte("hash")}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.HAURL != want.HAURL || got.HAToken != want.HAToken || string(got.AdminPasswordHash) != string(want.AdminPasswordHash) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStoreSaveRestrictsFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions don't apply on Windows")
	}

	path := filepath.Join(t.TempDir(), "standalone.json")
	s := NewStore(path)
	cfg := Config{HAURL: "http://ha.local:8123", HAToken: "secret-token", AdminPasswordHash: []byte("hash")}
	if err := s.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want %o", got, 0o600)
	}
}
