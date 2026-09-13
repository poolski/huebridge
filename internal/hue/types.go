package hue

// LightState is the CLIP v1 "state" object. See
// docs/superpowers/specs/hue-api-v1-openapi.json, schema LightState, and
// hue-clip-v1-api-reference.md's "Lights" section for field semantics.
type LightState struct {
	On        bool        `json:"on"`
	Bri       *uint8      `json:"bri,omitempty"`
	Xy        *[2]float64 `json:"xy,omitempty"`
	Ct        *uint16     `json:"ct,omitempty"`
	ColorMode string      `json:"colormode,omitempty"`
	Alert     string      `json:"alert,omitempty"`
	Reachable bool        `json:"reachable"`
}

type Light struct {
	State            LightState `json:"state"`
	Type             string     `json:"type"`
	Name             string     `json:"name"`
	ModelID          string     `json:"modelid"`
	ManufacturerName string     `json:"manufacturername"`
	UniqueID         string     `json:"uniqueid"`
	SwVersion        string     `json:"swversion"`
}

type GroupState struct {
	AllOn bool `json:"all_on"`
	AnyOn bool `json:"any_on"`
}

type GroupAction struct {
	On  bool   `json:"on"`
	Bri *uint8 `json:"bri,omitempty"`
}

type Group struct {
	Name       string      `json:"name"`
	Lights     []string    `json:"lights"`
	Type       string      `json:"type"`
	Class      string      `json:"class"`
	GroupState GroupState  `json:"state"`
	Action     GroupAction `json:"action"`
}

type BridgeConfig struct {
	Name             string                          `json:"name"`
	DatastoreVersion string                          `json:"datastoreversion"`
	SwVersion        string                          `json:"swversion"`
	APIVersion       string                          `json:"apiversion"`
	Mac              string                          `json:"mac"`
	BridgeID         string                          `json:"bridgeid"`
	FactoryNew       bool                            `json:"factorynew"`
	ModelID          string                          `json:"modelid"`
	ZigbeeChannel    int                             `json:"zigbeechannel"`
	LinkButton       bool                            `json:"linkbutton"`
	Whitelist        map[string]ConfigWhitelistEntry `json:"whitelist,omitempty"`
}

// ConfigWhitelistEntry is how a real bridge reports each paired app in
// GET /api/{username}/config's "whitelist" map. Some clients (e.g. Hue
// Essentials) check that their own username appears here to confirm
// pairing actually completed; the JSON keys have literal spaces, matching
// the real bridge's wire format (see
// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config").
type ConfigWhitelistEntry struct {
	Name        string `json:"name"`
	CreateDate  string `json:"create date"`
	LastUseDate string `json:"last use date"`
}

type SceneLightState struct {
	On  bool   `json:"on"`
	Bri *uint8 `json:"bri,omitempty"`
}

type Scene struct {
	ID          string                     `json:"-"`
	Name        string                     `json:"name"`
	Type        string                     `json:"type"`
	Group       string                     `json:"group,omitempty"`
	Lights      []string                   `json:"lights"`
	LightStates map[string]SceneLightState `json:"lightstates"`
	Owner       string                     `json:"owner"`
	Recycle     bool                       `json:"recycle"`
	Locked      bool                       `json:"locked"`
}

type ScheduleCommand struct {
	Address string `json:"address"`
	Method  string `json:"method"`
	Body    any    `json:"body"`
}

type Schedule struct {
	Name      string          `json:"name"`
	Command   ScheduleCommand `json:"command"`
	LocalTime string          `json:"localtime"`
	Status    string          `json:"status"`
}
