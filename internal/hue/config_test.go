package hue

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
	"testing"
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
