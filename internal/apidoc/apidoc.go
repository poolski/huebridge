// Package apidoc records the HTTP routes huebridge registers as it wires up
// its servers, so a single endpoint can describe the whole API surface (CLIP
// v1 plus the admin/debug routes) — including request/response payload
// shapes — without a hand-maintained description that drifts out of sync
// with server.go/ingress.go as routes are added or removed.
//
// Each route-registering call is wrapped with Register, which uses
// reflection (runtime.FuncForPC) to read the handler function's own name off
// the compiled binary rather than requiring a separately-typed description.
// Callers may additionally pass Request/Response options naming the Go type
// a handler decodes/encodes; the payload schema is then derived from that
// type's fields via reflection (encoding/json struct tags included), so a
// field added to e.g. hue.Light automatically shows up here too. Handlers
// that decode into an untyped map (huebridge does this for the Hue light/
// group "state"/"action" bodies, which are genuinely dynamic key sets) have
// no such type to reflect on and are simply left without a request schema —
// inventing one would claim more precision than the code actually has.
//
// The inventory is served as an OpenAPI 3.0 document (see OpenAPIDocument),
// the spec meant for describing HTTP paths and payload shapes — unlike
// JSON:API, which standardizes response envelopes for an API's own
// resources and has no vocabulary for describing other endpoints' schemas.
package apidoc

import (
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
	// Request/Response are the Go types (if any) a handler decodes/
	// encodes, supplied via the Request/Response options to Register.
	// Multiple entries mean the handler can produce/accept more than one
	// shape (e.g. GET /api/config's response depends on whether the
	// caller is a recognized user) and are rendered as an OpenAPI oneOf.
	Request  []reflect.Type
	Response []reflect.Type
	// RequestContentType is the request body's wire format, e.g.
	// "application/x-www-form-urlencoded" for the admin UI's HTML forms.
	// Empty (the common case) means "application/json".
	RequestContentType string
}

// SchemaOption attaches request/response type information to a Route at
// Register time.
type SchemaOption func(*Route)

// Request records v's type as (one of) the route's request body shape.
func Request(v any) SchemaOption {
	t := reflect.TypeOf(v)
	return func(r *Route) { r.Request = append(r.Request, t) }
}

// Response records v's type as (one of) the route's response body shape.
func Response(v any) SchemaOption {
	t := reflect.TypeOf(v)
	return func(r *Route) { r.Response = append(r.Response, t) }
}

// FormEncoded marks the route's request body as
// "application/x-www-form-urlencoded" (an HTML form post) rather than the
// default "application/json" — used by the admin UI's routes, none of which
// speak JSON in requests.
func FormEncoded(r *Route) { r.RequestContentType = "application/x-www-form-urlencoded" }

var (
	mu     sync.Mutex
	routes []Route
)

// Register records pattern (as passed to (*http.ServeMux).HandleFunc, e.g.
// "POST /api/{username}/lights") under group and returns h unchanged, so a
// route registration can be wrapped in place:
//
//	mux.HandleFunc(pattern, apidoc.Register("clip", pattern, handler))
//
// Pass Request/Response to additionally describe the payload shape(s), e.g.
//
//	apidoc.Register("clip", pattern, handler, apidoc.Response(hue.Light{}))
func Register(group, pattern string, h http.HandlerFunc, opts ...SchemaOption) http.HandlerFunc {
	method, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod {
		method, path = "", pattern
	}

	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:] // drop the module path, keep "package.func"
	}

	rt := Route{Method: method, Path: path, Handler: name, Group: group}
	for _, opt := range opts {
		opt(&rt)
	}

	mu.Lock()
	defer mu.Unlock()
	routes = append(routes, rt)
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

// Handler serves every registered route as an OpenAPI 3.0 document (see
// OpenAPIDocument). Register it after every other route in the process has
// been set up, so its own snapshot is complete.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, OpenAPIDocument("huebridge", "1.0.0"))
}
