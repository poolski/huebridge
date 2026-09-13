package hue

import (
	"encoding/json"
	"net"
	"net/http"
)

func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow, wl *Whitelist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: "126",
			SwVersion:        "1000000000",
			APIVersion:       "1.61.0",
			Mac:              mac.String(),
			BridgeID:         bridgeID,
			FactoryNew:       false,
			ModelID:          "BSB002",
			ZigbeeChannel:    25,
			LinkButton:       win.IsOpen(),
		}

		// GET /api/config has no {username} at all, and an unrecognized
		// {username} must fall back to the same stripped response — only a
		// recognized user gets the whitelist. Without it, some clients
		// (e.g. Hue Essentials) can't confirm their new username was
		// actually registered after POST /api succeeds, conclude pairing
		// failed, and restart the whole flow from scratch in a loop.
		if username := r.PathValue("username"); username != "" {
			if _, ok := wl.Lookup(username); ok {
				cfg.Whitelist = make(map[string]ConfigWhitelistEntry)
				for u, entry := range wl.All() {
					createDate := entry.CreateDate.UTC().Format("2006-01-02T15:04:05")
					cfg.Whitelist[u] = ConfigWhitelistEntry{
						Name:        entry.Name,
						CreateDate:  createDate,
						LastUseDate: createDate,
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}
