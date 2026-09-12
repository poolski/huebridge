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

func TestIngress_DeleteEntityRemovesIt(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	req := httptest.NewRequest("POST", "/entities/light.kitchen/delete", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 303 {
		t.Fatalf("got status %d, want 303 (redirect back to the picker)", rec.Code)
	}
	if _, ok := reg.ByEntityID("light.kitchen"); ok {
		t.Fatal("expected light.kitchen to be removed after POST /entities/{id}/delete")
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

func TestIngress_CreateGroupRegistersIt(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	reg.Add("light.hall", "Hall")
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	form := url.Values{
		"name":      {"Downstairs"},
		"class":     {"Living room"},
		"entity_id": {"light.kitchen", "light.hall"},
	}
	req := httptest.NewRequest("POST", "/groups", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 303 {
		t.Fatalf("got status %d, want 303", rec.Code)
	}

	groups := reg.AllGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].Name != "Downstairs" || groups[0].Class != "Living room" || len(groups[0].EntityIDs) != 2 {
		t.Fatalf("got %+v, want Downstairs/Living room with 2 members", groups[0])
	}
}

func TestIngress_CreateGroupRequiresMembers(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	form := url.Values{"name": {"Empty"}}
	req := httptest.NewRequest("POST", "/groups", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("got status %d, want 400 for a group with no members", rec.Code)
	}
}

func TestIngress_IndexListsGroups(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	reg.Add("light.kitchen", "Kitchen")
	reg.AddGroup("Downstairs", "Living room", []string{"light.kitchen"})
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Downstairs") || !strings.Contains(body, "Kitchen") {
		t.Fatal("expected the index page to list the Downstairs group and its Kitchen member")
	}
}

func TestIngress_RedirectRespectsIngressPathPrefix(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	req := httptest.NewRequest("POST", "/pairing/allow", nil)
	req.Header.Set("X-Ingress-Path", "/api/hassio_ingress/abc123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Location"); got != "/api/hassio_ingress/abc123/" {
		t.Fatalf("got Location=%q, want the ingress prefix preserved", got)
	}
}

func TestIngress_RedirectFallsBackToRootWithoutHeader(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, func() []string { return nil })

	req := httptest.NewRequest("POST", "/pairing/allow", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("got Location=%q, want \"/\"", got)
	}
}
