package hue

import (
	"encoding/json"
	"net"
	"net/http"
)

// currentDatastoreVersion/currentSwVersion/currentAPIVersion: the official
// Hue app validates this triple against a real released firmware
// combination and, if it doesn't recognize it, nags for an update that a
// non-genuine bridge can never actually deliver — see
// docs/superpowers/notes/2026-09-13-tls-pairing-failure-log.md for the full
// investigation. Bumping this triple to match a real live bridge's reported
// values did not unblock pairing (see
// docs/superpowers/notes/2026-09-13-android-emulator-repro-log.md, Finding 2),
// so these are reverted to the original defaults pending further
// investigation. Overridable at runtime via SetVersionOverrides.
var (
	currentDatastoreVersion = "126"
	currentSwVersion        = "1000000000"
	currentAPIVersion       = "1.61.0"
)

// SetVersionOverrides replaces currentDatastoreVersion/currentSwVersion/
// currentAPIVersion with any non-empty argument, leaving the corresponding
// default in place otherwise. Meant to be called at most once, at startup,
// before the server accepts connections — these three aren't behind a
// mutex, so mutating them after that point is a data race.
func SetVersionOverrides(datastoreVersion, swVersion, apiVersion string) {
	if datastoreVersion != "" {
		currentDatastoreVersion = datastoreVersion
	}
	if swVersion != "" {
		currentSwVersion = swVersion
	}
	if apiVersion != "" {
		currentAPIVersion = apiVersion
	}
}

func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow, wl *Whitelist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: currentDatastoreVersion,
			SwVersion:        currentSwVersion,
			APIVersion:       currentAPIVersion,
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
