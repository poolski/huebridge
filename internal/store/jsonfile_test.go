package store

import (
	"path/filepath"
	"testing"
)

type sample struct {
	Count int `json:"count"`
}

func TestJSONFile_LoadMissingReturnsDefault(t *testing.T) {
	f := NewJSONFile[sample](filepath.Join(t.TempDir(), "missing.json"))
	got, err := f.Load(sample{Count: 42})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Count != 42 {
		t.Fatalf("got %+v, want default Count=42", got)
	}
}

func TestJSONFile_SaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	f := NewJSONFile[sample](path)

	if err := f.Save(sample{Count: 7}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := f.Load(sample{Count: 0})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Count != 7 {
		t.Fatalf("got %+v, want Count=7", got)
	}
}
