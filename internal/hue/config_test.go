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
}
