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

type Group struct {
	HueID     int      `json:"hue_id"`
	Name      string   `json:"name"`
	Class     string   `json:"class"`
	EntityIDs []string `json:"entity_ids"`
}

type fileFormat struct {
	NextID      int     `json:"next_id"`
	Entries     []Entry `json:"entries"`
	NextGroupID int     `json:"next_group_id"`
	Groups      []Group `json:"groups"`
}

type Registry struct {
	mu    sync.Mutex
	file  *store.JSONFile[fileFormat]
	state fileFormat
}

func NewRegistry(path string) (*Registry, error) {
	file := store.NewJSONFile[fileFormat](path)
	state, err := file.Load(fileFormat{NextID: 1, NextGroupID: 1})
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	// Self-heal: ensure NextID is strictly greater than all existing HueIDs.
	// This handles corrupted or hand-edited registry files.
	maxHueID := 0
	for _, e := range state.Entries {
		if e.HueID > maxHueID {
			maxHueID = e.HueID
		}
	}
	if state.NextID <= maxHueID {
		state.NextID = maxHueID + 1
	}

	// Same self-heal for groups: ensure NextGroupID is strictly greater than
	// all existing group HueIDs.
	maxGroupHueID := 0
	for _, g := range state.Groups {
		if g.HueID > maxGroupHueID {
			maxGroupHueID = g.HueID
		}
	}
	if state.NextGroupID <= maxGroupHueID {
		state.NextGroupID = maxGroupHueID + 1
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
	// Early-return if nothing changed.
	if len(filtered) == len(r.state.Entries) {
		return nil
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

func (r *Registry) AddGroup(name, class string, entityIDs []string) (Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state.NextGroupID == 0 {
		r.state.NextGroupID = 1
	}

	g := Group{HueID: r.state.NextGroupID, Name: name, Class: class, EntityIDs: entityIDs}
	r.state.NextGroupID++
	r.state.Groups = append(r.state.Groups, g)

	if err := r.file.Save(r.state); err != nil {
		return Group{}, fmt.Errorf("save registry: %w", err)
	}
	return g, nil
}

func (r *Registry) AllGroups() []Group {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Group, len(r.state.Groups))
	copy(out, r.state.Groups)
	return out
}

func (r *Registry) GroupByHueID(id int) (Group, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, g := range r.state.Groups {
		if g.HueID == id {
			return g, true
		}
	}
	return Group{}, false
}
