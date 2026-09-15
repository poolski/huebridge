package apidoc

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

// pathParamPattern matches the {name} segments Go 1.22+'s ServeMux and
// OpenAPI's path templating happen to share the exact same syntax for.
var pathParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

// OpenAPIDocument renders every route recorded via Register as an OpenAPI
// 3.0 document: paths, operations grouped by tag (Route.Group), and
// request/response schemas derived by reflecting over the Go types passed to
// Request/Response at registration time.
func OpenAPIDocument(title, version string) map[string]any {
	schemas := map[string]any{}
	paths := map[string]any{}

	for _, rt := range All() {
		item, _ := paths[rt.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[rt.Path] = item
		}

		op := map[string]any{
			"operationId": operationID(rt),
			"tags":        []string{rt.Group},
		}
		if params := pathParameters(rt.Path); len(params) > 0 {
			op["parameters"] = params
		}
		if len(rt.Request) > 0 {
			contentType := rt.RequestContentType
			if contentType == "" {
				contentType = "application/json"
			}
			op["requestBody"] = map[string]any{
				"content": map[string]any{
					contentType: map[string]any{"schema": schemaOrOneOf(rt.Request, schemas)},
				},
			}
		}

		responses := map[string]any{"200": map[string]any{"description": "OK"}}
		if len(rt.Response) > 0 {
			responses["200"] = map[string]any{
				"description": "OK",
				"content": map[string]any{
					"application/json": map[string]any{"schema": schemaOrOneOf(rt.Response, schemas)},
				},
			}
		}
		op["responses"] = responses

		method := rt.Method
		if method == "" {
			method = "GET"
		}
		item[strings.ToLower(method)] = op
	}

	doc := map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": title, "version": version},
		"paths":   paths,
	}
	if len(schemas) > 0 {
		doc["components"] = map[string]any{"schemas": schemas}
	}
	return doc
}

// operationID must be unique across the document; the recorded handler name
// already is, since Go gives every function/closure literal a distinct name.
func operationID(rt Route) string {
	return rt.Handler
}

func pathParameters(path string) []map[string]any {
	var params []map[string]any
	for _, m := range pathParamPattern.FindAllStringSubmatch(path, -1) {
		params = append(params, map[string]any{
			"name":     m[1],
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": "string"},
		})
	}
	return params
}

// schemaOrOneOf renders a single schema for one type, or a oneOf of several
// for a handler whose response/request shape depends on the request (e.g.
// GET /api/config's stripped-vs-full config).
func schemaOrOneOf(types []reflect.Type, schemas map[string]any) map[string]any {
	if len(types) == 1 {
		return schemaFor(types[0], schemas)
	}
	alts := make([]map[string]any, len(types))
	for i, t := range types {
		alts[i] = schemaFor(t, schemas)
	}
	return map[string]any{"oneOf": alts}
}

// schemaFor derives a JSON Schema fragment from a Go type via reflection.
// Named struct types are registered once under components/schemas and
// referenced by $ref from then on, so a type used by several routes (e.g.
// hue.Light) only needs describing once.
func schemaFor(t reflect.Type, schemas map[string]any) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaFor(t.Elem(), schemas)}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaFor(t.Elem(), schemas)}
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			return structSchema(t, schemas) // anonymous struct: inline, nothing to name a $ref after
		}
		if _, exists := schemas[name]; !exists {
			schemas[name] = map[string]any{} // reserved: breaks recursion on self-referential types
			schemas[name] = structSchema(t, schemas)
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	default: // interface{}/any (e.g. a map[string]any value) and anything else: unconstrained
		return map[string]any{}
	}
}

func structSchema(t reflect.Type, schemas map[string]any) map[string]any {
	props := map[string]any{}
	var required []string
	for f := range t.Fields() {
		if f.PkgPath != "" { // unexported: encoding/json would skip it too
			continue
		}
		name, omitempty := f.Name, false
		if tag := f.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				name = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					omitempty = true
				}
			}
		}
		props[name] = schemaFor(f.Type, schemas)
		if !omitempty {
			required = append(required, name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func writeJSON(w http.ResponseWriter, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
