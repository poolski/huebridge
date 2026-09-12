package hue

// LightState is the CLIP v1 "state" object. See
// docs/superpowers/specs/hue-api-v1-openapi.json, schema LightState, and
// hue-clip-v1-api-reference.md's "Lights" section for field semantics.
type LightState struct {
	On        bool       `json:"on"`
	Bri       *uint8     `json:"bri,omitempty"`
	Xy        *[2]float64 `json:"xy,omitempty"`
	Ct        *uint16    `json:"ct,omitempty"`
	ColorMode string     `json:"colormode,omitempty"`
	Alert     string     `json:"alert,omitempty"`
	Reachable bool       `json:"reachable"`
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
