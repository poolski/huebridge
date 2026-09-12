// Package fake provides an in-memory backend.Backend test double.
package fake

import (
	"context"
	"fmt"
	"sync"

	"huebridge/internal/backend"
)

type Backend struct {
	mu             sync.Mutex
	states         map[string]backend.EntityState
	subs           []chan backend.StateChange
	mirroredScenes map[string]bool
}

func New() *Backend {
	return &Backend{states: make(map[string]backend.EntityState)}
}

// Seed sets an entity's initial state, for test setup.
func (b *Backend) Seed(state backend.EntityState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.states[state.EntityID] = state
}

// Push updates an entity's state and notifies subscribers, simulating an
// HA-originated change.
func (b *Backend) Push(state backend.EntityState) {
	b.mu.Lock()
	b.states[state.EntityID] = state
	subs := append([]chan backend.StateChange{}, b.subs...)
	b.mu.Unlock()

	for _, ch := range subs {
		ch <- backend.StateChange{State: state}
	}
}

func (b *Backend) GetState(_ context.Context, entityID string) (backend.EntityState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.states[entityID]
	if !ok {
		return backend.EntityState{}, fmt.Errorf("fake backend: unknown entity %q", entityID)
	}
	return s, nil
}

func (b *Backend) SetState(_ context.Context, entityID string, desired backend.DesiredState) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.states[entityID]
	if !ok {
		return fmt.Errorf("fake backend: unknown entity %q", entityID)
	}
	if desired.On != nil {
		s.On = *desired.On
	}
	if desired.Brightness != nil {
		s.Brightness = desired.Brightness
	}
	if desired.ColorXY != nil {
		s.ColorXY = desired.ColorXY
	}
	if desired.ColorTempMirek != nil {
		s.ColorTempMirek = desired.ColorTempMirek
	}
	b.states[entityID] = s
	return nil
}

func (b *Backend) Subscribe(_ context.Context) (<-chan backend.StateChange, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan backend.StateChange, 16)
	b.subs = append(b.subs, ch)
	return ch, nil
}

func (b *Backend) MirrorScene(_ context.Context, sceneID, name string, lightStates map[string]backend.DesiredState) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.mirroredScenes == nil {
		b.mirroredScenes = map[string]bool{}
	}
	b.mirroredScenes[sceneID] = true
	return nil
}

func (b *Backend) DeleteMirroredScene(_ context.Context, sceneID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.mirroredScenes, sceneID)
	return nil
}

func (b *Backend) MirroredSceneCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.mirroredScenes)
}
