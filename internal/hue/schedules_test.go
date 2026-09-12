package hue

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"huebridge/internal/backend"
	"huebridge/internal/backend/fake"
)

func TestScheduleStore_CreateEnforcesBodySizeLimit(t *testing.T) {
	store := NewScheduleStore(t.TempDir() + "/schedules.json")

	// A body that serializes to more than 90 characters must be rejected,
	// per the official spec's ScheduleCommand.body constraint.
	bigBody := map[string]any{
		"on": true, "bri": 200, "hue": 46920, "sat": 254, "xy": []float64{0.1234567, 0.7654321}, "transitiontime": 4, "ct": 300,
	}
	_, err := store.Create("Too big", "/groups/1/action", "PUT", bigBody, "W127/T07:00:00")
	if err == nil {
		t.Fatal("expected an error for an oversized command body, got nil")
	}
}

func TestTicker_FiresScheduleAtLocalTime(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	store := NewScheduleStore(t.TempDir() + "/schedules.json")
	sched, err := store.Create("Wake up", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T07:00:00")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	ticker := NewTicker(store, be, newStaticResolver(map[int]string{1: "light.kitchen"}))
	firedAt := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC) // Monday
	ticker.Tick(context.Background(), firedAt)

	got, _ := be.GetState(nil, "light.kitchen")
	if !got.On {
		t.Fatal("expected the schedule to turn the kitchen light on at 07:00")
	}

	reloaded, _ := store.Get(sched.ID)
	if reloaded.Status != "enabled" {
		t.Fatalf("got Status=%q after firing a recurring schedule, want it to stay enabled", reloaded.Status)
	}
}

