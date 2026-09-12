package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"

	"huebridge/internal/backend"
)

type wsMessage struct {
	Type  string          `json:"type"`
	ID    int             `json:"id,omitempty"`
	Event json.RawMessage `json:"event,omitempty"`
}

type stateChangedEvent struct {
	EventType string `json:"event_type"`
	Data      struct {
		EntityID string  `json:"entity_id"`
		NewState haState `json:"new_state"`
	} `json:"data"`
}

func (b *Backend) Subscribe(ctx context.Context) (<-chan backend.StateChange, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, b.wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}

	var authMsg wsMessage
	if err := conn.ReadJSON(&authMsg); err != nil {
		return nil, fmt.Errorf("read auth_required: %w", err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "auth", "access_token": b.token}); err != nil {
		return nil, fmt.Errorf("send auth: %w", err)
	}
	var authResult wsMessage
	if err := conn.ReadJSON(&authResult); err != nil {
		return nil, fmt.Errorf("read auth result: %w", err)
	}
	if authResult.Type != "auth_ok" {
		return nil, fmt.Errorf("authentication failed: %s", authResult.Type)
	}

	if err := conn.WriteJSON(map[string]any{
		"id":         1,
		"type":       "subscribe_events",
		"event_type": "state_changed",
	}); err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	ch := make(chan backend.StateChange, 64)

	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()

	go func() {
		defer conn.Close()
		defer close(ch)
		for {
			var msg wsMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Type != "event" {
				continue
			}
			var event stateChangedEvent
			if err := json.Unmarshal(msg.Event, &event); err != nil {
				continue
			}
			if event.EventType != "state_changed" {
				continue
			}
			ch <- backend.StateChange{
				State: toEntityState(event.Data.EntityID, event.Data.NewState),
			}
		}
	}()

	return ch, nil
}
