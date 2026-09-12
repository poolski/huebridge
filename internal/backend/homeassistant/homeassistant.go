// Package homeassistant implements backend.Backend against a real Home
// Assistant instance: REST for reads/writes, WebSocket for live state
// change events.
package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"huebridge/internal/backend"
)

type Backend struct {
	baseURL string
	token   string
	wsURL   string // overridable in tests; defaults to baseURL's ws equivalent
	client  *http.Client

	mu   sync.Mutex
	subs []chan backend.StateChange
}

func New(baseURL, token string) *Backend {
	return &Backend{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		wsURL:   "ws" + strings.TrimPrefix(strings.TrimSuffix(baseURL, "/"), "http") + "/api/websocket",
		client:  &http.Client{},
	}
}

type haState struct {
	EntityID   string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

func (b *Backend) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("Content-Type", "application/json")

	return b.client.Do(req)
}

func toEntityState(entityID string, s haState) backend.EntityState {
	state := backend.EntityState{
		EntityID:  entityID,
		On:        s.State == "on",
		Reachable: s.State != "unavailable",
	}
	if bri, ok := s.Attributes["brightness"]; ok {
		if f, ok := bri.(float64); ok {
			v := uint8(f)
			state.Brightness = &v
		}
	}
	return state
}

func (b *Backend) GetState(ctx context.Context, entityID string) (backend.EntityState, error) {
	resp, err := b.doJSON(ctx, http.MethodGet, "/api/states/"+entityID, nil)
	if err != nil {
		return backend.EntityState{}, fmt.Errorf("get state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return backend.EntityState{}, fmt.Errorf("get state: unexpected status %d", resp.StatusCode)
	}

	var s haState
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return backend.EntityState{}, fmt.Errorf("decode state: %w", err)
	}
	return toEntityState(entityID, s), nil
}

func (b *Backend) SetState(ctx context.Context, entityID string, desired backend.DesiredState) error {
	domain := strings.SplitN(entityID, ".", 2)[0]
	payload := map[string]any{"entity_id": entityID}

	var service string
	switch {
	case desired.On != nil && !*desired.On:
		// Explicit off.
		service = "turn_off"
	case desired.On != nil && *desired.On:
		// Explicit on, plus any attribute changes.
		service = "turn_on"
		addAttributes(payload, desired)
	default:
		// desired.On == nil: leave on/off state as-is (Task 1 contract).
		// HA has no way to change a light's attributes without implicitly
		// turning it on, so we only issue turn_on (to apply attribute
		// changes) when the entity is already on; otherwise this is a
		// no-op.
		hasAttrChanges := desired.Brightness != nil || desired.ColorXY != nil || desired.ColorTempMirek != nil
		if !hasAttrChanges {
			return nil
		}

		current, err := b.GetState(ctx, entityID)
		if err != nil {
			return fmt.Errorf("get current state: %w", err)
		}
		if !current.On {
			return nil
		}
		service = "turn_on"
		addAttributes(payload, desired)
	}

	resp, err := b.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/services/%s/%s", domain, service), payload)
	if err != nil {
		return fmt.Errorf("call service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("call service: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func addAttributes(payload map[string]any, desired backend.DesiredState) {
	if desired.Brightness != nil {
		payload["brightness"] = *desired.Brightness
	}
	if desired.ColorXY != nil {
		payload["xy_color"] = []float64{desired.ColorXY[0], desired.ColorXY[1]}
	}
	if desired.ColorTempMirek != nil {
		payload["color_temp"] = *desired.ColorTempMirek
	}
}
