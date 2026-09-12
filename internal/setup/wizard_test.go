package setup

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func noDiscovery(time.Duration) ([]string, error) { return nil, nil }

func newTestClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return &http.Client{Jar: jar}
}

func TestWizardHappyPath(t *testing.T) {
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer ha.Close()

	store := NewStore(filepath.Join(t.TempDir(), "standalone.json"))
	w := NewWizard(store, noDiscovery)
	srv := httptest.NewServer(w.Handler())
	defer srv.Close()
	client := newTestClient(t)

	// Step 1: set the admin password.
	resp, err := client.PostForm(srv.URL+"/setup/password", url.Values{
		"password": {"hunter2"},
		"confirm":  {"hunter2"},
	})
	if err != nil {
		t.Fatalf("post password: %v", err)
	}
	resp.Body.Close()
	if resp.Request.URL.Path != "/setup/homeassistant" {
		t.Fatalf("after password step, ended up at %s, want /setup/homeassistant", resp.Request.URL.Path)
	}

	// Step 2: submit Home Assistant connection details.
	resp, err = client.PostForm(srv.URL+"/setup/homeassistant", url.Values{
		"ha_url":   {ha.URL},
		"ha_token": {"good-token"},
	})
	if err != nil {
		t.Fatalf("post homeassistant: %v", err)
	}
	resp.Body.Close()

	select {
	case <-w.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("wizard did not signal completion")
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HAURL != ha.URL || cfg.HAToken != "good-token" {
		t.Fatalf("got %+v, want HAURL=%s HAToken=good-token", cfg, ha.URL)
	}
	if err := bcrypt.CompareHashAndPassword(cfg.AdminPasswordHash, []byte("hunter2")); err != nil {
		t.Fatalf("stored password hash does not match submitted password: %v", err)
	}
}

func TestWizardRejectsBadTokenWithoutPersisting(t *testing.T) {
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ha.Close()

	path := filepath.Join(t.TempDir(), "standalone.json")
	store := NewStore(path)
	w := NewWizard(store, noDiscovery)
	srv := httptest.NewServer(w.Handler())
	defer srv.Close()
	client := newTestClient(t)

	resp, _ := client.PostForm(srv.URL+"/setup/password", url.Values{"password": {"hunter2"}, "confirm": {"hunter2"}})
	resp.Body.Close()

	resp, err := client.PostForm(srv.URL+"/setup/homeassistant", url.Values{
		"ha_url":   {ha.URL},
		"ha_token": {"bad-token"},
	})
	if err != nil {
		t.Fatalf("post homeassistant: %v", err)
	}
	resp.Body.Close()

	select {
	case <-w.Done():
		t.Fatal("wizard signaled completion despite a rejected token")
	case <-time.After(200 * time.Millisecond):
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Complete() {
		t.Fatalf("config should not have been persisted, got %+v", cfg)
	}
}

func TestWizardRejectsMismatchedPasswordConfirmation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "standalone.json"))
	w := NewWizard(store, noDiscovery)
	srv := httptest.NewServer(w.Handler())
	defer srv.Close()
	client := newTestClient(t)

	resp, err := client.PostForm(srv.URL+"/setup/password", url.Values{
		"password": {"hunter2"},
		"confirm":  {"different"},
	})
	if err != nil {
		t.Fatalf("post password: %v", err)
	}
	defer resp.Body.Close()
	if resp.Request.URL.Path != "/setup/password" {
		t.Fatalf("mismatched confirmation should redisplay the password form, got %s", resp.Request.URL.Path)
	}
}

func TestWizardRejectsEmptyHAURLOrTokenWithoutPersisting(t *testing.T) {
	tests := []struct {
		name    string
		haURL   string
		haToken string
	}{
		{"empty url", "", "some-token"},
		{"empty token", "http://homeassistant.local:8123", ""},
		{"both empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "standalone.json")
			store := NewStore(path)
			w := NewWizard(store, noDiscovery)
			srv := httptest.NewServer(w.Handler())
			defer srv.Close()
			client := newTestClient(t)

			resp, _ := client.PostForm(srv.URL+"/setup/password", url.Values{"password": {"hunter2"}, "confirm": {"hunter2"}})
			resp.Body.Close()

			resp, err := client.PostForm(srv.URL+"/setup/homeassistant", url.Values{
				"ha_url":   {tt.haURL},
				"ha_token": {tt.haToken},
			})
			if err != nil {
				t.Fatalf("post homeassistant: %v", err)
			}
			defer resp.Body.Close()

			select {
			case <-w.Done():
				t.Fatal("wizard signaled completion despite an empty ha_url/ha_token")
			case <-time.After(200 * time.Millisecond):
			}

			cfg, err := store.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Complete() {
				t.Fatalf("config should not have been persisted, got %+v", cfg)
			}
		})
	}
}

func TestWizardHASubmitBeforePasswordRedirectsToPasswordStep(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "standalone.json"))
	w := NewWizard(store, noDiscovery)
	srv := httptest.NewServer(w.Handler())
	defer srv.Close()
	client := newTestClient(t)

	resp, err := client.PostForm(srv.URL+"/setup/homeassistant", url.Values{
		"ha_url":   {"http://homeassistant.local:8123"},
		"ha_token": {"some-token"},
	})
	if err != nil {
		t.Fatalf("post homeassistant: %v", err)
	}
	defer resp.Body.Close()
	if resp.Request.URL.Path != "/setup/password" {
		t.Fatalf("submitting the HA step before the password step should redirect to /setup/password, got %s", resp.Request.URL.Path)
	}
}
