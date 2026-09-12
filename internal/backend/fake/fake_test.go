package fake

import (
	"context"
	"testing"

	"huebridge/internal/backend"
)

func TestFakeBackend_SeedAndGetState(t *testing.T) {
	b := New()
	b.Seed(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	got, err := b.GetState(context.Background(), "light.kitchen")
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if !got.On || !got.Reachable {
		t.Fatalf("got %+v, want On=true Reachable=true", got)
	}
}

func TestFakeBackend_SetStateUpdatesState(t *testing.T) {
	b := New()
	b.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	on := true
	if err := b.SetState(context.Background(), "light.kitchen", backend.DesiredState{On: &on}); err != nil {
		t.Fatalf("SetState returned error: %v", err)
	}

	got, _ := b.GetState(context.Background(), "light.kitchen")
	if !got.On {
		t.Fatalf("got On=%v, want true", got.On)
	}
}

func TestFakeBackend_SubscribeReceivesPushedChanges(t *testing.T) {
	b := New()
	b.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	ch, err := b.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	b.Push(backend.EntityState{EntityID: "light.kitchen", On: true, Reachable: true})

	select {
	case change := <-ch:
		if !change.State.On {
			t.Fatalf("got On=%v, want true", change.State.On)
		}
	default:
		t.Fatal("expected a state change on the subscription channel")
	}
}

func TestFakeBackend_GetStateUnknownEntity(t *testing.T) {
	b := New()
	if _, err := b.GetState(context.Background(), "light.missing"); err == nil {
		t.Fatal("expected an error for an unknown entity, got nil")
	}
}
