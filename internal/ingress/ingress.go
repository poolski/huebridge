// Package ingress serves huebridge's entity-picker and pairing-control web
// UI, shown inside Home Assistant's Supervisor ingress panel.
package ingress

import (
	"net/http"
	"time"

	"huebridge/internal/hue"
	"huebridge/internal/registry"
)

type indexData struct {
	Entries   []registry.Entry
	Available []string
}

// NewHandler builds the ingress UI. availableEntities returns the set of HA
// entity ids the picker offers — supplied as a func rather than a fixed
// list so the caller can refresh it from HA's entity registry on each page
// load without this package depending on the HA client directly.
func NewHandler(reg *registry.Registry, win *hue.PairingWindow, availableEntities func() []string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		indexTemplate.Execute(w, indexData{
			Entries:   reg.All(),
			Available: availableEntities(),
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("POST /pairing/allow", func(w http.ResponseWriter, r *http.Request) {
		win.Open(30 * time.Second)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return mux
}
