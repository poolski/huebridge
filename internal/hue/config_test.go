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
