package hue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"huebridge/internal/backend"
	"huebridge/internal/store"
)

type StoredSchedule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Method    string `json:"method"`
	Body      any    `json:"body"`
	LocalTime string `json:"localtime"`
	Status    string `json:"status"`
	// lastFiredDay avoids re-firing a recurring schedule more than once
	// within the same minute across repeated Tick calls.
	lastFiredDay string
}

type scheduleFile struct {
	Schedules map[string]StoredSchedule `json:"schedules"`
}

type ScheduleStore struct {
	mu    sync.Mutex
	file  *store.JSONFile[scheduleFile]
	state scheduleFile
}

func NewScheduleStore(path string) *ScheduleStore {
	file := store.NewJSONFile[scheduleFile](path)
	state, _ := file.Load(scheduleFile{Schedules: map[string]StoredSchedule{}})
	if state.Schedules == nil {
		state.Schedules = map[string]StoredSchedule{}
	}
	return &ScheduleStore{file: file, state: state}
}

// maxCommandBodyBytes matches the official spec's ScheduleCommand.body
// limit: 90 characters serialized. See
// docs/superpowers/specs/hue-api-v1-openapi.json, schema ScheduleCommand.
const maxCommandBodyBytes = 90

func (s *ScheduleStore) Create(name, address, method string, body any, localTime string) (StoredSchedule, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return StoredSchedule{}, fmt.Errorf("encode command body: %w", err)
	}
	if len(encoded) > maxCommandBodyBytes {
		return StoredSchedule{}, fmt.Errorf("command body is %d bytes, exceeds the %d-byte limit", len(encoded), maxCommandBodyBytes)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := strconv.Itoa(len(s.state.Schedules) + 1)
	sched := StoredSchedule{ID: id, Name: name, Address: address, Method: method, Body: body, LocalTime: localTime, Status: "enabled"}
	s.state.Schedules[id] = sched
	return sched, s.file.Save(s.state)
}

func (s *ScheduleStore) Get(id string) (StoredSchedule, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sched, ok := s.state.Schedules[id]
	return sched, ok
}

func (s *ScheduleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Schedules, id)
	return s.file.Save(s.state)
}

func (s *ScheduleStore) All() []StoredSchedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StoredSchedule, 0, len(s.state.Schedules))
	for _, sched := range s.state.Schedules {
		out = append(out, sched)
	}
	return out
}

// resolveEntityID adapts a numeric-light-id -> HA-entity-id lookup for the
// ticker, matching how a schedule's recorded /lights/<id>/state address
// resolves to a real entity.
type resolveEntityID func(hueLightID int) (string, bool)

func newStaticResolver(m map[int]string) resolveEntityID {
	return func(id int) (string, bool) {
		v, ok := m[id]
		return v, ok
	}
}

// Ticker fires due schedules by translating their recorded command into a
// backend.SetState call. Tick is exposed directly (rather than an internal
// sleep loop) so tests can drive it deterministically; production code
// calls it once a minute from a time.Ticker in cmd/huebridge/main.go.
type Ticker struct {
	store   *ScheduleStore
	be      backend.Backend
	resolve resolveEntityID
}

func NewTicker(store *ScheduleStore, be backend.Backend, resolve resolveEntityID) *Ticker {
	return &Ticker{store: store, be: be, resolve: resolve}
}

// matchesWeeklyTime checks a "W<bitmask>/T<HH:MM:SS>" localtime pattern
// against now, at minute resolution. Only the weekly-recurring form is
// supported in v1 — absolute and randomized patterns are out of scope
// until a real need for them shows up.
func matchesWeeklyTime(pattern string, now time.Time) bool {
	if !strings.HasPrefix(pattern, "W127/T") {
		return false // v1 only supports "every day" (bitmask 127); see design doc non-goals for narrower recurrence
	}
	wantTime := strings.TrimPrefix(pattern, "W127/T")
	gotTime := now.Format("15:04:05")
	return strings.HasPrefix(gotTime, wantTime[:5]) // compare to the minute
}

func (t *Ticker) Tick(now time.Time) {
	today := now.Format("2006-01-02 15:04")
	for _, sched := range t.store.All() {
		if sched.Status != "enabled" {
			continue
		}
		if sched.lastFiredDay == today {
			continue
		}
		if !matchesWeeklyTime(sched.LocalTime, now) {
			continue
		}

		var body map[string]any
		if b, ok := sched.Body.(map[string]any); ok {
			body = b
		}

		desired := backend.DesiredState{}
		if v, ok := body["on"].(bool); ok {
			desired.On = &v
		}
		if v, ok := body["bri"].(float64); ok {
			b := uint8(v)
			desired.Brightness = &b
		}

		if entityID, ok := t.resolveTargetEntity(sched.Address); ok {
			t.be.SetState(nil, entityID, desired)
		}

		sched.lastFiredDay = today
		t.store.mu.Lock()
		t.store.state.Schedules[sched.ID] = sched
		t.store.mu.Unlock()
	}
}

func (t *Ticker) resolveTargetEntity(address string) (string, bool) {
	parts := strings.Split(strings.Trim(address, "/"), "/")
	for i, p := range parts {
		if p == "lights" && i+1 < len(parts) {
			id, err := strconv.Atoi(parts[i+1])
			if err != nil {
				return "", false
			}
			return t.resolve(id)
		}
	}
	return "", false
}

func toSchedule(s StoredSchedule) Schedule {
	return Schedule{
		Name:      s.Name,
		Command:   ScheduleCommand{Address: s.Address, Method: s.Method, Body: s.Body},
		LocalTime: s.LocalTime,
		Status:    s.Status,
	}
}

func handleGetSchedules(schedules *ScheduleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]Schedule{}
		for _, s := range schedules.All() {
			out[s.ID] = toSchedule(s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func handleGetSchedule(schedules *ScheduleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, ok := schedules.Get(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toSchedule(s))
	}
}

func handlePostSchedule(schedules *ScheduleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name      string          `json:"name"`
			LocalTime string          `json:"localtime"`
			Command   ScheduleCommand `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusOK, 2, r.URL.Path, "body contains invalid JSON")
			return
		}

		sched, err := schedules.Create(req.Name, req.Command.Address, req.Command.Method, req.Command.Body, req.LocalTime)
		if err != nil {
			WriteError(w, http.StatusOK, 7, r.URL.Path, err.Error())
			return
		}

		WriteSuccess(w, map[string]any{"id": sched.ID})
	}
}

func handleDeleteSchedule(schedules *ScheduleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := schedules.Get(id); !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		schedules.Delete(id)
		WriteSuccess(w, map[string]any{"id": id})
	}
}
