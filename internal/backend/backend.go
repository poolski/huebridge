package backend

import "context"

// EntityState is a snapshot of one entity's state, in Hue-oriented terms.
// Brightness/ColorXY/ColorTempMirek are nil when the underlying entity
// doesn't support that feature (e.g. an on/off-only switch).
type EntityState struct {
	EntityID       string
	On             bool
	Brightness     *uint8      // 0-255, matching HA's native light.brightness attribute
	ColorXY        *[2]float64 // CIE xy, 0-1
	ColorTempMirek *uint16
	Reachable      bool
}

// DesiredState carries only the fields a caller wants to change; nil means
// "leave as-is".
type DesiredState struct {
	On             *bool
	Brightness     *uint8
	ColorXY        *[2]float64
	ColorTempMirek *uint16
}

type StateChange struct {
	State EntityState
}

// Backend abstracts entity access so the CLIP v1 layer never talks to Home
// Assistant (or any future backend) directly.
type Backend interface {
	GetState(ctx context.Context, entityID string) (EntityState, error)
	SetState(ctx context.Context, entityID string, desired DesiredState) error
	// Subscribe returns a channel of state changes for all entities this
	// backend knows about. The channel is closed if the backend's
	// underlying connection is permanently torn down (never expected in
	// normal operation — callers should treat closure as fatal).
	Subscribe(ctx context.Context) (<-chan StateChange, error)
}

// SceneMirror is implemented by backends that can mirror a Hue-app scene
// into their own native scene concept. HomeAssistantBackend implements it
// against HA's scene config storage collection; not every future Backend
// needs to.
type SceneMirror interface {
	MirrorScene(ctx context.Context, sceneID, name string, lightStates map[string]DesiredState) error
	DeleteMirroredScene(ctx context.Context, sceneID string) error
}
