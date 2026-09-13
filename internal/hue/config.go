package hue

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// configTimeFormat matches the real bridge's UTC/localtime and whitelist
// date wire format exactly: no 'Z' or offset suffix. See
// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config".
const configTimeFormat = "2006-01-02T15:04:05"

// currentDatastoreVersion/currentSwVersion/currentAPIVersion were bumped to
// match a real BSB002 bridge's *current* firmware (confirmed live against
// GET /api/<user>/config), on the theory that an old-looking version might
// prompt a firmware push (see noUpdatesAvailable). That didn't fix the
// official app's refusal to poll for pairing, and diyHue — long known to
// work with the official app — reports much older-looking defaults
// (swversion 1967054020, apiversion 1.67.0) despite being actively
// maintained. Reverted to the older placeholder values pending evidence
// that a specific version actually matters here.
const (
	currentDatastoreVersion = "126"
	currentSwVersion        = "1000000000"
	currentAPIVersion       = "1.61.0"
)

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
			DatastoreVersion: currentDatastoreVersion,
			SwVersion:        currentSwVersion,
			APIVersion:       currentAPIVersion,
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
func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow, wl *Whitelist, ip string) http.HandlerFunc {
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
			// lastaccesstype isn't tracked at all — "none" is what a real
			// bridge reports for an entry it hasn't classified.
			created := entry.CreateDate.Format(configTimeFormat)
			whitelist[username] = ConfigWhitelistEntry{
				CreateDate:     created,
				LastUseDate:    created,
				Name:           entry.Name,
				LastAccessType: "none",
			}
		}

		swUpdate, swUpdate2 := noUpdatesAvailable()
		now := time.Now()
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: currentDatastoreVersion,
			SwVersion:        currentSwVersion,
			APIVersion:       currentAPIVersion,
			Mac:              mac.String(),
			BridgeID:         bridgeID,
			FactoryNew:       false,
			ReplacesBridgeID: nil,
			ModelID:          "BSB002",
			StarterKitID:     "",
			ZigbeeChannel:    25,
			LinkButton:       win.IsOpen(),
			UTC:              now.UTC().Format(configTimeFormat),
			LocalTime:        now.Format(configTimeFormat),
			Dhcp:             true,
			IPAddress:        ip,
			// Deprecated since bridge firmware 1.21/1.37 — a real bridge
			// always reports these two values now, never anything else.
			ProxyAddress: "none",
			ProxyPort:    0,
			SwUpdate:     swUpdate,
			SwUpdate2:    swUpdate2,
			Whitelist:    whitelist,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

// updaterCapturePath is a TEMPORARY debug capture location for reverse
// engineering the firmware-push format POST /updater receives — see
// handleUpdater. Remove capture once that's understood.
const updaterCapturePath = "/tmp/huebridge-updater-capture.bin"

// handleUpdater serves POST /updater — a real bridge accepts an app-pushed
// firmware file here. huebridge has nothing to install; accept and ignore
// it rather than 404, since noUpdatesAvailable() should mean this is only
// ever hit by a user-forced "check for update" rather than app-driven
// behavior. Its response shape is undocumented (not observed against a
// real bridge), so this just answers 200 with an empty body.
//
// TEMPORARY: also captures the raw request body to updaterCapturePath so
// its firmware-container format can be inspected. Remove this capture
// once that's done — it's not something a normal install should carry.
func handleUpdater() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f, err := os.Create(updaterCapturePath); err != nil {
			log.Printf("capture /updater body: create %s: %v", updaterCapturePath, err)
		} else {
			n, err := io.Copy(f, r.Body)
			f.Close()
			if err != nil {
				log.Printf("capture /updater body: %v", err)
			} else {
				log.Printf("captured /updater body: %d bytes to %s (Content-Type: %s)", n, updaterCapturePath, r.Header.Get("Content-Type"))
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
