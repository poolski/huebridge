package registry

import (
	"encoding/json"
	"os"
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

func TestRegistry_HandlesCorruptedNextGroupID(t *testing.T) {
	// Simulate a hand-edited or corrupted registry.json where next_group_id
	// is too low relative to existing groups' HueIDs.
	path := filepath.Join(t.TempDir(), "registry.json")

	corrupted := fileFormat{
		NextID:      1,
		NextGroupID: 2,
		Groups: []Group{
			{HueID: 1, Name: "Downstairs", Class: "Living room", EntityIDs: []string{"light.kitchen"}},
			{HueID: 3, Name: "Upstairs", Class: "Bedroom", EntityIDs: []string{"light.hall"}},
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

	// The self-healing should have set NextGroupID to max(3) + 1 = 4.
	g, err := r.AddGroup("Garage", "Garage", []string{"light.garage"})
	if err != nil {
		t.Fatalf("AddGroup() error: %v", err)
	}
	if g.HueID == 1 || g.HueID == 3 {
		t.Fatalf("got HueID=%d, collides with existing group", g.HueID)
	}
	if g.HueID <= 3 {
		t.Fatalf("got HueID=%d, want > 3 to avoid collision with existing groups", g.HueID)
	}
}

func TestRegistry_RemoveStripsEntityFromGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := NewRegistry(path)
	r.Add("light.kitchen", "Kitchen")
	r.Add("light.hall", "Hall")
	r.AddGroup("Downstairs", "Living room", []string{"light.kitchen", "light.hall"})

	if err := r.Remove("light.kitchen"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	groups := r.AllGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].EntityIDs) != 1 || groups[0].EntityIDs[0] != "light.hall" {
		t.Fatalf("got members %v, want [light.hall] after removing light.kitchen", groups[0].EntityIDs)
	}

	// The stripped membership must be persisted, not just in memory.
	reloaded, err := NewRegistry(path)
	if err != nil {
		t.Fatalf("NewRegistry() reload error: %v", err)
	}
	g, ok := reloaded.GroupByHueID(groups[0].HueID)
	if !ok {
		t.Fatal("expected the group to survive the reload")
	}
	if len(g.EntityIDs) != 1 || g.EntityIDs[0] != "light.hall" {
		t.Fatalf("got members %v after reload, want [light.hall]", g.EntityIDs)
	}
}
