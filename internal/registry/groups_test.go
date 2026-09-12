package registry

import (
	"path/filepath"
	"testing"
)

func TestRegistry_AddGroupAssignsStableID(t *testing.T) {
	r, _ := NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	r.Add("light.kitchen", "Kitchen")
	r.Add("light.hall", "Hall")

	g, err := r.AddGroup("Downstairs", "Living room", []string{"light.kitchen", "light.hall"})
	if err != nil {
		t.Fatalf("AddGroup() error: %v", err)
	}
	if g.HueID != 1 {
		t.Fatalf("got HueID=%d, want 1", g.HueID)
	}
	if len(g.EntityIDs) != 2 {
		t.Fatalf("got %d entity ids, want 2", len(g.EntityIDs))
	}
}

func TestRegistry_GroupIDsDoNotCollideWithLightIDs(t *testing.T) {
	// Groups and lights are numbered in independent spaces (both start at 1),
	// matching the real bridge where /lights/1 and /groups/1 coexist.
	r, _ := NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	light, _ := r.Add("light.kitchen", "Kitchen")
	group, _ := r.AddGroup("Downstairs", "Living room", []string{"light.kitchen"})

	if light.HueID != 1 || group.HueID != 1 {
		t.Fatalf("got light.HueID=%d group.HueID=%d, want both to start at 1 independently", light.HueID, group.HueID)
	}
}
