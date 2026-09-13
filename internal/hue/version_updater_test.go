package hue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLegacyAPIVersion(t *testing.T) {
	// Values taken from Bifrost's own test suite for get_legacy_apiversion,
	// which this ports: https://github.com/chrivers/bifrost.
	tests := []struct {
		version uint64
		want    string
	}{
		{12345, "1.34.0"},
		{1968096020, "1.68.0"},
		{1970084010, "1.70.0"},
	}
	for _, tt := range tests {
		if got := legacyAPIVersion(tt.version); got != tt.want {
			t.Errorf("legacyAPIVersion(%d) = %q, want %q", tt.version, got, tt.want)
		}
	}
}

func TestLegacySwVersion(t *testing.T) {
	if got := legacySwVersion(1968096020); got != "1968096020" {
		t.Errorf("legacySwVersion(1968096020) = %q, want 1968096020", got)
	}
}

func TestFetchLatestSignifyVersion_PicksHighest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("deviceTypeId"); got != bridgeModelID {
			t.Errorf("got deviceTypeId=%q, want %q", got, bridgeModelID)
		}
		fmt.Fprint(w, `{"updates":[{"version":1949203030},{"version":1970084010},{"version":1960041030}]}`)
	}))
	defer srv.Close()

	got, err := fetchLatestSignifyVersion(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchLatestSignifyVersion: %v", err)
	}
	if got != 1970084010 {
		t.Fatalf("got version=%d, want 1970084010 (the highest of the three)", got)
	}
}

func TestFetchLatestSignifyVersion_NoUpdatesIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"updates":[]}`)
	}))
	defer srv.Close()

	if _, err := fetchLatestSignifyVersion(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("got nil error for an empty updates list, want an error")
	}
}

func TestFetchLatestSignifyVersion_NonOKStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := fetchLatestSignifyVersion(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("got nil error for a 503 response, want an error")
	}
}

func TestVersionUpdater_RefreshAppliesFetchedVersion(t *testing.T) {
	origSw, origAPI, origSwOverridden, origAPIOverridden := currentSwVersion, currentAPIVersion, swVersionOverridden, apiVersionOverridden
	t.Cleanup(func() {
		currentSwVersion, currentAPIVersion = origSw, origAPI
		swVersionOverridden, apiVersionOverridden = origSwOverridden, origAPIOverridden
	})
	swVersionOverridden, apiVersionOverridden = false, false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"updates":[{"version":1970084010}]}`)
	}))
	defer srv.Close()

	u := newTestVersionUpdater(srv)
	if err := u.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if currentSwVersion != "1970084010" {
		t.Fatalf("got currentSwVersion=%q, want 1970084010", currentSwVersion)
	}
	if currentAPIVersion != "1.70.0" {
		t.Fatalf("got currentAPIVersion=%q, want 1.70.0", currentAPIVersion)
	}
}

func TestVersionUpdater_RefreshRespectsOverrides(t *testing.T) {
	origSw, origAPI, origSwOverridden, origAPIOverridden := currentSwVersion, currentAPIVersion, swVersionOverridden, apiVersionOverridden
	t.Cleanup(func() {
		currentSwVersion, currentAPIVersion = origSw, origAPI
		swVersionOverridden, apiVersionOverridden = origSwOverridden, origAPIOverridden
	})
	currentSwVersion, currentAPIVersion = "1949203030", "1.49.0"
	swVersionOverridden, apiVersionOverridden = true, true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"updates":[{"version":1970084010}]}`)
	}))
	defer srv.Close()

	u := newTestVersionUpdater(srv)
	if err := u.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if currentSwVersion != "1949203030" {
		t.Fatalf("got currentSwVersion=%q, want the pinned override 1949203030 to survive the fetch", currentSwVersion)
	}
	if currentAPIVersion != "1.49.0" {
		t.Fatalf("got currentAPIVersion=%q, want the pinned override 1.49.0 to survive the fetch", currentAPIVersion)
	}
}

func TestVersionUpdater_RefreshIfExpired_SkipsWithinCacheTTL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"updates":[{"version":1970084010}]}`)
	}))
	defer srv.Close()

	u := newTestVersionUpdater(srv)
	u.lastFetch = time.Now()

	u.refreshIfExpired(context.Background())
	if calls != 0 {
		t.Fatalf("got %d fetches within the cache TTL, want 0", calls)
	}

	u.lastFetch = time.Now().Add(-versionCacheTTL - time.Minute)
	u.refreshIfExpired(context.Background())
	if calls != 1 {
		t.Fatalf("got %d fetches once the cache expired, want 1", calls)
	}
}

// newTestVersionUpdater returns a VersionUpdater pointed at srv instead of
// the real Signify endpoint.
func newTestVersionUpdater(srv *httptest.Server) *VersionUpdater {
	u := NewVersionUpdater(srv.Client())
	u.baseURL = srv.URL
	return u
}
