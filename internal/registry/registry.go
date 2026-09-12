// Package registry maps Home Assistant entity ids to stable numeric Hue
// light ids. Ids are assigned once and never reused, since the Hue app and
// any scene/schedule referencing them expects them to stay stable across
// restarts.
package registry

import (
	"fmt"
	"sync"

	"huebridge/internal/store"
)

type Entry struct {
	HueID    int    `json:"hue_id"`
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
}

type fileFormat struct {
	NextID  int     `json:"next_id"`
	Entries []Entry `json:"entries"`
}

type Registry struct {
	mu    sync.Mutex
	file  *store.JSONFile[fileFormat]
	state fileFormat
}

func NewRegistry(path string) (*Registry, error) {
	file := store.NewJSONFile[fileFormat](path)
	state, err := file.Load(fileFormat{NextID: 1})
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}
	return &Registry{file: file, state: state}, nil
}

func (r *Registry) Add(entityID, name string) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.state.Entries {
		if e.EntityID == entityID {
			return e, nil
		}
	}

	entry := Entry{HueID: r.state.NextID, EntityID: entityID, Name: name}
	r.state.NextID++
	r.state.Entries = append(r.state.Entries, entry)

	if err := r.file.Save(r.state); err != nil {
		return Entry{}, fmt.Errorf("save registry: %w", err)
	}
	return entry, nil
}

func (r *Registry) Remove(entityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := r.state.Entries[:0]
	for _, e := range r.state.Entries {
		if e.EntityID != entityID {
			filtered = append(filtered, e)
		}
	}
	r.state.Entries = filtered

	return r.file.Save(r.state)
}

func (r *Registry) ByHueID(id int) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.state.Entries {
		if e.HueID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func (r *Registry) ByEntityID(entityID string) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.state.Entries {
		if e.EntityID == entityID {
			return e, true
		}
	}
	return Entry{}, false
}

func (r *Registry) All() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, len(r.state.Entries))
	copy(out, r.state.Entries)
	return out
}
