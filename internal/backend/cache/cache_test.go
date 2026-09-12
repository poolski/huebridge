package cache

import (
	"context"
	"testing"
	"time"

	"huebridge/internal/backend"
	"huebridge/internal/backend/fake"
)

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}

func TestCache_ServesSubscribedState(t *testing.T) {
	inner := fake.New()
	inner.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	c := New(inner)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	inner.Push(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	waitFor(t, func() bool {
		got, err := c.GetState(ctx, "light.kitchen")
		return err == nil && got.On
	})
}

func TestCache_ServesLastKnownStateWhenBackendFails(t *testing.T) {
	inner := fake.New()
	c := New(inner)

	// The entity is known to the cache (pushed over the subscription) but
	// not to the backend, so a fallback GetState would fail.
	c.put(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	got, err := c.GetState(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("GetState() error: %v, want the cached state", err)
	}
	if !got.On {
		t.Fatalf("got %+v, want the cached On=true state", got)
	}
}

func TestCache_FallsBackToBackendOnMiss(t *testing.T) {
	inner := fake.New()
	inner.Seed(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	c := New(inner)
	got, err := c.GetState(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("GetState() error: %v", err)
	}
	if !got.On {
		t.Fatalf("got %+v, want On=true from the wrapped backend", got)
	}
	if _, ok := c.lookup("light.kitchen"); !ok {
		t.Fatal("expected the fallback read to populate the cache")
	}
}

func TestCache_SetStateUpdatesSnapshot(t *testing.T) {
	inner := fake.New()
	inner.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	c := New(inner)
	if _, err := c.GetState(context.Background(), "light.kitchen"); err != nil {
		t.Fatalf("GetState() error: %v", err)
	}

	on := true
	if err := c.SetState(context.Background(), "light.kitchen", backend.DesiredState{On: &on}); err != nil {
		t.Fatalf("SetState() error: %v", err)
	}

	got, _ := c.lookup("light.kitchen")
	if !got.On {
		t.Fatalf("got %+v, want the snapshot updated to On=true", got)
	}
}
