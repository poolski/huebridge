package hue

import (
	"net"
	"net/http"

	"huebridge/internal/backend"
	"huebridge/internal/registry"
)

// NewServer assembles the CLIP v1 router. reg/be may be nil in tests that
// only exercise pairing/config.
func NewServer(reg *registry.Registry, be backend.Backend, wl *Whitelist, win *PairingWindow, bridgeID string, mac net.HardwareAddr, scenesPath string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api", handlePairing(wl, win))
	mux.HandleFunc("GET /api/{username}/config", handleGetConfig(bridgeID, mac, win))

	if reg != nil && be != nil {
		mux.HandleFunc("GET /api/{username}/lights", handleGetLights(reg, be))
		mux.HandleFunc("GET /api/{username}/lights/{id}", handleGetLight(reg, be))
		mux.HandleFunc("PUT /api/{username}/lights/{id}/state", handlePutLightState(reg, be))
		mux.HandleFunc("GET /api/{username}/groups", handleGetGroups(reg, be))
		mux.HandleFunc("GET /api/{username}/groups/{id}", handleGetGroup(reg, be))
		mux.HandleFunc("PUT /api/{username}/groups/{id}/action", handlePutGroupAction(reg, be))

		scenes := NewSceneStore(scenesPath)
		mux.HandleFunc("GET /api/{username}/scenes", handleGetScenes(scenes))
		mux.HandleFunc("GET /api/{username}/scenes/{id}", handleGetScene(scenes))
		mux.HandleFunc("POST /api/{username}/scenes", handlePostScene(reg, be, scenes))
		mux.HandleFunc("DELETE /api/{username}/scenes/{id}", handleDeleteScene(scenes, be))
	}

	return mux
}
