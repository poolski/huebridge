package hue

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_GetAuthenticated(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/testuser/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var cfg BridgeConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.BridgeID != "AABBCCFFFEDDEEFF" {
		t.Fatalf("got BridgeID=%q, want AABBCCFFFEDDEEFF", cfg.BridgeID)
	}
	if cfg.Mac != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("got Mac=%q, want aa:bb:cc:dd:ee:ff", cfg.Mac)
	}
	if cfg.APIVersion != currentAPIVersion {
		t.Fatalf("got APIVersion=%q, want %q", cfg.APIVersion, currentAPIVersion)
	}
	if cfg.Whitelist != nil {
		t.Fatalf("got Whitelist=%v for an unrecognized user, want nil", cfg.Whitelist)
	}
}

// TestConfig_GetAuthenticated_IncludesWhitelistForPairedUser guards against a
// pairing loop some clients (e.g. Hue Essentials) hit: after POST /api
// succeeds, they GET .../config to confirm the new username was actually
// registered by checking it appears in the whitelist. If it's missing, they
// conclude pairing failed and restart the whole flow from scratch.
func TestConfig_GetAuthenticated_IncludesWhitelistForPairedUser(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	if err := wl.Add(WhitelistEntry{Username: "paireduser", Name: "Hue#iPhone"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/paireduser/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var cfg BridgeConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entry, ok := cfg.Whitelist["paireduser"]
	if !ok {
		t.Fatalf("expected whitelist to contain paireduser, got %v", cfg.Whitelist)
	}
	if entry.Name != "Hue#iPhone" {
		t.Fatalf("got whitelist entry name=%q, want Hue#iPhone", entry.Name)
	}
}

// TestSetVersionOverrides guards the "unset env var leaves the default
// untouched" behavior the official Hue app's update-nag bug depends on: see
// docs/superpowers/notes/2026-09-13-tls-pairing-failure-log.md.
func TestSetVersionOverrides(t *testing.T) {
	origDatastore, origSw, origAPI := currentDatastoreVersion, currentSwVersion, currentAPIVersion
	t.Cleanup(func() {
		currentDatastoreVersion, currentSwVersion, currentAPIVersion = origDatastore, origSw, origAPI
	})

	SetVersionOverrides("", "", "")
	if currentDatastoreVersion != origDatastore || currentSwVersion != origSw || currentAPIVersion != origAPI {
		t.Fatalf("empty overrides changed defaults: got (%q, %q, %q), want (%q, %q, %q)",
			currentDatastoreVersion, currentSwVersion, currentAPIVersion, origDatastore, origSw, origAPI)
	}

	SetVersionOverrides("197", "1978293000", "1.78.0")
	if currentDatastoreVersion != "197" || currentSwVersion != "1978293000" || currentAPIVersion != "1.78.0" {
		t.Fatalf("got (%q, %q, %q), want (\"197\", \"1978293000\", \"1.78.0\")",
			currentDatastoreVersion, currentSwVersion, currentAPIVersion)
	}
}

func TestSetTimezoneOverride(t *testing.T) {
	orig := currentTimezone
	t.Cleanup(func() { currentTimezone = orig })

	SetTimezoneOverride("")
	if currentTimezone != orig {
		t.Fatalf("empty override changed the default: got %q, want %q", currentTimezone, orig)
	}

	SetTimezoneOverride("America/New_York")
	if currentTimezone != "America/New_York" {
		t.Fatalf("got %q, want America/New_York", currentTimezone)
	}
}

func TestVersionStore_SetRejectsUnknownVersion(t *testing.T) {
	origDatastore, origSw, origAPI := currentDatastoreVersion, currentSwVersion, currentAPIVersion
	t.Cleanup(func() {
		currentDatastoreVersion, currentSwVersion, currentAPIVersion = origDatastore, origSw, origAPI
	})

	vs := NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	err := vs.Set(VersionTriple{DatastoreVersion: "1", SwVersion: "not-a-real-build", APIVersion: "9.9.9"})
	if err == nil {
		t.Fatal("expected Set to reject a version triple not in KnownVersions")
	}
	if currentDatastoreVersion != origDatastore || currentSwVersion != origSw || currentAPIVersion != origAPI {
		t.Fatal("a rejected Set must not change the currently-reported version")
	}
}

func TestVersionStore_SetPersistsAndAppliesAKnownVersion(t *testing.T) {
	origDatastore, origSw, origAPI := currentDatastoreVersion, currentSwVersion, currentAPIVersion
	t.Cleanup(func() {
		currentDatastoreVersion, currentSwVersion, currentAPIVersion = origDatastore, origSw, origAPI
	})

	want := KnownVersions[0]
	path := filepath.Join(t.TempDir(), "version.json")
	vs := NewVersionStore(path)
	if err := vs.Set(want); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := vs.Current(); got != want {
		t.Fatalf("got Current()=%+v, want %+v", got, want)
	}

	// A fresh store loading from the same file picks up the persisted
	// selection, so it survives a restart.
	currentDatastoreVersion, currentSwVersion, currentAPIVersion = origDatastore, origSw, origAPI
	reloaded := NewVersionStore(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := currentVersion(); got != want {
		t.Fatalf("after Load(), got current version %+v, want %+v", got, want)
	}
}

func TestVersionStore_LoadWithoutPriorSetLeavesDefaultsUntouched(t *testing.T) {
	origDatastore, origSw, origAPI := currentDatastoreVersion, currentSwVersion, currentAPIVersion
	t.Cleanup(func() {
		currentDatastoreVersion, currentSwVersion, currentAPIVersion = origDatastore, origSw, origAPI
	})

	vs := NewVersionStore(filepath.Join(t.TempDir(), "version.json"))
	if err := vs.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if currentDatastoreVersion != origDatastore || currentSwVersion != origSw || currentAPIVersion != origAPI {
		t.Fatal("Load with no persisted file must not change the defaults")
	}
}

func TestTimezoneStore_SetRejectsUnrecognizedTimezone(t *testing.T) {
	orig := currentTimezone
	t.Cleanup(func() { currentTimezone = orig })

	ts := NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	if err := ts.Set("Not/A_Real_Zone"); err == nil {
		t.Fatal("expected Set to reject a timezone name that isn't loadable")
	}
	if currentTimezone != orig {
		t.Fatal("a rejected Set must not change the currently-reported timezone")
	}
}

func TestTimezoneStore_SetPersistsAndAppliesATimezone(t *testing.T) {
	orig := currentTimezone
	t.Cleanup(func() { currentTimezone = orig })

	want := "America/New_York"
	path := filepath.Join(t.TempDir(), "timezone.json")
	ts := NewTimezoneStore(path)
	if err := ts.Set(want); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := ts.Current(); got != want {
		t.Fatalf("got Current()=%q, want %q", got, want)
	}

	// A fresh store loading from the same file picks up the persisted
	// selection, so it survives a restart.
	currentTimezone = orig
	reloaded := NewTimezoneStore(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := currentTZ(); got != want {
		t.Fatalf("after Load(), got current timezone %q, want %q", got, want)
	}
}

func TestTimezoneStore_LoadWithoutPriorSetLeavesDefaultUntouched(t *testing.T) {
	orig := currentTimezone
	t.Cleanup(func() { currentTimezone = orig })

	ts := NewTimezoneStore(filepath.Join(t.TempDir(), "timezone.json"))
	if err := ts.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if currentTimezone != orig {
		t.Fatal("Load with no persisted file must not change the default")
	}
}

func TestConfig_GetUnauthenticated_HasNoWhitelist(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	if err := wl.Add(WhitelistEntry{Username: "paireduser", Name: "Hue#iPhone"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var cfg BridgeConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Whitelist != nil {
		t.Fatalf("got Whitelist=%v for the unauthenticated route, want nil", cfg.Whitelist)
	}
}

// TestConfig_GetUnauthenticated_ReturnsExactStrippedFieldSet guards the
// real-bridge behavior documented in
// docs/superpowers/specs/hue-clip-v1-api-reference.md ("Config"): the
// unauthenticated/unrecognized-user response is a fixed 10-field subset,
// with no network details, portal/backup/swupdate state, or whitelist.
func TestConfig_GetUnauthenticated_ReturnsExactStrippedFieldSet(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var raw map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := []string{
		"name", "datastoreversion", "swversion", "apiversion", "mac",
		"bridgeid", "factorynew", "replacesbridgeid", "modelid", "starterkitid",
	}
	if len(raw) != len(want) {
		t.Fatalf("got %d fields %v, want exactly the %d fields %v", len(raw), raw, len(want), want)
	}
	for _, k := range want {
		if _, ok := raw[k]; !ok {
			t.Fatalf("missing expected field %q in %v", k, raw)
		}
	}
}

// TestConfig_GetAuthenticated_ReturnsFullFieldSet guards the full-response
// shape a recognized user gets, per the same spec's authenticated example.
func TestConfig_GetAuthenticated_ReturnsFullFieldSet(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	if err := wl.Add(WhitelistEntry{Username: "paireduser", Name: "Hue#iPhone"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/paireduser/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var raw map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	wantTopLevel := []string{
		"name", "zigbeechannel", "bridgeid", "mac", "dhcp", "ipaddress", "netmask",
		"gateway", "proxyaddress", "proxyport", "UTC", "localtime", "timezone",
		"modelid", "datastoreversion", "swversion", "apiversion", "swupdate2",
		"linkbutton", "portalservices", "analyticsconsent", "portalconnection",
		"portalstate", "internetservices", "factorynew", "replacesbridgeid",
		"starterkitid", "backup", "httpblocked", "whitelist",
	}
	for _, k := range wantTopLevel {
		if _, ok := raw[k]; !ok {
			t.Fatalf("missing expected field %q in %v", k, raw)
		}
	}

	swupdate2, ok := raw["swupdate2"].(map[string]interface{})
	if !ok {
		t.Fatalf("swupdate2 is not an object: %v", raw["swupdate2"])
	}
	for _, k := range []string{"checkforupdate", "lastchange", "bridge", "state", "autoinstall"} {
		if _, ok := swupdate2[k]; !ok {
			t.Fatalf("missing swupdate2.%s in %v", k, swupdate2)
		}
	}

	portalstate, ok := raw["portalstate"].(map[string]interface{})
	if !ok {
		t.Fatalf("portalstate is not an object: %v", raw["portalstate"])
	}
	for _, k := range []string{"signedon", "incoming", "outgoing", "communication"} {
		if _, ok := portalstate[k]; !ok {
			t.Fatalf("missing portalstate.%s in %v", k, portalstate)
		}
	}

	internetservices, ok := raw["internetservices"].(map[string]interface{})
	if !ok {
		t.Fatalf("internetservices is not an object: %v", raw["internetservices"])
	}
	for _, k := range []string{"internet", "remoteaccess", "time", "swupdate"} {
		if _, ok := internetservices[k]; !ok {
			t.Fatalf("missing internetservices.%s in %v", k, internetservices)
		}
	}

	backup, ok := raw["backup"].(map[string]interface{})
	if !ok {
		t.Fatalf("backup is not an object: %v", raw["backup"])
	}
	for _, k := range []string{"status", "errorcode"} {
		if _, ok := backup[k]; !ok {
			t.Fatalf("missing backup.%s in %v", k, backup)
		}
	}

	if tz := raw["timezone"]; tz != "Europe/London" {
		t.Fatalf("got timezone=%v, want the default Europe/London", tz)
	}
}

// TestConfig_GetAuthenticated_UsesOverriddenTimezone guards
// SetTimezoneOverride actually taking effect in the served config, and that
// "localtime" is computed in that zone rather than just echoing "UTC".
func TestConfig_GetAuthenticated_UsesOverriddenTimezone(t *testing.T) {
	orig := currentTimezone
	t.Cleanup(func() { currentTimezone = orig })
	SetTimezoneOverride("Pacific/Kiritimati") // fixed UTC+14, no DST to complicate the assertion

	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	if err := wl.Add(WhitelistEntry{Username: "paireduser", Name: "Hue#iPhone"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil)

	req := httptest.NewRequest("GET", "/api/paireduser/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var cfg BridgeConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Timezone != "Pacific/Kiritimati" {
		t.Fatalf("got Timezone=%q, want Pacific/Kiritimati", cfg.Timezone)
	}
	wantUTC, err := time.Parse("2006-01-02T15:04:05", cfg.UTC)
	if err != nil {
		t.Fatalf("parse UTC=%q: %v", cfg.UTC, err)
	}
	gotLocal, err := time.Parse("2006-01-02T15:04:05", cfg.LocalTime)
	if err != nil {
		t.Fatalf("parse LocalTime=%q: %v", cfg.LocalTime, err)
	}
	if want := wantUTC.Add(14 * time.Hour); !gotLocal.Equal(want) {
		t.Fatalf("got LocalTime=%v, want UTC+14 (%v) for UTC=%v", gotLocal, want, wantUTC)
	}
}
