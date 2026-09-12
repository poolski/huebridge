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
