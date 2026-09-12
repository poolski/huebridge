package registry

import (
	"encoding/json"
	"os"
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

func TestRegistry_HandlesCorruptedNextID(t *testing.T) {
	// Simulate a hand-edited or corrupted registry.json where next_id is too low.
	path := filepath.Join(t.TempDir(), "registry.json")
	tmpDir := filepath.Dir(path)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Write corrupted JSON: next_id is 2, but entries already have HueID 1 and 3.
	corrupted := fileFormat{
		NextID: 2,
		Entries: []Entry{
			{HueID: 1, EntityID: "light.kitchen", Name: "Kitchen"},
			{HueID: 3, EntityID: "light.hall", Name: "Hall"},
		},
	}
	data, _ := json.MarshalIndent(corrupted, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write corrupted registry: %v", err)
	}

	// Load the corrupted registry - it should self-heal.
	r, err := NewRegistry(path)
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}

	// The self-healing should have set NextID to max(3) + 1 = 4.
	new1, _ := r.Add("light.bedroom", "Bedroom")
	if new1.HueID <= 3 {
		t.Fatalf("got HueID=%d, want > 3 to avoid collision with existing entries", new1.HueID)
	}

	// Add another entity and verify it also doesn't collide.
	new2, _ := r.Add("light.garage", "Garage")
	if new2.HueID == 1 || new2.HueID == 3 || new2.HueID == new1.HueID {
		t.Fatalf("got HueID=%d, collides with existing entry or previous new entry", new2.HueID)
	}
}
