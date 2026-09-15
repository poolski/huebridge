package apidoc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHandler(w http.ResponseWriter, r *http.Request) {}

type widget struct {
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

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

func TestRegister_RequestAndResponseOptionsAreRecorded(t *testing.T) {
	routes = nil

	Register("clip", "POST /widgets", testHandler, Request(widget{}), Response([]widget{}))

	all := All()
	if len(all) != 1 {
		t.Fatalf("got %d routes, want 1", len(all))
	}
	if len(all[0].Request) != 1 || all[0].Request[0].Name() != "widget" {
		t.Fatalf("got Request=%v, want [widget]", all[0].Request)
	}
	if len(all[0].Response) != 1 || all[0].Response[0].Kind().String() != "slice" {
		t.Fatalf("got Response=%v, want [[]widget]", all[0].Response)
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

func TestHandler_ServesAnOpenAPIDocumentOfRegisteredRoutes(t *testing.T) {
	routes = nil
	Register("clip", "POST /widgets", testHandler, Request(widget{}), Response(widget{}))

	req := httptest.NewRequest("GET", "/debug/routes", nil)
	rec := httptest.NewRecorder()
	Handler(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("got Content-Type %q, want application/json", ct)
	}

	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Tags        []string
			RequestBody struct {
				Content map[string]struct {
					Schema map[string]any `json:"schema"`
				} `json:"content"`
			} `json:"requestBody"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if doc.OpenAPI == "" {
		t.Fatal("expected a non-empty openapi version field")
	}
	op, ok := doc.Paths["/widgets"]["post"]
	if !ok {
		t.Fatalf("expected a POST /widgets operation, got paths %v", doc.Paths)
	}
	if len(op.Tags) != 1 || op.Tags[0] != "clip" {
		t.Fatalf("got tags %v, want [clip]", op.Tags)
	}
	ref := op.RequestBody.Content["application/json"].Schema["$ref"]
	if ref != "#/components/schemas/widget" {
		t.Fatalf("got request schema %v, want a $ref to widget", op.RequestBody.Content["application/json"].Schema)
	}
	if _, ok := doc.Components.Schemas["widget"]; !ok {
		t.Fatalf("expected components.schemas to define widget, got %v", doc.Components.Schemas)
	}
}
