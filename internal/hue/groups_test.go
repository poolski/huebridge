package hue

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"huebridge/internal/backend"
	"huebridge/internal/backend/fake"
	"huebridge/internal/registry"
)

func setupServerWithGroup(t *testing.T) (*fake.Backend, *registry.Registry) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})
	be.Seed(backend.EntityState{EntityID: "light.hall", On: false, Reachable: true})

	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	reg.Add("light.hall", "Hall")
	reg.AddGroup("Downstairs", "Living room", []string{"light.kitchen", "light.hall"})

	return be, reg
}

func newTestServer(reg *registry.Registry, be backend.Backend, t *testing.T) *http.ServeMux {
	mac := mustParseMAC("aa:bb:cc:dd:ee:ff")
	return NewServer(reg, be, NewWhitelist(filepath.Join(t.TempDir(), "wl.json")), &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, filepath.Join(t.TempDir(), "scenes.json"), filepath.Join(t.TempDir(), "schedules.json"))
}

func TestGroups_GetOne(t *testing.T) {
	be, reg := setupServerWithGroup(t)
	srv := newTestServer(reg, be, t)

	req := httptest.NewRequest("GET", "/api/testuser/groups/1", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var g Group
	json.NewDecoder(rec.Body).Decode(&g)
	if g.Name != "Downstairs" || len(g.Lights) != 2 {
		t.Fatalf("got %+v, want Name=Downstairs with 2 lights", g)
	}
	if !g.GroupState.AnyOn || g.GroupState.AllOn {
		t.Fatalf("got GroupState=%+v, want AnyOn=true AllOn=false (one light on, one off)", g.GroupState)
	}
}

func TestGroups_PutAction(t *testing.T) {
	be, reg := setupServerWithGroup(t)
	srv := newTestServer(reg, be, t)

	req := httptest.NewRequest("PUT", "/api/testuser/groups/1/action", bytes.NewReader([]byte(`{"on":true}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	kitchen, _ := be.GetState(nil, "light.kitchen")
	hall, _ := be.GetState(nil, "light.hall")
	if !kitchen.On || !hall.On {
		t.Fatalf("got kitchen.On=%v hall.On=%v, want both true", kitchen.On, hall.On)
	}
}
