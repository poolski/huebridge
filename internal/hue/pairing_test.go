package hue

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func mustParseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return mac
}

func TestPairing_RejectedWithoutOpenWindow(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "whitelist.json"))
	win := &PairingWindow{}
	srv := NewServer(nil, nil, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil)

	req := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"devicetype":"test#app"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []map[string]map[string]any
	json.NewDecoder(rec.Body).Decode(&body)

	if len(body) != 1 || body[0]["error"] == nil {
		t.Fatalf("got %+v, want a single error entry", body)
	}
	if int(body[0]["error"]["type"].(float64)) != 101 {
		t.Fatalf("got error type %v, want 101 (link button not pressed)", body[0]["error"]["type"])
	}
}

func TestPairing_SucceedsWithOpenWindow(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "whitelist.json"))
	win := &PairingWindow{}
	win.Open(30 * timeSecond)
	srv := NewServer(nil, nil, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil)

	req := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"devicetype":"test#app"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []map[string]map[string]any
	json.NewDecoder(rec.Body).Decode(&body)

	if len(body) != 1 || body[0]["success"] == nil {
		t.Fatalf("got %+v, want a single success entry", body)
	}
	username, _ := body[0]["success"]["username"].(string)
	if username == "" {
		t.Fatal("expected a non-empty username in the success response")
	}

	if _, ok := wl.Lookup(username); !ok {
		t.Fatalf("username %q was not persisted to the whitelist", username)
	}
}

func TestPairing_GeneratesClientKeyWhenRequested(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "whitelist.json"))
	win := &PairingWindow{}
	win.Open(30 * timeSecond)
	srv := NewServer(nil, nil, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil)

	req := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"devicetype":"test#app","generateclientkey":true}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []map[string]map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0]["success"] == nil {
		t.Fatalf("got %+v, want a single success entry", body)
	}
	clientkey, _ := body[0]["success"]["clientkey"].(string)
	if len(clientkey) != 32 {
		t.Fatalf("got clientkey=%q (len %d), want a 32-character hex string", clientkey, len(clientkey))
	}
	if clientkey != strings.ToUpper(clientkey) {
		t.Fatalf("got clientkey=%q, want uppercase", clientkey)
	}
}

func TestPairing_OmitsClientKeyWhenNotRequested(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "whitelist.json"))
	win := &PairingWindow{}
	win.Open(30 * timeSecond)
	srv := NewServer(nil, nil, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil)

	req := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"devicetype":"test#app"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var body []map[string]map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if _, ok := body[0]["success"]["clientkey"]; ok {
		t.Fatalf("got clientkey in response, want it omitted when generateclientkey wasn't set")
	}
}

// Some clients (Hue Essentials among them) POST to /api/ with a trailing
// slash rather than /api; a real bridge accepts both.
func TestPairing_SucceedsWithTrailingSlash(t *testing.T) {
	wl := NewWhitelist(filepath.Join(t.TempDir(), "whitelist.json"))
	win := &PairingWindow{}
	win.Open(30 * timeSecond)
	srv := NewServer(nil, nil, wl, win, "AABBCCFFFEDDEEFF", mustParseMAC("aa:bb:cc:dd:ee:ff"), nil, nil)

	req := httptest.NewRequest("POST", "/api/", bytes.NewReader([]byte(`{"devicetype":"test#app"}`)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	var body []map[string]map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0]["success"] == nil {
		t.Fatalf("got %+v, want a single success entry", body)
	}
}
