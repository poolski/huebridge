package hue

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"huebridge/internal/backend"
	"huebridge/internal/backend/fake"
	"huebridge/internal/registry"
)

func TestScenes_CreateFromGroupCapturesCurrentState(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	reg.AddGroup("Downstairs", "Living room", []string{"light.kitchen"})

	srv := NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), filepath.Join(t.TempDir(), "scenes.json"))

	req := httptest.NewRequest("POST", "/api/testuser/scenes", bytes.NewReader([]byte(`{"name":"Relax","group":"1"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []SuccessItem
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0].Success["id"] == nil {
		t.Fatalf("got %+v, want a single success item with an id", body)
	}

	if be.MirroredSceneCount() != 1 {
		t.Fatalf("got %d mirrored scenes on the backend, want 1", be.MirroredSceneCount())
	}
}

func TestScenes_DeleteRemovesMirroredScene(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	reg.AddGroup("Downstairs", "Living room", []string{"light.kitchen"})

	scenesPath := filepath.Join(t.TempDir(), "scenes.json")
	scenes := NewSceneStore(scenesPath)
	scene, _ := scenes.Create("Relax", "1", []string{"light.kitchen"}, map[string]backend.DesiredState{})
	be.MirrorScene(nil, scene.ID, "Relax", nil)

	srv := NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), scenesPath)

	req := httptest.NewRequest("DELETE", "/api/testuser/scenes/"+scene.ID, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if be.MirroredSceneCount() != 0 {
		t.Fatalf("got %d mirrored scenes after delete, want 0", be.MirroredSceneCount())
	}
	// Reload from disk: the server handled the delete through its own
	// SceneStore instance (constructed from the same scenesPath inside
	// NewServer), so we verify persistence rather than checking the
	// in-memory `scenes` handle above, which never observes that store's
	// writes.
	reloaded := NewSceneStore(scenesPath)
	if _, ok := reloaded.Get(scene.ID); ok {
		t.Fatal("expected scene to be removed from the store after delete")
	}
}
