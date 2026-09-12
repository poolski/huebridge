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
	if err := wl.Add(WhitelistEntry{Username: "testuser", Name: "test#app", CreateDate: time.Now()}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil, "192.168.1.100")

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
		t.Fatalf("got APIVersion=%q, want %s", cfg.APIVersion, currentAPIVersion)
	}
	if cfg.IPAddress != "192.168.1.100" {
		t.Fatalf("got IPAddress=%q, want 192.168.1.100", cfg.IPAddress)
	}
	entry, ok := cfg.Whitelist["testuser"]
	if !ok {
		t.Fatalf("got Whitelist=%+v, want an entry for testuser", cfg.Whitelist)
	}
	if entry.Name != "test#app" {
		t.Fatalf("got Whitelist[testuser].Name=%q, want test#app", entry.Name)
	}
	if entry.LastAccessType != "none" {
		t.Fatalf("got Whitelist[testuser].LastAccessType=%q, want none", entry.LastAccessType)
	}
	if cfg.SwUpdate2.State != "noupdates" {
		t.Fatalf("got SwUpdate2.State=%q, want noupdates", cfg.SwUpdate2.State)
	}
}

func TestConfig_GetAuthenticatedUnrecognizedUsernameGetsStrippedConfig(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil, "192.168.1.100")

	req := httptest.NewRequest("GET", "/api/never-paired/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["whitelist"]; ok {
		t.Fatalf("response includes whitelist for an unrecognized username, want it stripped like GET /api/config")
	}
}

func TestConfig_GetPublicNoUsername(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "wl.json"))
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	srv := NewServer(nil, nil, wl, &PairingWindow{}, "AABBCCFFFEDDEEFF", mac, nil, nil, "192.168.1.100")

	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got status %d, want 200", rec.Code)
	}

	body := rec.Body.Bytes()

	var cfg PublicBridgeConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.BridgeID != "AABBCCFFFEDDEEFF" {
		t.Fatalf("got BridgeID=%q, want AABBCCFFFEDDEEFF", cfg.BridgeID)
	}
	if cfg.ReplacesBridgeID != nil {
		t.Fatalf("got ReplacesBridgeID=%v, want nil", cfg.ReplacesBridgeID)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if _, ok := raw["replacesbridgeid"]; !ok {
		t.Fatalf("replacesbridgeid key missing from response, want present with null value")
	}
	for _, forbidden := range []string{"whitelist", "linkbutton", "zigbeechannel"} {
		if _, ok := raw[forbidden]; ok {
			t.Fatalf("response includes %q, real bridges strip this from the unauthenticated config", forbidden)
		}
	}
}
