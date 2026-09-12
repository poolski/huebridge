// Package cache wraps a backend.Backend with an in-memory snapshot of
// last-known entity state, kept current from the backend's Subscribe
// stream.
//
// The Hue app polls /lights roughly once a second. Serving each poll with
// one REST round-trip per exposed entity would hammer Home Assistant for
// data the WebSocket already pushes us, so reads come from the snapshot and
// only fall back to the wrapped backend for an entity we have never seen.
// A consequence worth having: when HA is briefly unreachable we serve the
// last known state instead of failing the poll.
package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"huebridge/internal/backend"
)

const (
	initialRetryDelay = time.Second
	maxRetryDelay     = 30 * time.Second
)

type Backend struct {
	inner backend.Backend

	mu     sync.RWMutex
	states map[string]backend.EntityState
}

func New(inner backend.Backend) *Backend {
	return &Backend{inner: inner, states: make(map[string]backend.EntityState)}
}

func (c *Backend) put(state backend.EntityState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[state.EntityID] = state
}

func (c *Backend) lookup(entityID string) (backend.EntityState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.states[entityID]
	return s, ok
}

// Run keeps the snapshot current until ctx is cancelled, resubscribing with
// a capped exponential backoff whenever the subscription fails or its
// channel closes. Callers run it in its own goroutine.
func (c *Backend) Run(ctx context.Context) {
	delay := initialRetryDelay
	for {
		ch, err := c.inner.Subscribe(ctx)
		if err != nil {
			log.Printf("state cache: subscribe failed, retrying in %s: %v", delay, err)
		} else {
			delay = initialRetryDelay
			for change := range ch {
				c.put(change.State)
			}
			if ctx.Err() != nil {
				return
			}
			log.Printf("state cache: subscription closed, resubscribing in %s", delay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < maxRetryDelay {
			delay *= 2
		}
	}
}

// GetState serves the cached snapshot, falling back to the wrapped backend
// only for an entity the cache has never seen.
func (c *Backend) GetState(ctx context.Context, entityID string) (backend.EntityState, error) {
	if s, ok := c.lookup(entityID); ok {
		return s, nil
	}

	s, err := c.inner.GetState(ctx, entityID)
	if err != nil {
		return backend.EntityState{}, err
	}
	c.put(s)
	return s, nil
}

// SetState writes through to the backend and optimistically applies the
// change to the snapshot, so the app sees its own command reflected without
// waiting for HA's state_changed event to come back round.
func (c *Backend) SetState(ctx context.Context, entityID string, desired backend.DesiredState) error {
	if err := c.inner.SetState(ctx, entityID, desired); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.states[entityID]
	if !ok {
		return nil
	}
	if desired.On != nil {
		s.On = *desired.On
	}
	if desired.Brightness != nil {
		s.Brightness = desired.Brightness
	}
	if desired.ColorXY != nil {
		s.ColorXY = desired.ColorXY
		s.ColorTempMirek = nil
	}
	if desired.ColorTempMirek != nil {
		s.ColorTempMirek = desired.ColorTempMirek
		s.ColorXY = nil
	}
	c.states[entityID] = s
	return nil
}

func (c *Backend) Subscribe(ctx context.Context) (<-chan backend.StateChange, error) {
	return c.inner.Subscribe(ctx)
}

func (c *Backend) ListEntities(ctx context.Context) ([]string, error) {
	return c.inner.ListEntities(ctx)
}

// MirrorScene and DeleteMirroredScene keep the wrapper transparent to the
// scene handlers' backend.SceneMirror type assertion.
func (c *Backend) MirrorScene(ctx context.Context, sceneID, name string, lightStates map[string]backend.DesiredState) error {
	m, ok := c.inner.(backend.SceneMirror)
	if !ok {
		return fmt.Errorf("backend does not support scene mirroring")
	}
	return m.MirrorScene(ctx, sceneID, name, lightStates)
}

func (c *Backend) DeleteMirroredScene(ctx context.Context, sceneID string) error {
	m, ok := c.inner.(backend.SceneMirror)
	if !ok {
		return fmt.Errorf("backend does not support scene mirroring")
	}
	return m.DeleteMirroredScene(ctx, sceneID)
}
