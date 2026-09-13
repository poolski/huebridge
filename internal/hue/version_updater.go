package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// signifyUpdateCheckURL is the real bridge firmware's own update-check
// endpoint. The diyHue community found querying it directly is the only way
// to get a swversion/apiversion pair the official app currently accepts —
// see docs/superpowers/notes/2026-09-13-tls-pairing-failure-log.md for the
// history of a hardcoded pair drifting stale. Bifrost (a Rust Hue bridge
// emulator, https://github.com/chrivers/bifrost) queries this same endpoint
// the same way; VersionUpdater below ports its approach.
const signifyUpdateCheckURL = "https://firmware.meethue.com/v1/checkupdate"

// bridgeModelID is the deviceTypeId Signify's update-check endpoint expects
// for this hardware — matches PublicBridgeConfig/BridgeConfig's ModelID.
const bridgeModelID = "BSB002"

// versionCacheTTL matches Bifrost's VersionUpdater: Signify's update list
// doesn't change more than about once a day, so cache a successful fetch
// for this long instead of querying on every check.
const versionCacheTTL = 24 * time.Hour

// versionFetchTimeout bounds every request to Signify so an unreachable or
// slow server can't hang startup or the periodic refresh loop.
const versionFetchTimeout = 10 * time.Second

type signifyUpdateEntry struct {
	Version uint64 `json:"version"`
}

type signifyUpdateResponse struct {
	Updates []signifyUpdateEntry `json:"updates"`
}

// fetchLatestSignifyVersion queries baseURL (Signify's update-check
// endpoint in production, a test server in tests) for bridgeModelID and
// returns the highest numeric version it lists, matching Bifrost's
// fetch_version.
func fetchLatestSignifyVersion(ctx context.Context, client *http.Client, baseURL string) (uint64, error) {
	url := fmt.Sprintf("%s?deviceTypeId=%s&version=0", baseURL, bridgeModelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("signify update check: unexpected status %s", resp.Status)
	}
	var parsed signifyUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, fmt.Errorf("decode signify update response: %w", err)
	}
	var (
		maxVersion uint64
		found      bool
	)
	for _, u := range parsed.Updates {
		if !found || u.Version > maxVersion {
			maxVersion, found = u.Version, true
		}
	}
	if !found {
		return 0, fmt.Errorf("signify update check: no updates listed for deviceTypeId=%s", bridgeModelID)
	}
	return maxVersion, nil
}

// legacySwVersion mirrors Bifrost's SwVersion::get_legacy_swversion: the
// numeric firmware version reported as-is.
func legacySwVersion(version uint64) string {
	return strconv.FormatUint(version, 10)
}

// legacyAPIVersion mirrors Bifrost's SwVersion::get_legacy_apiversion: a
// real bridge's dotted apiversion isn't tracked independently — it's
// derived from specific digits of the numeric firmware version. E.g.
// 1968096020, zero-padded to at least 5 digits, becomes "1.68.0" from digit
// 0 and digits 2-3.
func legacyAPIVersion(version uint64) string {
	padded := fmt.Sprintf("%05d", version)
	return fmt.Sprintf("%s.%s.0", padded[0:1], padded[2:4])
}

// VersionUpdater periodically queries Signify for the newest
// swversion/apiversion pair and applies it via setDiscoveredVersion,
// replicating Bifrost's VersionUpdater: the compiled-in fallback (see
// config.go) is served immediately so pairing never waits on a network
// round trip, then this refreshes in the background and caches a
// successful fetch for versionCacheTTL.
type VersionUpdater struct {
	client  *http.Client
	baseURL string // overridden by tests to point at a test server

	mu        sync.Mutex
	lastFetch time.Time
}

// NewVersionUpdater returns a VersionUpdater using client, or
// http.DefaultClient if client is nil.
func NewVersionUpdater(client *http.Client) *VersionUpdater {
	if client == nil {
		client = http.DefaultClient
	}
	return &VersionUpdater{client: client, baseURL: signifyUpdateCheckURL}
}

// Refresh fetches the newest version from Signify and applies it via
// setDiscoveredVersion unconditionally, regardless of cache freshness —
// used for the initial fetch in Run and by POST /updater
// (refreshVersionNow/handleUpdater), matching Bifrost's post_updater
// forcing an immediate recheck instead of waiting out the cache.
func (u *VersionUpdater) Refresh(ctx context.Context) error {
	version, err := fetchLatestSignifyVersion(ctx, u.client, u.baseURL)
	if err != nil {
		return err
	}
	sw, api := legacySwVersion(version), legacyAPIVersion(version)

	u.mu.Lock()
	u.lastFetch = time.Now()
	u.mu.Unlock()

	setDiscoveredVersion(sw, api)
	log.Printf("hue: discovered swversion=%s apiversion=%s from Signify", sw, api)
	return nil
}

// refreshIfExpired refetches only once versionCacheTTL has elapsed since
// the last successful fetch, so Run's periodic tick doesn't hit Signify's
// servers more than necessary.
func (u *VersionUpdater) refreshIfExpired(ctx context.Context) {
	u.mu.Lock()
	expired := time.Since(u.lastFetch) > versionCacheTTL
	u.mu.Unlock()
	if !expired {
		return
	}
	if err := u.Refresh(ctx); err != nil {
		log.Printf("hue: refresh Signify firmware version: %v", err)
	}
}

// Run performs an initial best-effort fetch (bounded by
// versionFetchTimeout, so an unreachable Signify never blocks the caller
// for long) and then loops, rechecking hourly whether the cache has
// expired. Meant to be run in its own goroutine; returns when ctx is done.
func (u *VersionUpdater) Run(ctx context.Context) {
	fetchCtx, cancel := context.WithTimeout(ctx, versionFetchTimeout)
	if err := u.Refresh(fetchCtx); err != nil {
		log.Printf("hue: initial Signify firmware version fetch failed, using fallback: %v", err)
	}
	cancel()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, versionFetchTimeout)
			u.refreshIfExpired(tickCtx)
			cancel()
		}
	}
}

// active is the process's single running VersionUpdater, set by
// StartVersionUpdater and consulted by refreshVersionNow (handleUpdater) to
// force an immediate recheck. Like SetVersionOverrides, meant to be set at
// most once, at startup, before the server accepts connections.
var active *VersionUpdater

// StartVersionUpdater creates a VersionUpdater, registers it so POST
// /updater (handleUpdater) can force a refresh, and runs it in a new
// goroutine until ctx is done.
func StartVersionUpdater(ctx context.Context, client *http.Client) {
	u := NewVersionUpdater(client)
	active = u
	go u.Run(ctx)
}

// refreshVersionNow forces the active VersionUpdater, if any, to recheck
// Signify immediately in the background — a no-op if StartVersionUpdater
// was never called (e.g. in tests).
func refreshVersionNow() {
	if active == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), versionFetchTimeout)
		defer cancel()
		if err := active.Refresh(ctx); err != nil {
			log.Printf("hue: refresh Signify firmware version on demand: %v", err)
		}
	}()
}
