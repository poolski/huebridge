package registry

import (
	"path/filepath"
	"testing"
)

func TestRegistry_AddAssignsStableID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, err := NewRegistry(path)
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	e1, err := r.Add("light.kitchen", "Kitchen")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if e1.HueID != 1 {
		t.Fatalf("got HueID=%d, want 1", e1.HueID)
	}

	e2, err := r.Add("light.hall", "Hall")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if e2.HueID != 2 {
		t.Fatalf("got HueID=%d, want 2", e2.HueID)
	}
}

func TestRegistry_IDsSurviveReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := NewRegistry(path)
	added, _ := r.Add("light.kitchen", "Kitchen")

	reloaded, err := NewRegistry(path)
	if err != nil {
		t.Fatalf("NewRegistry() reload error: %v", err)
	}

	got, ok := reloaded.ByEntityID("light.kitchen")
	if !ok {
		t.Fatal("expected light.kitchen to be present after reload")
	}
	if got.HueID != added.HueID {
		t.Fatalf("got HueID=%d after reload, want %d (must stay stable)", got.HueID, added.HueID)
	}

	// A newly added entity must not reuse an existing id.
	next, _ := reloaded.Add("light.hall", "Hall")
	if next.HueID == added.HueID {
		t.Fatalf("new entity reused existing HueID %d", next.HueID)
	}
}

func TestRegistry_RemoveThenAllExcludesEntity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := NewRegistry(path)
	r.Add("light.kitchen", "Kitchen")

	if err := r.Remove("light.kitchen"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if len(r.All()) != 0 {
		t.Fatalf("got %d entries after Remove, want 0", len(r.All()))
	}
}
