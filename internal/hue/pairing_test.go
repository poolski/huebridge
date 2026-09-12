package hue

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http/httptest"
	"path/filepath"
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
