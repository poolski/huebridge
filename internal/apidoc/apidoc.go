// Package apidoc records the HTTP routes huebridge registers as it wires up
// its servers, so a single endpoint can describe the whole API surface (CLIP
// v1 plus the admin/debug routes) without a hand-maintained list that drifts
// out of sync with server.go/ingress.go as routes are added or removed.
//
// Each route-registering call is wrapped with Register, which uses
// reflection (runtime.FuncForPC) to read the handler function's own name off
// the compiled binary rather than requiring a separately-typed description —
// that's what keeps the inventory dynamic as the code changes.
package apidoc

import (
	"encoding/json"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Route describes one registered HTTP route.
type Route struct {
	// Method is the HTTP verb, e.g. "GET" — empty for a pattern with no
	// verb prefix (Go's ServeMux allows registering those too).
	Method string
	// Path is the ServeMux pattern's path portion, e.g. "/api/{username}/lights".
	Path string
	// Handler is the registering function's name, as read off the
	// compiled binary — "package.handleGetLights" for a plain
	// http.HandlerFunc, "package.handleGetLights.func1" for a closure
	// returned by a factory function of that name.
	Handler string
	// Group labels which server the route belongs to (e.g. "clip",
	// "admin", "debug"), since huebridge runs more than one mux.
	Group string
}

var (
	mu     sync.Mutex
	routes []Route
)

// Register records pattern (as passed to (*http.ServeMux).HandleFunc, e.g.
// "POST /api/{username}/lights") under group and returns h unchanged, so a
// route registration can be wrapped in place:
//
//	mux.HandleFunc(pattern, apidoc.Register("clip", pattern, handler))
func Register(group, pattern string, h http.HandlerFunc) http.HandlerFunc {
	method, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod {
		method, path = "", pattern
	}

	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:] // drop the module path, keep "package.func"
	}

	mu.Lock()
	defer mu.Unlock()
	routes = append(routes, Route{Method: method, Path: path, Handler: name, Group: group})
	return h
}

// All returns a snapshot of every route registered so far, sorted by path
// then method so the output is stable across process restarts.
func All() []Route {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Route, len(routes))
	copy(out, routes)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// resource is a JSON:API (https://jsonapi.org) resource object describing
// one route.
type resource struct {
	Type       string     `json:"type"`
	ID         string     `json:"id"`
	Attributes attributes `json:"attributes"`
}

type attributes struct {
	Method  string `json:"method,omitempty"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
	Group   string `json:"group"`
}

// document is the top-level JSON:API envelope.
type document struct {
	Data []resource `json:"data"`
}

// Handler serves every registered route as a JSON:API document. Register it
// after every other route in the process has been set up, so its own
// snapshot is complete.
func Handler(w http.ResponseWriter, r *http.Request) {
	rs := All()
	data := make([]resource, len(rs))
	for i, rt := range rs {
		id := rt.Path
		if rt.Method != "" {
			id = rt.Method + " " + rt.Path
		}
		data[i] = resource{
			Type: "route",
			ID:   id,
			Attributes: attributes{
				Method:  rt.Method,
				Path:    rt.Path,
				Handler: rt.Handler,
				Group:   rt.Group,
			},
		}
	}

	w.Header().Set("Content-Type", "application/vnd.api+json")
	json.NewEncoder(w).Encode(document{Data: data})
}
