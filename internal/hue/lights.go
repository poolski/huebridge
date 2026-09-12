package hue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"huebridge/internal/backend"
	"huebridge/internal/registry"
)

func toLight(entry registry.Entry, state backend.EntityState) Light {
	l := Light{
		Type:             "Dimmable light",
		Name:             entry.Name,
		ModelID:          "LWB010",
		ManufacturerName: "huebridge",
		UniqueID:         fmt.Sprintf("huebridge-%d", entry.HueID),
		SwVersion:        "1.0.0",
		State: LightState{
			On:        state.On,
			Reachable: state.Reachable,
			Alert:     "none",
		},
	}
	if state.Brightness != nil {
		l.State.Bri = state.Brightness
		l.Type = "Dimmable light"
	}
	if state.ColorXY != nil {
		l.State.Xy = state.ColorXY
		l.State.ColorMode = "xy"
		l.Type = "Extended color light"
	} else if state.ColorTempMirek != nil {
		l.State.Ct = state.ColorTempMirek
		l.State.ColorMode = "ct"
	}
	return l
}

func handleGetLights(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]Light{}
		for _, entry := range reg.All() {
			state, err := be.GetState(r.Context(), entry.EntityID)
			if err != nil {
				state = backend.EntityState{EntityID: entry.EntityID, Reachable: false}
			}
			out[strconv.Itoa(entry.HueID)] = toLight(entry, state)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func handleGetLight(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		entry, ok := reg.ByHueID(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		state, err := be.GetState(r.Context(), entry.EntityID)
		if err != nil {
			state = backend.EntityState{EntityID: entry.EntityID, Reachable: false}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toLight(entry, state))
	}
}

func handlePutLightState(reg *registry.Registry, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		entry, ok := reg.ByHueID(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}

		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			WriteError(w, http.StatusOK, 2, r.URL.Path, "body contains invalid JSON")
			return
		}

		desired := backend.DesiredState{}
		if v, ok := raw["on"].(bool); ok {
			desired.On = &v
		}
		if v, ok := raw["bri"].(float64); ok {
			b := uint8(v)
			desired.Brightness = &b
		}
		if v, ok := raw["xy"].([]any); ok && len(v) == 2 {
			x, _ := v[0].(float64)
			y, _ := v[1].(float64)
			xy := [2]float64{x, y}
			desired.ColorXY = &xy
		}
		if v, ok := raw["ct"].(float64); ok {
			ct := uint16(v)
			desired.ColorTempMirek = &ct
		}

		if err := be.SetState(r.Context(), entry.EntityID, desired); err != nil {
			WriteError(w, http.StatusOK, 901, r.URL.Path, "internal error")
			return
		}

		var items []SuccessItem
		base := fmt.Sprintf("/lights/%d/state/", entry.HueID)
		for _, key := range []string{"on", "bri", "hue", "sat", "xy", "ct", "transitiontime"} {
			if v, ok := raw[key]; ok {
				items = append(items, SuccessItem{Success: map[string]any{base + key: v}})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}
