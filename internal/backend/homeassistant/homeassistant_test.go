package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"huebridge/internal/backend"
)

func newTestServer(t *testing.T, wsHandler http.HandlerFunc) (*httptest.Server, *Backend) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/websocket", wsHandler)
	mux.HandleFunc("/api/states/light.kitchen", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"entity_id": "light.kitchen",
			"state":     "on",
			"attributes": map[string]any{
				"brightness": 200,
			},
		})
	})
	mux.HandleFunc("/api/services/light/turn_on", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/websocket"
	b := New(srv.URL, "test-token")
	b.wsURL = wsURL
	return srv, b
}

func upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
}

func TestHomeAssistantBackend_GetState(t *testing.T) {
	_, b := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader().Upgrade(w, r, nil)
		defer conn.Close()
		conn.WriteJSON(map[string]any{"type": "auth_required"})
		conn.ReadMessage()
		conn.WriteJSON(map[string]any{"type": "auth_ok"})
		conn.ReadMessage()
	})

	got, err := b.GetState(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if !got.On || got.Brightness == nil || *got.Brightness != 200 {
		t.Fatalf("got %+v, want On=true Brightness=200", got)
	}
}

func TestHomeAssistantBackend_SubscribeReceivesStateChangedEvent(t *testing.T) {
	_, b := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader().Upgrade(w, r, nil)
		defer conn.Close()
		conn.WriteJSON(map[string]any{"type": "auth_required"})
		conn.ReadMessage()
		conn.WriteJSON(map[string]any{"type": "auth_ok"})
		conn.ReadMessage() // subscribe_events call

		conn.WriteJSON(map[string]any{
			"type": "event",
			"event": map[string]any{
				"event_type": "state_changed",
				"data": map[string]any{
					"entity_id": "light.kitchen",
					"new_state": map[string]any{
						"state":      "off",
						"attributes": map[string]any{},
					},
				},
			},
		})
		time.Sleep(100 * time.Millisecond)
	})

	ch, err := b.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case change := <-ch:
		if change.State.EntityID != "light.kitchen" || change.State.On {
			t.Fatalf("got %+v, want light.kitchen On=false", change.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for state change")
	}
}

// newSetStateTestServer builds a REST-only test server whose
// /api/states/light.kitchen response reports the given on/off state, and
// which records whether /api/services/light/turn_on was called.
func newSetStateTestServer(t *testing.T, entityOn bool) (b *Backend, turnOnCalled *bool) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/states/light.kitchen", func(w http.ResponseWriter, r *http.Request) {
		state := "off"
		if entityOn {
			state = "on"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"entity_id":  "light.kitchen",
			"state":      state,
			"attributes": map[string]any{},
		})
	})
	mux.HandleFunc("/api/services/light/turn_on", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/services/light/turn_off", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return New(srv.URL, "test-token"), &called
}

func TestHomeAssistantBackend_SetState_NilOnAttributeOnlyWhileOff_NoServiceCall(t *testing.T) {
	b, turnOnCalled := newSetStateTestServer(t, false)

	brightness := uint8(150)
	err := b.SetState(context.Background(), "light.kitchen", backend.DesiredState{
		Brightness: &brightness,
	})
	if err != nil {
		t.Fatalf("SetState() error: %v", err)
	}
	if *turnOnCalled {
		t.Fatal("turn_on service was called, but entity was off and On was nil (leave as-is)")
	}
}

func TestHomeAssistantBackend_SetState_NilOnAttributeOnlyWhileOn_CallsTurnOn(t *testing.T) {
	b, turnOnCalled := newSetStateTestServer(t, true)

	brightness := uint8(150)
	err := b.SetState(context.Background(), "light.kitchen", backend.DesiredState{
		Brightness: &brightness,
	})
	if err != nil {
		t.Fatalf("SetState() error: %v", err)
	}
	if !*turnOnCalled {
		t.Fatal("turn_on service was not called, but entity was already on and had attribute changes to apply")
	}
}

func TestToEntityState_ParsesColor(t *testing.T) {
	got := toEntityState("light.kitchen", haState{
		EntityID: "light.kitchen",
		State:    "on",
		Attributes: map[string]any{
			"brightness": float64(200),
			"xy_color":   []any{0.4576, 0.4099},
			"color_temp": float64(366),
		},
	})

	if got.ColorXY == nil || got.ColorXY[0] != 0.4576 || got.ColorXY[1] != 0.4099 {
		t.Fatalf("got ColorXY=%v, want [0.4576 0.4099]", got.ColorXY)
	}
	if got.ColorTempMirek == nil || *got.ColorTempMirek != 366 {
		t.Fatalf("got ColorTempMirek=%v, want 366", got.ColorTempMirek)
	}
}

func TestToEntityState_IgnoresMalformedColor(t *testing.T) {
	got := toEntityState("light.kitchen", haState{
		State: "on",
		Attributes: map[string]any{
			"xy_color":   []any{0.4576},
			"color_temp": "not a number",
		},
	})

	if got.ColorXY != nil {
		t.Fatalf("got ColorXY=%v, want nil for a one-element xy_color", got.ColorXY)
	}
	if got.ColorTempMirek != nil {
		t.Fatalf("got ColorTempMirek=%v, want nil for a non-numeric color_temp", got.ColorTempMirek)
	}
}

func TestHomeAssistantBackend_ListEntities(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"entity_id": "light.kitchen", "state": "on"},
			{"entity_id": "light.hall", "state": "off"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	b := New(srv.URL, "test-token")
	got, err := b.ListEntities(context.Background())
	if err != nil {
		t.Fatalf("ListEntities() error: %v", err)
	}
	if len(got) != 2 || got[0] != "light.hall" || got[1] != "light.kitchen" {
		t.Fatalf("got %v, want [light.hall light.kitchen]", got)
	}
}
