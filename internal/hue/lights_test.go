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

func setupServerWithOneLight(t *testing.T) (*fake.Backend, *registry.Registry) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")

	return be, reg
}

func TestLights_GetAll(t *testing.T) {
	be, reg := setupServerWithOneLight(t)
	srv := NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), filepath.Join(t.TempDir(), "scenes.json"))

	req := httptest.NewRequest("GET", "/api/testuser/lights", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body map[string]Light
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	light, ok := body["1"]
	if !ok {
		t.Fatalf("got %+v, want key \"1\" for the registered light", body)
	}
	if light.Name != "Kitchen" || light.State.On {
		t.Fatalf("got %+v, want Name=Kitchen On=false", light)
	}
}

func TestLights_PutState(t *testing.T) {
	be, reg := setupServerWithOneLight(t)
	srv := NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), filepath.Join(t.TempDir(), "scenes.json"))

	req := httptest.NewRequest("PUT", "/api/testuser/lights/1/state", bytes.NewReader([]byte(`{"on":true,"bri":200}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []SuccessItem
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 2 {
		t.Fatalf("got %d success items, want 2 (on, bri)", len(body))
	}

	got, _ := be.GetState(nil, "light.kitchen")
	if !got.On || got.Brightness == nil || *got.Brightness != 200 {
		t.Fatalf("backend state = %+v, want On=true Brightness=200", got)
	}
}

func TestLights_PutStateUnknownID(t *testing.T) {
	be, reg := setupServerWithOneLight(t)
	srv := NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), filepath.Join(t.TempDir(), "scenes.json"))

	req := httptest.NewRequest("PUT", "/api/testuser/lights/99/state", bytes.NewReader([]byte(`{"on":true}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []ErrorItem
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0].Error.Type != 3 {
		t.Fatalf("got %+v, want a single type-3 (resource not available) error", body)
	}
}
