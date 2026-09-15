// Package ingress serves huebridge's entity-picker, group-management and
// pairing-control web UI, shown inside Home Assistant's Supervisor ingress
// panel.
package ingress

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"huebridge/internal/apidoc"
	"huebridge/internal/hue"
	"huebridge/internal/registry"
)

// groupView pairs a registry group with its members' display names, so the
// index can show "Downstairs — Kitchen, Hall" rather than raw entity ids.
type groupView struct {
	registry.Group
	MemberNames []string
}

type indexData struct {
	Entries         []registry.Entry
	Groups          []groupView
	Available       []string
	CurrentVersion  hue.VersionTriple
	KnownVersions   []hue.VersionTriple
	CurrentTimezone string
	CommonTimezones []string
}

// These types exist solely to give apidoc's reflection-based schema
// generation something to read — the handlers below still parse their
// bodies with r.ParseForm/r.FormValue, not json.Decode, since the admin UI
// posts regular HTML forms. Each type's fields and json tags must be kept in
// sync with the FormValue calls the corresponding handler actually makes.

// addEntityRequest is POST /entities's form body.
type addEntityRequest struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
}

// addGroupRequest is POST /groups's form body.
type addGroupRequest struct {
	Name      string   `json:"name"`
	EntityIDs []string `json:"entity_id"`
	Class     string   `json:"class,omitempty"`
}

// setVersionRequest is POST /version's form body — Version is the
// "datastore|sw|api" encoding versionOptionValue produces.
type setVersionRequest struct {
	Version string `json:"version"`
}

// setDebugVersionRequest is POST /debug/version's form body.
type setDebugVersionRequest struct {
	DatastoreVersion string `json:"datastoreversion"`
	SwVersion        string `json:"swversion"`
	APIVersion       string `json:"apiversion"`
}

// setTimezoneRequest is POST /timezone's form body.
type setTimezoneRequest struct {
	Timezone string `json:"timezone"`
}

// versionOptionValue encodes v as a single <option value> so the three
// fields survive a form round-trip together, never mixed with another
// triple's field.
func versionOptionValue(v hue.VersionTriple) string {
	return v.DatastoreVersion + "|" + v.SwVersion + "|" + v.APIVersion
}

// parseVersionOptionValue reverses versionOptionValue.
func parseVersionOptionValue(s string) (hue.VersionTriple, bool) {
	parts := strings.SplitN(s, "|", 3)
	if len(parts) != 3 {
		return hue.VersionTriple{}, false
	}
	return hue.VersionTriple{DatastoreVersion: parts[0], SwVersion: parts[1], APIVersion: parts[2]}, true
}

