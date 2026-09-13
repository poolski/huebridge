package hue

import (
	"net"
	"net/http"

	"huebridge/internal/backend"
	"huebridge/internal/registry"
)

// requireUser rejects any request whose {username} path value isn't in the
// whitelist with the CLIP v1 "unauthorized user" error (type 1), matching
// what a real bridge returns for an unpaired or revoked application key.
func requireUser(wl *Whitelist, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.PathValue("username")
		if _, ok := wl.Lookup(username); !ok {
			WriteError(w, http.StatusOK, 1, "/", "unauthorized user")
			return
		}
		next(w, r)
	}
}

// NewServer assembles the CLIP v1 router. reg/be may be nil in tests that
// only exercise pairing/config. scenes and schedules are passed in already
// constructed so the caller can share the same stores with the schedule
// ticker — two stores over one file would each hold their own stale copy of
// its contents.
func NewServer(reg *registry.Registry, be backend.Backend, wl *Whitelist, win *PairingWindow, bridgeID string, mac net.HardwareAddr, scenes *SceneStore, schedules *ScheduleStore) *http.ServeMux {
	mux := http.NewServeMux()

	// POST /api is the pairing endpoint: it has no username yet, so it is
	// the one route that must not sit behind requireUser. Go's ServeMux
	// treats /api and /api/ as different patterns, but real clients aren't
	// consistent about the trailing slash (Hue Essentials always sends
	// one), so both need to resolve here.
	mux.HandleFunc("POST /api", handlePairing(wl, win))
	mux.HandleFunc("POST /api/", handlePairing(wl, win))

	handle := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, requireUser(wl, h))
	}

	// GET /api/config and GET /api/{username}/config are deliberately NOT
	// behind requireUser. Real bridges answer the no-username form
	// unauthenticated too (some clients, e.g. Hue Essentials, probe it
	// directly before pairing), and the app also reads the {username} form
	// with a throwaway username before it has paired, to identify the
	// bridge; a real bridge answers an unrecognized user with a stripped
	// config rather than an error (see
	// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config"). The
	// payload we serve is already that stripped subset — no whitelist, no
	// network details.
	mux.HandleFunc("GET /api/config", handleGetConfig(bridgeID, mac, win))
	mux.HandleFunc("GET /api/{username}/config", handleGetConfig(bridgeID, mac, win))

	if reg != nil && be != nil {
		handle("GET /api/{username}/lights", handleGetLights(reg, be))
		handle("GET /api/{username}/lights/{id}", handleGetLight(reg, be))
		handle("PUT /api/{username}/lights/{id}/state", handlePutLightState(reg, be))
		handle("GET /api/{username}/groups", handleGetGroups(reg, be))
		handle("GET /api/{username}/groups/{id}", handleGetGroup(reg, be))
		handle("PUT /api/{username}/groups/{id}/action", handlePutGroupAction(reg, be))

		handle("GET /api/{username}/scenes", handleGetScenes(scenes))
		handle("GET /api/{username}/scenes/{id}", handleGetScene(scenes))
		handle("POST /api/{username}/scenes", handlePostScene(reg, be, scenes))
		handle("DELETE /api/{username}/scenes/{id}", handleDeleteScene(scenes, be))

		handle("GET /api/{username}/schedules", handleGetSchedules(schedules))
		handle("GET /api/{username}/schedules/{id}", handleGetSchedule(schedules))
		handle("POST /api/{username}/schedules", handlePostSchedule(schedules))
		handle("DELETE /api/{username}/schedules/{id}", handleDeleteSchedule(schedules))
	}

	return mux
}
