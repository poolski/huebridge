package ingress

import (
	"encoding/json"
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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, win, versions, timezones, func() []string { return []string{"light.kitchen", "light.hall"} })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, win, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, win, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

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
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	req := httptest.NewRequest("POST", "/pairing/allow", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("got Location=%q, want \"/\"", got)
	}
}

func TestIngress_SetVersionAppliesAKnownVersion(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	want := hue.KnownVersions[0]
	form := url.Values{"version": {want.DatastoreVersion + "|" + want.SwVersion + "|" + want.APIVersion}}
	req := httptest.NewRequest("POST", "/version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 303 {
		t.Fatalf("got status %d, want 303 (redirect back to the picker)", rec.Code)
	}
	if got := versions.Current(); got != want {
		t.Fatalf("got current version %+v, want %+v", got, want)
	}
}

func TestIngress_SetVersionRejectsUnknownVersion(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	before := versions.Current()
	form := url.Values{"version": {"1|not-a-real-build|9.9.9"}}
	req := httptest.NewRequest("POST", "/version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("got status %d, want 400 for an unrecognized version", rec.Code)
	}
	if got := versions.Current(); got != before {
		t.Fatalf("a rejected version change must not alter the reported version: got %+v, want %+v", got, before)
	}
}

func TestIngress_DebugSetVersionAppliesAnArbitraryVersion(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	want := hue.VersionTriple{DatastoreVersion: "1", SwVersion: "not-a-real-build", APIVersion: "9.9.9"}
	form := url.Values{
		"datastoreversion": {want.DatastoreVersion},
		"swversion":        {want.SwVersion},
		"apiversion":       {want.APIVersion},
	}
	req := httptest.NewRequest("POST", "/debug/version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if got := versions.Current(); got != want {
		t.Fatalf("got current version %+v, want %+v", got, want)
	}
}

func TestIngress_DebugSetVersionRejectsIncompleteInput(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	before := versions.Current()
	form := url.Values{"datastoreversion": {"1"}, "swversion": {"1000000000"}}
	req := httptest.NewRequest("POST", "/debug/version", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("got status %d, want 400 for a missing apiversion", rec.Code)
	}
	if got := versions.Current(); got != before {
		t.Fatalf("a rejected version change must not alter the reported version: got %+v, want %+v", got, before)
	}
}

func TestIngress_SetTimezoneAppliesARecognizedTimezone(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	form := url.Values{"timezone": {"America/New_York"}}
	req := httptest.NewRequest("POST", "/timezone", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 303 {
		t.Fatalf("got status %d, want 303 (redirect back to the picker)", rec.Code)
	}
	if got := timezones.Current(); got != "America/New_York" {
		t.Fatalf("got current timezone %q, want America/New_York", got)
	}
}

func TestIngress_SetTimezoneRejectsUnrecognizedTimezone(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	before := timezones.Current()
	form := url.Values{"timezone": {"Not/A_Real_Zone"}}
	req := httptest.NewRequest("POST", "/timezone", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("got status %d, want 400 for an unrecognized timezone", rec.Code)
	}
	if got := timezones.Current(); got != before {
		t.Fatalf("a rejected timezone change must not alter the reported timezone: got %q, want %q", got, before)
	}
}

func TestIngress_DebugRoutesListsRegisteredRoutes(t *testing.T) {
	reg, _ := registry.NewRegistry(filepath.Join(t.TempDir(), "registry.json"))
	versions := hue.NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	timezones := hue.NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	h := NewHandler(reg, &hue.PairingWindow{}, versions, timezones, func() []string { return nil })

	req := httptest.NewRequest("GET", "/debug/routes", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.api+json" {
		t.Fatalf("got Content-Type %q, want application/vnd.api+json", ct)
	}

	var doc struct {
		Data []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Method  string `json:"method"`
				Path    string `json:"path"`
				Handler string `json:"handler"`
				Group   string `json:"group"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	found := map[string]string{} // id -> group
	for _, res := range doc.Data {
		if res.Type != "route" {
			t.Fatalf("got resource type %q, want %q", res.Type, "route")
		}
		found[res.ID] = res.Attributes.Group
	}
	want := map[string]string{
		"POST /entities":      "admin",
		"POST /debug/version": "debug",
		"GET /debug/routes":   "debug",
	}
	for id, group := range want {
		got, ok := found[id]
		if !ok {
			t.Fatalf("expected /debug/routes to list %q, got %v", id, found)
		}
		if got != group {
			t.Fatalf("got group %q for %q, want %q", got, id, group)
		}
	}
}
