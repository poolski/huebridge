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
	if cfg.APIVersion != "1.61.0" {
		t.Fatalf("got APIVersion=%q, want 1.61.0", cfg.APIVersion)
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
