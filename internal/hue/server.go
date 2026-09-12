package hue

import (
	"net/http"

	"huebridge/internal/registry"
)

// NewServer assembles the CLIP v1 router. reg may be nil in tests that only
// exercise pairing.
func NewServer(reg *registry.Registry, wl *Whitelist, win *PairingWindow) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api", handlePairing(wl, win))
	return mux
}
