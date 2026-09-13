// Package ingress serves huebridge's entity-picker, group-management and
// pairing-control web UI, shown inside Home Assistant's Supervisor ingress
// panel.
package ingress

import (
	"net/http"
	"strings"
	"time"

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
	Entries        []registry.Entry
	Groups         []groupView
	Available      []string
	CurrentVersion hue.VersionTriple
	KnownVersions  []hue.VersionTriple
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
func NewHandler(reg *registry.Registry, win *hue.PairingWindow, versions *hue.VersionStore, availableEntities func() []string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
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
			Entries:        entries,
			Groups:         groups,
			Available:      availableEntities(),
			CurrentVersion: versions.Current(),
			KnownVersions:  hue.KnownVersions,
		})
	})

	mux.HandleFunc("POST /entities", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("POST /entities/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if err := reg.Remove(r.PathValue("id")); err != nil {
			http.Error(w, "failed to remove entity", http.StatusInternalServerError)
			return
		}
		redirectHome(w, r)
	})

	// Groups are what scenes are created against, so without a way to make
	// one the Hue app can never create a scene at all.
	mux.HandleFunc("POST /groups", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("POST /pairing/allow", func(w http.ResponseWriter, r *http.Request) {
		win.Open(30 * time.Second)
		redirectHome(w, r)
	})

	mux.HandleFunc("POST /version", func(w http.ResponseWriter, r *http.Request) {
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
	})

	return mux
}
