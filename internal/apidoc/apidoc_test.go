package apidoc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHandler(w http.ResponseWriter, r *http.Request) {}

func TestRegister_RecordsMethodPathAndHandlerName(t *testing.T) {
	routes = nil // isolate from any routes other tests in this package recorded

	got := Register("clip", "GET /api/{username}/lights", testHandler)
	if got == nil {
		t.Fatal("Register must return the handler unchanged")
	}

	all := All()
	if len(all) != 1 {
		t.Fatalf("got %d routes, want 1", len(all))
	}
	r := all[0]
	if r.Method != "GET" || r.Path != "/api/{username}/lights" || r.Group != "clip" {
		t.Fatalf("got %+v, want Method=GET Path=/api/{username}/lights Group=clip", r)
	}
	if r.Handler == "" {
		t.Fatal("expected a non-empty handler name from reflection")
	}
}

func TestRegister_PatternWithoutMethodLeavesMethodEmpty(t *testing.T) {
	routes = nil

	Register("admin", "/no-verb", testHandler)

	all := All()
	if len(all) != 1 {
		t.Fatalf("got %d routes, want 1", len(all))
	}
	if all[0].Method != "" || all[0].Path != "/no-verb" {
		t.Fatalf("got %+v, want Method=\"\" Path=/no-verb", all[0])
	}
}

func TestAll_IsSortedAndIsolatedFromTheRegistry(t *testing.T) {
	routes = nil

	Register("clip", "POST /b", testHandler)
	Register("clip", "GET /a", testHandler)

	all := All()
	if len(all) != 2 || all[0].Path != "/a" || all[1].Path != "/b" {
		t.Fatalf("got %+v, want routes sorted by path", all)
	}

	all[0].Path = "mutated"
	if routes[1].Path == "mutated" {
		t.Fatal("All() must return a copy, not a view into the internal registry")
	}
}

func TestHandler_ServesAJSONAPIDocumentOfRegisteredRoutes(t *testing.T) {
	routes = nil
	Register("debug", "GET /debug/routes", Handler)

	req := httptest.NewRequest("GET", "/debug/routes", nil)
	rec := httptest.NewRecorder()
	Handler(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.api+json" {
		t.Fatalf("got Content-Type %q, want application/vnd.api+json", ct)
	}

	var doc document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(doc.Data) != 1 {
		t.Fatalf("got %d resources, want 1", len(doc.Data))
	}
	res := doc.Data[0]
	if res.Type != "route" || res.ID != "GET /debug/routes" {
		t.Fatalf("got %+v, want Type=route ID=\"GET /debug/routes\"", res)
	}
	if res.Attributes.Method != "GET" || res.Attributes.Path != "/debug/routes" || res.Attributes.Group != "debug" {
		t.Fatalf("got attributes %+v, want Method=GET Path=/debug/routes Group=debug", res.Attributes)
	}
}
