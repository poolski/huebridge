package hue

import (
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// configTimeFormat matches the real bridge's UTC/localtime and whitelist
// date wire format exactly: no 'Z' or offset suffix. See
// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config".
const configTimeFormat = "2006-01-02T15:04:05"

// noUpdatesAvailable reports the real bridge's "nothing to install" shape
// for both the legacy and current software-update status objects. Its
// absence has been observed causing clients to treat an update as
// available and attempt to push one to POST /updater, which huebridge has
// no real firmware to accept.
func noUpdatesAvailable() (SwUpdate, SwUpdate2) {
	return SwUpdate{
		DeviceTypes: SwUpdateDeviceTypes{Lights: []string{}, Sensors: []string{}},
	}, SwUpdate2{
		Bridge: SwUpdate2Bridge{State: "noupdates"},
		State:  "noupdates",
	}
}

// handleGetPublicConfig serves GET /api/config, the unauthenticated
// bridge-identification endpoint apps probe (with no username at all)
// before pairing, e.g. via "Manual bridge setup" in the Hue app.
func handleGetPublicConfig(bridgeID string, mac net.HardwareAddr) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := PublicBridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: "126",
			SwVersion:        "1000000000",
			APIVersion:       "1.61.0",
			Mac:              mac.String(),
			BridgeID:         bridgeID,
			FactoryNew:       false,
			ReplacesBridgeID: nil,
			ModelID:          "BSB002",
			StarterKitID:     "",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

// handleGetConfig serves GET /api/{username}/config. A recognized username
// gets the full config, including the whitelist; an unrecognized one gets
// the same stripped subset GET /api/config (no username at all) returns —
// matching a real bridge, and avoiding leaking the whitelist to a caller
// that was never actually paired.
func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow, wl *Whitelist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := wl.Lookup(r.PathValue("username")); !ok {
			handleGetPublicConfig(bridgeID, mac)(w, r)
			return
		}

		whitelist := map[string]ConfigWhitelistEntry{}
		for username, entry := range wl.All() {
			// last use date isn't tracked live (it would mean a disk write
			// on every authenticated request, undoing the point of the
			// state cache) — create date is a reasonable stand-in for it.
			created := entry.CreateDate.Format(configTimeFormat)
			whitelist[username] = ConfigWhitelistEntry{
				CreateDate:  created,
				LastUseDate: created,
				Name:        entry.Name,
			}
		}

		swUpdate, swUpdate2 := noUpdatesAvailable()
		now := time.Now()
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: "126",
			SwVersion:        "1000000000",
			APIVersion:       "1.61.0",
			Mac:              mac.String(),
			BridgeID:         bridgeID,
			FactoryNew:       false,
			ReplacesBridgeID: nil,
			ModelID:          "BSB002",
			ZigbeeChannel:    25,
			LinkButton:       win.IsOpen(),
			UTC:              now.UTC().Format(configTimeFormat),
			LocalTime:        now.Format(configTimeFormat),
			SwUpdate:         swUpdate,
			SwUpdate2:        swUpdate2,
			Whitelist:        whitelist,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

// handleUpdater serves POST /updater — a real bridge accepts an app-pushed
// firmware file here. huebridge has nothing to install; accept and ignore
// it rather than 404, since noUpdatesAvailable() should mean this is only
// ever hit by a user-forced "check for update" rather than app-driven
// behavior. Its response shape is undocumented (not observed against a
// real bridge), so this just answers 200 with an empty body.
func handleUpdater() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
