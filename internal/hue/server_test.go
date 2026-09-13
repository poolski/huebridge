package hue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huebridge/internal/backend"
	"huebridge/internal/backend/fake"
	"huebridge/internal/registry"
)

// testUser is a whitelisted username the shared test helpers pair in
// advance, so tests exercising authenticated routes don't each have to run
// the pairing flow first.
const testUser = "testuser"

// newAuthedServer builds a CLIP v1 server with testUser already in the
// whitelist. Passing nil for scenes/schedules gives the server fresh stores
// under t.TempDir().
func newAuthedServer(t *testing.T, reg *registry.Registry, be backend.Backend, scenes *SceneStore, schedules *ScheduleStore) *http.ServeMux {
	t.Helper()
	if scenes == nil {
		scenes = NewSceneStore(filepath.Join(t.TempDir(), "scenes.json"))
	}
	if schedules == nil {
		schedules = NewScheduleStore(filepath.Join(t.TempDir(), "schedules.json"))
	}
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	if err := wl.Add(WhitelistEntry{Username: testUser, Name: "test#app", CreateDate: time.Now()}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	return NewServer(reg, be, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), scenes, schedules, "192.168.1.100")
}

func TestAuth_UnknownUsernameIsRejected(t *testing.T) {
	be, reg := setupServerWithOneLight(t)
	srv := newAuthedServer(t, reg, be, nil, nil)

	req := httptest.NewRequest("GET", "/api/not-a-paired-user/lights", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []ErrorItem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0].Error.Type != 1 {
		t.Fatalf("got %+v, want a single type-1 (unauthorized user) error", body)
	}
}

func TestAuth_PairedUsernameIsAccepted(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")

	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	win := &PairingWindow{}
	win.Open(30 * timeSecond)
	srv := NewServer(reg, be, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"),
		NewSceneStore(filepath.Join(t.TempDir(), "scenes.json")),
		NewScheduleStore(filepath.Join(t.TempDir(), "schedules.json")), "192.168.1.100")

	// Pair first, then use the username the bridge handed back.
	pairReq := httptest.NewRequest("POST", "/api", strings.NewReader(`{"devicetype":"test#app"}`))
	pairRec := httptest.NewRecorder()
	srv.ServeHTTP(pairRec, pairReq)

	var pairBody []SuccessItem
	if err := json.NewDecoder(pairRec.Body).Decode(&pairBody); err != nil {
		t.Fatalf("decode pairing response: %v", err)
	}
	if len(pairBody) != 1 {
		t.Fatalf("got %+v, want a single pairing success item", pairBody)
	}
	username, _ := pairBody[0].Success["username"].(string)
	if username == "" {
		t.Fatal("pairing returned an empty username")
	}

	req := httptest.NewRequest("GET", "/api/"+username+"/lights", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body map[string]Light
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode lights response: %v", err)
	}
	if _, ok := body["1"]; !ok {
		t.Fatalf("got %+v, want the registered light under key \"1\"", body)
	}
}

// TestServer_SharesScheduleStoreWithCaller proves NewServer writes through
// the very store instance it was handed, rather than opening a second one
// over the same file — the caller and the ticker must observe each other's
// writes.
func TestServer_SharesScheduleStoreWithCaller(t *testing.T) {
	be, reg := setupServerWithOneLight(t)
	schedules := NewScheduleStore(filepath.Join(t.TempDir(), "schedules.json"))
	srv := newAuthedServer(t, reg, be, nil, schedules)

	req := httptest.NewRequest("POST", "/api/"+testUser+"/schedules",
		strings.NewReader(`{"name":"Wake up","localtime":"W127/T07:00:00","command":{"address":"/lights/1/state","method":"PUT","body":{"on":true}}}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []SuccessItem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("got %+v, want a single success item", body)
	}
	id, _ := body[0].Success["id"].(string)

	if _, ok := schedules.Get(id); !ok {
		t.Fatalf("schedule %q is not visible in the store passed to NewServer", id)
	}
}

// TestServer_UpdaterAcceptsPushedFirmware covers POST /updater — the app
// pushes a firmware file here when it believes an update is available.
// huebridge has nothing to install; this just must not 404.
func TestServer_UpdaterAcceptsPushedFirmware(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil, "192.168.1.100")

	req := httptest.NewRequest("POST", "/updater", strings.NewReader("BSB002"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}
