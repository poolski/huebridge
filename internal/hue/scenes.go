package hue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

		if mirror, ok := be.(backend.SceneMirror); ok {
			mirror.MirrorScene(r.Context(), scene.ID, scene.Name, lightStates)
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

		if mirror, ok := be.(backend.SceneMirror); ok {
			mirror.DeleteMirroredScene(r.Context(), id)
		}
		scenes.Delete(id)

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