// redirectHome sends the browser back to the index. Home Assistant serves
// the add-on under a per-session ingress prefix and tells us what it is via
// X-Ingress-Path; a bare "/" would escape the panel and land on HA's own
// dashboard.
func redirectHome(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSuffix(r.Header.Get("X-Ingress-Path"), "/") + "/"
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// NewHandler builds the ingress UI. availableEntities returns the set of HA
// entity ids the picker offers — supplied as a func rather than a fixed
// list so the caller can refresh it from HA's entity registry on each page
// load without this package depending on the HA client directly.
func NewHandler(reg *registry.Registry, win *hue.PairingWindow, versions *hue.VersionStore, timezones *hue.TimezoneStore, availableEntities func() []string) http.Handler {
	mux := http.NewServeMux()

	register := func(group, pattern string, h http.HandlerFunc, opts ...apidoc.SchemaOption) {
		mux.HandleFunc(pattern, apidoc.Register(group, pattern, h, opts...))
	}

	register("admin", "GET /", func(w http.ResponseWriter, r *http.Request) {
		entries := reg.All()
		names := make(map[string]string, len(entries))
		for _, e := range entries {
			names[e.EntityID] = e.Name
		}

		groups := make([]groupView, 0, len(reg.AllGroups()))
		for _, g := range reg.AllGroups() {
			members := make([]string, 0, len(g.EntityIDs))
			for _, id := range g.EntityIDs {
				if name, ok := names[id]; ok {
					members = append(members, name)
				} else {
					members = append(members, id)
				}
			}
			groups = append(groups, groupView{Group: g, MemberNames: members})
		}

		indexTemplate.Execute(w, indexData{
			Entries:         entries,
			Groups:          groups,
			Available:       availableEntities(),
			CurrentVersion:  versions.Current(),
			KnownVersions:   hue.KnownVersions,
			CurrentTimezone: timezones.Current(),
			CommonTimezones: hue.CommonTimezones,
		})
	})

	register("admin", "POST /entities", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		entityID := r.FormValue("entity_id")
		name := r.FormValue("name")
		if entityID == "" || name == "" {
			http.Error(w, "entity_id and name are required", http.StatusBadRequest)
			return
		}
		if _, err := reg.Add(entityID, name); err != nil {
			http.Error(w, "failed to add entity", http.StatusInternalServerError)
			return
		}
		redirectHome(w, r)
	}, apidoc.FormEncoded, apidoc.Request(addEntityRequest{}))

	register("admin", "POST /entities/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if err := reg.Remove(r.PathValue("id")); err != nil {
			http.Error(w, "failed to remove entity", http.StatusInternalServerError)
			return
		}
		redirectHome(w, r)
	})

	// Groups are what scenes are created against, so without a way to make
	// one the Hue app can never create a scene at all.
	register("admin", "POST /groups", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		name := r.FormValue("name")
		entityIDs := r.Form["entity_id"]
		if name == "" || len(entityIDs) == 0 {
			http.Error(w, "name and at least one entity are required", http.StatusBadRequest)
			return
		}

		class := r.FormValue("class")
		if class == "" {
			class = "Other"
		}

		if _, err := reg.AddGroup(name, class, entityIDs); err != nil {
			http.Error(w, "failed to add group", http.StatusInternalServerError)
			return
		}
		redirectHome(w, r)
	}, apidoc.FormEncoded, apidoc.Request(addGroupRequest{}))

	register("admin", "POST /pairing/allow", func(w http.ResponseWriter, r *http.Request) {
		win.Open(30 * time.Second)
		redirectHome(w, r)
	})

	register("admin", "POST /version", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		v, ok := parseVersionOptionValue(r.FormValue("version"))
		if !ok {
			http.Error(w, "version is required", http.StatusBadRequest)
			return
		}
		if err := versions.Set(v); err != nil {
			http.Error(w, "not a recognized version: "+err.Error(), http.StatusBadRequest)
			return
		}
		redirectHome(w, r)
	}, apidoc.FormEncoded, apidoc.Request(setVersionRequest{}))

	// POST /debug/version accepts an arbitrary datastore/software/API version
	// triple, unlike POST /version above which only allows KnownVersions —
	// it exists for scripting tests against combinations the admin picker
	// deliberately won't offer.
	register("debug", "POST /debug/version", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		v := hue.VersionTriple{
			DatastoreVersion: r.FormValue("datastoreversion"),
			SwVersion:        r.FormValue("swversion"),
			APIVersion:       r.FormValue("apiversion"),
		}
		if err := versions.SetUnchecked(v); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}, apidoc.FormEncoded, apidoc.Request(setDebugVersionRequest{}), apidoc.Response(hue.VersionTriple{}))

	register("admin", "POST /timezone", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		tz := r.FormValue("timezone")
		if tz == "" {
			http.Error(w, "timezone is required", http.StatusBadRequest)
			return
		}
		if err := timezones.Set(tz); err != nil {
			http.Error(w, "not a recognized timezone: "+err.Error(), http.StatusBadRequest)
			return
		}
		redirectHome(w, r)
	}, apidoc.FormEncoded, apidoc.Request(setTimezoneRequest{}))

	// GET /debug/routes describes every route registered across all of
	// huebridge's servers (this admin mux and the CLIP v1 mux built by
	// hue.NewServer) as an OpenAPI 3.0 document, payload schemas included —
	// see internal/apidoc. Register it last so its own entry is included in
	// the snapshot it serves.
	register("debug", "GET /debug/routes", apidoc.Handler)

	return mux
}
