// Package setup (continued) implements the standalone first-run setup
// wizard: an admin password step, then a Home Assistant URL/token step
// that's verified against HA before anything is persisted.
package setup

import (
	"context"
	"net/http"
	"sync"
	"time"

	"huebridge/internal/backend/homeassistant"
)

// Wizard serves the setup HTML flow and signals completion via Done()
// once a working config has been persisted to its Store.
type Wizard struct {
	store    *Store
	discover func(time.Duration) ([]string, error)
	verifyHA func(ctx context.Context, url, token string) error

	mu            sync.Mutex
	passwordHash  []byte
	passwordIsSet bool

	doneOnce sync.Once
	done     chan struct{}
}

// NewWizard builds a Wizard persisting to s, using discover to prefill the
// Home Assistant URL field (see discovery.DiscoverHomeAssistant — passed
// as a parameter so this package doesn't need to import internal/discovery).
func NewWizard(s *Store, discover func(time.Duration) ([]string, error)) *Wizard {
	return &Wizard{
		store:    s,
		discover: discover,
		verifyHA: verifyHomeAssistant,
		done:     make(chan struct{}),
	}
}

// verifyHomeAssistant checks that token actually authenticates against the
// Home Assistant instance at url, using the same client the running bridge
// will use, so a bad token is caught during setup rather than at first use.
func verifyHomeAssistant(ctx context.Context, url, token string) error {
	_, err := homeassistant.New(url, token).ListEntities(ctx)
	return err
}

// Done reports when the wizard has collected and persisted a complete
// config. The caller (main) blocks on this to know when to stop serving
// the wizard and start the bridge proper.
func (w *Wizard) Done() <-chan struct{} { return w.done }

// Handler returns the wizard's HTTP handler.
func (w *Wizard) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, r *http.Request) {
		http.Redirect(rw, r, "/setup/password", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /setup/password", w.handlePasswordForm)
	mux.HandleFunc("POST /setup/password", w.handlePasswordSubmit)
	mux.HandleFunc("GET /setup/homeassistant", w.handleHAForm)
	mux.HandleFunc("POST /setup/homeassistant", w.handleHASubmit)
	return mux
}

func (w *Wizard) handlePasswordForm(rw http.ResponseWriter, r *http.Request) {
	passwordTemplate.Execute(rw, struct{ Error string }{})
}

func (w *Wizard) handlePasswordSubmit(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(rw, "invalid form", http.StatusBadRequest)
		return
	}
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")
	if password == "" || password != confirm {
		passwordTemplate.Execute(rw, struct{ Error string }{Error: "Passwords must match and not be empty."})
		return
	}
	hash, err := HashPassword(password)
	if err != nil {
		http.Error(rw, "failed to set password", http.StatusInternalServerError)
		return
	}

	w.mu.Lock()
	w.passwordHash = hash
	w.passwordIsSet = true
	w.mu.Unlock()

	http.Redirect(rw, r, "/setup/homeassistant", http.StatusSeeOther)
}

func (w *Wizard) handleHAForm(rw http.ResponseWriter, r *http.Request) {
	var discovered string
	if urls, err := w.discover(2 * time.Second); err == nil && len(urls) > 0 {
		discovered = urls[0]
	}
	haTemplate.Execute(rw, haFormData{DiscoveredURL: discovered})
}

func (w *Wizard) handleHASubmit(rw http.ResponseWriter, r *http.Request) {
	w.mu.Lock()
	passwordHash := w.passwordHash
	passwordSet := w.passwordIsSet
	w.mu.Unlock()
	if !passwordSet {
		http.Redirect(rw, r, "/setup/password", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(rw, "invalid form", http.StatusBadRequest)
		return
	}
	haURL := r.FormValue("ha_url")
	haToken := r.FormValue("ha_token")
	if haURL == "" || haToken == "" {
		haTemplate.Execute(rw, haFormData{
			Error:         "Home Assistant URL and access token are both required.",
			DiscoveredURL: haURL,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := w.verifyHA(ctx, haURL, haToken); err != nil {
		haTemplate.Execute(rw, haFormData{
			Error:         "Could not connect: " + err.Error(),
			DiscoveredURL: haURL,
		})
		return
	}

	cfg := Config{HAURL: haURL, HAToken: haToken, AdminPasswordHash: passwordHash}
	if err := w.store.Save(cfg); err != nil {
		http.Error(rw, "failed to save configuration", http.StatusInternalServerError)
		return
	}

	completeTemplate.Execute(rw, nil)
	w.doneOnce.Do(func() { close(w.done) })
}