func TestTicker_DoesNotFireBeforeScheduledTime(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	store := NewScheduleStore(t.TempDir() + "/schedules.json")
	store.Create("Wake up", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T07:00:00")

	ticker := NewTicker(store, be, newStaticResolver(map[int]string{1: "light.kitchen"}))
	tooEarly := time.Date(2026, 9, 14, 6, 59, 0, 0, time.UTC)
	ticker.Tick(context.Background(), tooEarly)

	got, _ := be.GetState(nil, "light.kitchen")
	if got.On {
		t.Fatal("schedule fired before its scheduled time")
	}
}

func TestSchedules_GetAllReturnsSeededSchedule(t *testing.T) {
	store := NewScheduleStore(t.TempDir() + "/schedules.json")
	sched, err := store.Create("Wake up", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T07:00:00")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	handler := handleGetSchedules(store)
	req := httptest.NewRequest("GET", "/api/testuser/schedules", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]Schedule
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	got, ok := body[sched.ID]
	if !ok {
		t.Fatalf("got %+v, want key %q for the seeded schedule", body, sched.ID)
	}
	if got.Name != "Wake up" || got.LocalTime != "W127/T07:00:00" || got.Status != "enabled" {
		t.Fatalf("got %+v, want Name=Wake up LocalTime=W127/T07:00:00 Status=enabled", got)
	}
	if got.Command.Address != "/lights/1/state" || got.Command.Method != "PUT" {
		t.Fatalf("got Command=%+v, want Address=/lights/1/state Method=PUT", got.Command)
	}
}

func TestSchedules_GetOneReturnsSchedule(t *testing.T) {
	store := NewScheduleStore(t.TempDir() + "/schedules.json")
	sched, err := store.Create("Wake up", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T07:00:00")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	handler := handleGetSchedule(store)
	req := httptest.NewRequest("GET", "/api/testuser/schedules/"+sched.ID, nil)
	req.SetPathValue("id", sched.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var got Schedule
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "Wake up" || got.LocalTime != "W127/T07:00:00" || got.Status != "enabled" {
		t.Fatalf("got %+v, want Name=Wake up LocalTime=W127/T07:00:00 Status=enabled", got)
	}
	if got.Command.Address != "/lights/1/state" || got.Command.Method != "PUT" {
		t.Fatalf("got Command=%+v, want Address=/lights/1/state Method=PUT", got.Command)
	}
}

func TestSchedules_GetOneUnknownIDReturnsError(t *testing.T) {
	store := NewScheduleStore(t.TempDir() + "/schedules.json")

	handler := handleGetSchedule(store)
	req := httptest.NewRequest("GET", "/api/testuser/schedules/does-not-exist", nil)
	req.SetPathValue("id", "does-not-exist")
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body []ErrorItem
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 1 || body[0].Error.Type != 3 {
		t.Fatalf("got %+v, want a single type-3 (resource not available) error", body)
	}
}

func TestScheduleStore_RejectsMalformedLocalTime(t *testing.T) {
	for _, localTime := range []string{"", "W127/T", "W127/T07", "07:00:00", "W127/T99:99:99"} {
		store := NewScheduleStore(t.TempDir() + "/schedules.json")
		if _, err := store.Create("Bad", "/lights/1/state", "PUT", map[string]any{"on": true}, localTime); err == nil {
			t.Fatalf("Create(localtime=%q) returned no error, want a rejection", localTime)
		}
	}
}

// TestTicker_SurvivesMalformedLocalTimeOnDisk covers a schedules.json that
// was hand-edited past Create's validation: the ticker must skip it, not
// panic and take the goroutine down with it.
func TestTicker_SurvivesMalformedLocalTimeOnDisk(t *testing.T) {
	be := fake.New()
	be.Seed(backend.EntityState{EntityID: "light.kitchen", On: false, Reachable: true})

	store := NewScheduleStore(t.TempDir() + "/schedules.json")
	store.state.Schedules["1"] = StoredSchedule{ID: "1", Name: "Broken", Address: "/lights/1/state", Method: "PUT", LocalTime: "W127/T", Status: "enabled"}

	ticker := NewTicker(store, be, newStaticResolver(map[int]string{1: "light.kitchen"}))
	ticker.Tick(context.Background(), time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC))
}

// TestScheduleStore_IDsAreNotReusedAfterDelete guards the ID-collision bug:
// ids came from len(schedules)+1, so deleting one handed its id straight to
// the next schedule created.
func TestScheduleStore_IDsAreNotReusedAfterDelete(t *testing.T) {
	path := t.TempDir() + "/schedules.json"
	store := NewScheduleStore(path)

	first, _ := store.Create("One", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T07:00:00")
	second, _ := store.Create("Two", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T08:00:00")
	if err := store.Delete(second.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	third, _ := store.Create("Three", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T09:00:00")
	if third.ID == first.ID || third.ID == second.ID {
		t.Fatalf("new schedule reused id %q (existing: %q, deleted: %q)", third.ID, first.ID, second.ID)
	}

	// The same must hold after a reload, via the persisted/self-healed NextID.
	reloaded := NewScheduleStore(path)
	fourth, _ := reloaded.Create("Four", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T10:00:00")
	if fourth.ID == first.ID || fourth.ID == third.ID {
		t.Fatalf("schedule created after reload reused id %q", fourth.ID)
	}
}

// TestScheduleStore_SelfHealsNextIDFromExistingSchedules mirrors the
// registry's self-heal: a file with schedules but a stale next_id must not
// hand out a colliding id.
func TestScheduleStore_SelfHealsNextIDFromExistingSchedules(t *testing.T) {
	path := t.TempDir() + "/schedules.json"
	if err := os.WriteFile(path, []byte(`{"next_id":1,"schedules":{"7":{"id":"7","name":"Existing","localtime":"W127/T07:00:00","status":"enabled"}}}`), 0o644); err != nil {
		t.Fatalf("write schedules file: %v", err)
	}

	store := NewScheduleStore(path)
	created, err := store.Create("New", "/lights/1/state", "PUT", map[string]any{"on": true}, "W127/T08:00:00")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "7" {
		t.Fatal("Create reused the existing schedule id 7 despite a stale next_id")
	}
}
