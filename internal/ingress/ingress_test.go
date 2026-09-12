package ingress

import (
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"huebridge/internal/hue"
	"huebridge/internal/registry"
)

func TestIngress_AddEntityRegistersIt(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	win := &hue.PairingWindow{}
	h := NewHandler(reg, win, func() []string { return []string{"light.kitchen", "light.hall"} })

	form := url.Values{"entity_id": {"light.kitchen"}, "name": {"Kitchen"}}
	req := httptest.NewRequest("POST", "/entities", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 303 {
		t.Fatalf("got status %d, want 303 (redirect back to the picker)", rec.Code)
	}

	if _, ok := reg.ByEntityID("light.kitchen"); !ok {
		t.Fatal("expected light.kitchen to be registered after POST /entities")
	}
}

func TestIngress_AllowPairingOpensWindow(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	win := &hue.PairingWindow{}
	h := NewHandler(reg, win, func() []string { return nil })

	req := httptest.NewRequest("POST", "/pairing/allow", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !win.IsOpen() {
		t.Fatal("expected the pairing window to be open after POST /pairing/allow")
	}
}

func TestIngress_IndexListsRegisteredEntities(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	win := &hue.PairingWindow{}
	h := NewHandler(reg, win, func() []string { return nil })

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Kitchen") {
		t.Fatal("expected the index page to list the registered \"Kitchen\" entity")
	}
}
