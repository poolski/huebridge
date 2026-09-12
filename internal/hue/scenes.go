package hue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"huebridge/internal/backend"
	"huebridge/internal/registry"
	"huebridge/internal/store"
)

type StoredScene struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Group       string                          `json:"group"`
	Lights      []string                        `json:"lights"`
	LightStates map[string]backend.DesiredState `json:"light_states"`
}

type sceneFile struct {
	Scenes map[string]StoredScene `json:"scenes"`
}

type SceneStore struct {
	mu    sync.Mutex
	file  *store.JSONFile[sceneFile]
	state sceneFile
}

func NewSceneStore(path string) *SceneStore {
	file := store.NewJSONFile[sceneFile](path)
	state, _ := file.Load(sceneFile{Scenes: map[string]StoredScene{}})
	if state.Scenes == nil {
		state.Scenes = map[string]StoredScene{}
	}
	return &SceneStore{file: file, state: state}
}

func (s *SceneStore) Create(name, group string, lights []string, lightStates map[string]backend.DesiredState) (StoredScene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idBytes := make([]byte, 8)
	rand.Read(idBytes)

	scene := StoredScene{
		ID:          hex.EncodeToString(idBytes),
		Name:        name,
		Group:       group,
		Lights:      lights,
		LightStates: lightStates,
	}
	s.state.Scenes[scene.ID] = scene
	return scene, s.file.Save(s.state)
}

func (s *SceneStore) Get(id string) (StoredScene, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.state.Scenes[id]
	return sc, ok
}

func (s *SceneStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.Scenes, id)
	return s.file.Save(s.state)
}

func (s *SceneStore) All() []StoredScene {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StoredScene, 0, len(s.state.Scenes))
	for _, sc := range s.state.Scenes {
		out = append(out, sc)
	}
	return out
}

func toSceneLightStates(lightStates map[string]backend.DesiredState) map[string]SceneLightState {
	out := map[string]SceneLightState{}
	for entityID, desired := range lightStates {
		sls := SceneLightState{}
		if desired.On != nil {
			sls.On = *desired.On
		}
		sls.Bri = desired.Brightness
		out[entityID] = sls
	}
	return out
}

func toScene(s StoredScene) Scene {
	return Scene{
		Name:        s.Name,
		Type:        "GroupScene",
		Group:       s.Group,
		Lights:      s.Lights,
		LightStates: toSceneLightStates(s.LightStates),
		Owner:       "huebridge",
		Recycle:     false,
		Locked:      false,
	}
}

func handleGetScenes(scenes *SceneStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]Scene{}
		for _, s := range scenes.All() {
			out[s.ID] = toScene(s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func handleGetScene(scenes *SceneStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		s, ok := scenes.Get(id)
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toScene(s))
	}
}

func handlePostScene(reg *registry.Registry, be backend.Backend, scenes *SceneStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusOK, 2, r.URL.Path, "body contains invalid JSON")
			return
		}

		g, ok := reg.GroupByHueID(atoiOrZero(req.Group))
		if !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+req.Group+", not available")
			return
		}

		lightStates := map[string]backend.DesiredState{}
		for _, entityID := range g.EntityIDs {
			state, err := be.GetState(r.Context(), entityID)
			if err != nil {
				continue
			}
			on := state.On
			lightStates[entityID] = backend.DesiredState{On: &on, Brightness: state.Brightness}
		}

		scene, err := scenes.Create(req.Name, req.Group, g.EntityIDs, lightStates)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, 901, r.URL.Path, "internal error")
			return
		}

		// A scene that exists Hue-side with no HA counterpart defeats the
		// point of mirroring, so a mirror failure fails the whole create:
		// we roll the scene back out of the store and report CLIP 901,
		// leaving the app free to retry.
		if mirror, ok := be.(backend.SceneMirror); ok {
			if err := mirror.MirrorScene(r.Context(), scene.ID, scene.Name, lightStates); err != nil {
				log.Printf("mirror scene %s (%s) into Home Assistant: %v", scene.ID, scene.Name, err)
				if delErr := scenes.Delete(scene.ID); delErr != nil {
					log.Printf("roll back unmirrored scene %s: %v", scene.ID, delErr)
				}
				WriteError(w, http.StatusOK, 901, r.URL.Path, "internal error, scene could not be mirrored to Home Assistant")
				return
			}
		}

		WriteSuccess(w, map[string]any{"id": scene.ID})
	}
}

func handleDeleteScene(scenes *SceneStore, be backend.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := scenes.Get(id); !ok {
			WriteError(w, http.StatusOK, 3, r.URL.Path, "resource, "+r.URL.Path+", not available")
			return
		}

		// Unlike create, a failed mirror-delete does not fail the request:
		// the user asked for the scene to go away, and a leftover HA scene
		// is a smaller problem than a scene the app can't get rid of. Log
		// it and carry on.
		if mirror, ok := be.(backend.SceneMirror); ok {
			if err := mirror.DeleteMirroredScene(r.Context(), id); err != nil {
				log.Printf("delete mirrored scene %s from Home Assistant: %v", id, err)
			}
		}
		if err := scenes.Delete(id); err != nil {
			log.Printf("delete scene %s: %v", id, err)
			WriteError(w, http.StatusOK, 901, r.URL.Path, "internal error")
			return
		}

		WriteSuccess(w, map[string]any{"id": id})
	}
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
