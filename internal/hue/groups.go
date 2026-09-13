package hue

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"huebridge/internal/backend"
	"huebridge/internal/registry"
)

func toGroup(ctx context.Context, reg *registry.Registry, be backend.Backend, g registry.Group) Group {
	lights := make([]string, 0, len(g.EntityIDs))
	allOn, anyOn := true, false
	// The Hue app expects a group to carry an "action" describing the last
	// state applied to it. We have no such record, so we report the first
	// member we can read as representative — good enough for the app to
	// render a brightness slider at a plausible position.
	action := GroupAction{}
	haveAction := false
	for _, entityID := range g.EntityIDs {
		if entry, ok := reg.ByEntityID(entityID); ok {
			lights = append(lights, strconv.Itoa(entry.HueID))
		}
		state, err := be.GetState(ctx, entityID)
		if err == nil && state.On {
			anyOn = true
		} else if err != nil || !state.On {
			allOn = false
		}
		if err == nil && !haveAction {
			action = GroupAction{On: state.On, Bri: state.Brightness}
			haveAction = true
		}
	}
	// anyOn is the group's on/off state as far as the app is concerned;
	// keep the action consistent with it.
	action.On = anyOn
	return Group{
		Name:    g.Name,
		Lights:  lights,
		Sensors: []string{},
		Type:    "Room",
		Class:   g.Class,
		GroupState: GroupState{
			AllOn: allOn && len(g.EntityIDs) > 0,
			AnyOn: anyOn,
		},
		Recycle: false,
		Action:  action,
	}
}

func handleGetGroup(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		g, ok := reg.GroupByHueID(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toGroup(r.Context(), reg, be, g))
	}
}

func handleGetGroups(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]Group{}
		for _, g := range reg.AllGroups() {
			out[strconv.Itoa(g.HueID)] = toGroup(r.Context(), reg, be, g)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func handlePutGroupAction(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		g, ok := reg.GroupByHueID(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}

		var raw map[string]any
		json.NewDecoder(r.Body).Decode(&raw)

		desired := backend.DesiredState{}
		if v, ok := raw["on"].(bool); ok {
			desired.On = &v
		}
		if v, ok := raw["bri"].(float64); ok {
			b := uint8(v)
			desired.Brightness = &b
		}

		for _, entityID := range g.EntityIDs {
			be.SetState(r.Context(), entityID, desired)
		}

		var items []SuccessItem
		for key, v := range raw {
			items = append(items, SuccessItem{Success: map[string]any{
				"/groups/" + strconv.Itoa(g.HueID) + "/action/" + key: v,
			}})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}
