package hue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"huebridge/internal/store"
)

const timeSecond = time.Second

// PairingWindow tracks whether "allow next pairing" is currently active,
// standing in for the physical link button. Opened from the ingress UI.
type PairingWindow struct {
	mu       sync.Mutex
	deadline time.Time
}

func (p *PairingWindow) Open(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deadline = time.Now().Add(d)
}

func (p *PairingWindow) IsOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Now().Before(p.deadline)
}

type WhitelistEntry struct {
	Username   string    `json:"username"`
	Name       string    `json:"name"`
	CreateDate time.Time `json:"create_date"`
}

type whitelistFile struct {
	Entries map[string]WhitelistEntry `json:"entries"`
}

type Whitelist struct {
	mu    sync.Mutex
	file  *store.JSONFile[whitelistFile]
	state whitelistFile
}

func NewWhitelist(path string) *Whitelist {
	file := store.NewJSONFile[whitelistFile](path)
	state, _ := file.Load(whitelistFile{Entries: map[string]WhitelistEntry{}})
	if state.Entries == nil {
		state.Entries = map[string]WhitelistEntry{}
	}
	return &Whitelist{file: file, state: state}
}

func (wl *Whitelist) Add(entry WhitelistEntry) error {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	wl.state.Entries[entry.Username] = entry
	return wl.file.Save(wl.state)
}

func (wl *Whitelist) Lookup(username string) (WhitelistEntry, bool) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	e, ok := wl.state.Entries[username]
	return e, ok
}

// All returns a copy of every whitelisted entry, keyed by username.
func (wl *Whitelist) All() map[string]WhitelistEntry {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	entries := make(map[string]WhitelistEntry, len(wl.state.Entries))
	for username, entry := range wl.state.Entries {
		entries[username] = entry
	}
	return entries
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func handlePairing(wl *Whitelist, win *PairingWindow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DeviceType string `json:"devicetype"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.DeviceType) == "" {
			WriteError(w, http.StatusOK, 6, "/api/devicetype", "parameter, devicetype, not available")
			return
		}

		if !win.IsOpen() {
			WriteError(w, http.StatusOK, 101, "/api/", "link button not pressed")
			return
		}

		username := randomHex(16)
		if err := wl.Add(WhitelistEntry{Username: username, Name: req.DeviceType, CreateDate: time.Now()}); err != nil {
			WriteError(w, http.StatusInternalServerError, 901, "/api/", "internal error")
			return
		}

		WriteSuccess(w, map[string]any{"username": username})
	}
}
