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

	srv := newAuthedServer(t, reg, be, nil, nil)

	req := httptest.NewRequest("POST", "/api/"+testUser+"/scenes", bytes.NewReader([]byte(`{"name":"Relax","group":"1"}`)))
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

	srv := newAuthedServer(t, reg, be, scenes, nil)

	req := httptest.NewRequest("DELETE", "/api/"+testUser+"/scenes/"+scene.ID, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if be.MirroredSceneCount() != 0 {
		t.Fatalf("got %d mirrored scenes after delete, want 0", be.MirroredSceneCount())
	}
	// The server was handed this very store, so the delete must be visible
	// on it in memory...
	if _, ok := scenes.Get(scene.ID); ok {
		t.Fatal("expected scene to be removed from the store passed to NewServer")
	}
	// ...and persisted to disk.
	reloaded := NewSceneStore(scenesPath)
	if _, ok := reloaded.Get(scene.ID); ok {
		t.Fatal("expected scene to be removed from the store's file after delete")
	}
}

func TestScenes_GetAllReturnsSeededScene(t *testing.T) {
	be := fake.New()
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))

	scenesPath := filepath.Join(t.TempDir(), "scenes.json")
	scenes := NewSceneStore(scenesPath)
	scene, _ := scenes.Create("Relax", "1", []string{"light.kitchen"}, map[string]backend.DesiredState{})

	srv := newAuthedServer(t, reg, be, scenes, nil)

	req := httptest.NewRequest("GET", "/api/"+testUser+"/scenes", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body map[string]Scene
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	got, ok := body[scene.ID]
	if !ok {
		t.Fatalf("got %+v, want key %q for the seeded scene", body, scene.ID)
	}
	if got.Name != "Relax" || len(got.Lights) != 1 || got.Lights[0] != "light.kitchen" {
		t.Fatalf("got %+v, want Name=Relax Lights=[light.kitchen]", got)
	}
}

func TestScenes_GetOneReturnsScene(t *testing.T) {
	be := fake.New()
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))

	scenesPath := filepath.Join(t.TempDir(), "scenes.json")
	scenes := NewSceneStore(scenesPath)
	scene, _ := scenes.Create("Relax", "1", []string{"light.kitchen"}, map[string]backend.DesiredState{})

	srv := newAuthedServer(t, reg, be, scenes, nil)

	req := httptest.NewRequest("GET", "/api/"+testUser+"/scenes/"+scene.ID, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var got Scene
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "Relax" || len(got.Lights) != 1 || got.Lights[0] != "light.kitchen" {
		t.Fatalf("got %+v, want Name=Relax Lights=[light.kitchen]", got)
	}
}

func TestScenes_GetOneUnknownIDReturnsError(t *testing.T) {
	be := fake.New()
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))

	srv := newAuthedServer(t, reg, be, nil, nil)

	req := httptest.NewRequest("GET", "/api/"+testUser+"/scenes/does-not-exist", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []ErrorItem
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0].Error.Type != 3 {
		t.Fatalf("got %+v, want a single type-3 (resource not available) error", body)
	}
}
