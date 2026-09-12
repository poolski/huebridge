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
	Name             string `json:"name"`
	DatastoreVersion string `json:"datastoreversion"`
	SwVersion        string `json:"swversion"`
	APIVersion       string `json:"apiversion"`
	Mac              string `json:"mac"`
	BridgeID         string `json:"bridgeid"`
	FactoryNew       bool   `json:"factorynew"`
	ModelID          string `json:"modelid"`
	ZigbeeChannel    int    `json:"zigbeechannel"`
	LinkButton       bool   `json:"linkbutton"`
}

// PublicBridgeConfig is the stripped config a real bridge returns from
// GET /api/config — no username, no auth — with no whitelist or network
// fields. Apps hit this to identify a bridge before pairing. See
// docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config".
type PublicBridgeConfig struct {
	Name             string  `json:"name"`
	DatastoreVersion string  `json:"datastoreversion"`
	SwVersion        string  `json:"swversion"`
	APIVersion       string  `json:"apiversion"`
	Mac              string  `json:"mac"`
	BridgeID         string  `json:"bridgeid"`
	FactoryNew       bool    `json:"factorynew"`
	ReplacesBridgeID *string `json:"replacesbridgeid"`
	ModelID          string  `json:"modelid"`
	StarterKitID     string  `json:"starterkitid"`
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
