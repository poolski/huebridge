package hue

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// configTimeFormat matches the real bridge's UTC/localtime and whitelist
// date wire format exactly: no 'Z' or offset suffix. See
// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config".
const configTimeFormat = "2006-01-02T15:04:05"

// currentDatastoreVersion/currentSwVersion/currentAPIVersion default to the
// diyHue community's maintainer-confirmed fallback pair
// (https://diyhue.discourse.group/t/app-requires-update-of-hue-bridge-update-fails/233,
// see also docs/superpowers/notes/2026-09-13-tls-pairing-failure-log.md) —
// used until VersionUpdater's background fetch from Signify (or
// SetVersionOverrides) replaces swVersion/apiVersion with a currently valid
// pair. versionMu guards all three: VersionUpdater writes swVersion/
// apiVersion from a background goroutine while request handlers read them
// concurrently.
var (
	versionMu               sync.RWMutex
	currentDatastoreVersion = "126"
	currentSwVersion        = "1949203030"
	currentAPIVersion       = "1.49.0"

	// swVersionOverridden/apiVersionOverridden record whether
	// SetVersionOverrides pinned a field, so VersionUpdater's background
	// fetches (setDiscoveredVersion) never silently clobber an explicit
	// override.
	swVersionOverridden  bool
	apiVersionOverridden bool
)

// SetVersionOverrides replaces currentDatastoreVersion/currentSwVersion/
// currentAPIVersion with any non-empty argument, leaving the corresponding
// default (or a later VersionUpdater fetch, for datastoreVersion only) in
// place otherwise. An overridden swVersion/apiVersion is pinned: it's
// exempt from VersionUpdater's background updates from here on.
func SetVersionOverrides(datastoreVersion, swVersion, apiVersion string) {
	versionMu.Lock()
	defer versionMu.Unlock()
	if datastoreVersion != "" {
		currentDatastoreVersion = datastoreVersion
	}
	if swVersion != "" {
		currentSwVersion = swVersion
		swVersionOverridden = true
	}
	if apiVersion != "" {
		currentAPIVersion = apiVersion
		apiVersionOverridden = true
	}
}

// setDiscoveredVersion applies a swVersion/apiVersion pair VersionUpdater
// fetched from Signify, skipping any field SetVersionOverrides pinned.
func setDiscoveredVersion(swVersion, apiVersion string) {
	versionMu.Lock()
	defer versionMu.Unlock()
	if !swVersionOverridden {
		currentSwVersion = swVersion
	}
	if !apiVersionOverridden {
		currentAPIVersion = apiVersion
	}
}

// APIVersion returns the currently reported apiversion, reflecting any
// SetVersionOverrides call or VersionUpdater fetch — used by
// discovery.StartSSDP so its SERVER header agrees with the CLIP API instead
// of carrying an independent, driftable version string.
func APIVersion() string {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return currentAPIVersion
}

// currentVersions returns a consistent snapshot of the datastore/software/
// API versions for the config handlers to serve, guarding against a
// concurrent SetVersionOverrides call or VersionUpdater fetch.
func currentVersions() (datastoreVersion, swVersion, apiVersion string) {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return currentDatastoreVersion, currentSwVersion, currentAPIVersion
}

// lastInstallDate is computed once at process start, not per-request — a
// real bridge's last-install timestamp is a fixed past event, and
// recomputing "7 days ago" on every single request made it creep forward
// every second, which is exactly the kind of never-settling state a
// client polling for update completion would keep waiting on.
var lastInstallDate = time.Now().Add(-7 * 24 * time.Hour).Format(configTimeFormat)

// noUpdatesAvailable reports the real bridge's "nothing to install" shape
// for both the legacy and current software-update status objects. A bare
// state:"noupdates" wasn't enough on its own — a live bridge also reports
// autoinstall enabled and a real past lastchange/lastinstall date, and
// leaving those at their zero values (no update history at all) has been
// observed prompting the official app to push a firmware update right
// after pairing succeeds.
func noUpdatesAvailable() (SwUpdate, SwUpdate2) {
	return SwUpdate{
		DeviceTypes: SwUpdateDeviceTypes{Lights: []string{}, Sensors: []string{}},
	}, SwUpdate2{
		AutoInstall: SwUpdate2AutoInstall{On: true, UpdateTime: "T04:00:00"},
		Bridge:      SwUpdate2Bridge{State: "noupdates", LastInstall: lastInstallDate},
		LastChange:  lastInstallDate,
		State:       "noupdates",
	}
}

// handleGetPublicConfig serves GET /api/config, the unauthenticated
// bridge-identification endpoint apps probe (with no username at all)
// before pairing, e.g. via "Manual bridge setup" in the Hue app.
func handleGetPublicConfig(bridgeID string, mac net.HardwareAddr) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		datastoreVersion, swVersion, apiVersion := currentVersions()
		cfg := PublicBridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: datastoreVersion,
			SwVersion:        swVersion,
			APIVersion:       apiVersion,
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
		datastoreVersion, swVersion, apiVersion := currentVersions()
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: datastoreVersion,
			SwVersion:        swVersion,
			APIVersion:       apiVersion,
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
//
// It also forces VersionUpdater to recheck Signify immediately rather than
// waiting out its cache, matching Bifrost's post_updater: this route is
// exactly what the official app hits for a user-triggered "check for
// update", so it's the moment a live answer matters most.
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
		refreshVersionNow()
		w.WriteHeader(http.StatusOK)
	}
}
