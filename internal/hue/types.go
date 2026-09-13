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
	Sensors    []string    `json:"sensors"`
	Type       string      `json:"type"`
	Class      string      `json:"class"`
	GroupState GroupState  `json:"state"`
	Recycle    bool        `json:"recycle"`
	Action     GroupAction `json:"action"`
}

// ConfigWhitelistEntry is one entry of BridgeConfig.Whitelist. The
// space-containing JSON keys match the real bridge's wire format exactly
// (see hue-clip-v1-api-reference.md, "Config").
type ConfigWhitelistEntry struct {
	CreateDate     string `json:"create date"`
	LastUseDate    string `json:"last use date"`
	Name           string `json:"name"`
	LastAccessType string `json:"lastaccesstype"`
}

// SwUpdateDeviceTypes and SwUpdate mirror the real bridge's legacy
// (v1) software-update status object.
type SwUpdateDeviceTypes struct {
	Bridge  bool     `json:"bridge"`
	Lights  []string `json:"lights"`
	Sensors []string `json:"sensors"`
}

type SwUpdate struct {
	CheckForUpdate bool                `json:"checkforupdate"`
	DeviceTypes    SwUpdateDeviceTypes `json:"devicetypes"`
	Notify         bool                `json:"notify"`
	Text           string              `json:"text"`
	UpdateState    int                 `json:"updatestate"`
	URL            string              `json:"url"`
}

// SwUpdate2 mirrors the current software-update status object. State
// "noupdates" tells the app there's nothing to check or download —
// without it, clients have been observed treating the field's absence as
// an update being available and prompting to install one huebridge has
// nowhere real to fetch.
type SwUpdate2AutoInstall struct {
	On         bool   `json:"on"`
	UpdateTime string `json:"updatetime"`
}

type SwUpdate2Bridge struct {
	LastInstall string `json:"lastinstall"`
	State       string `json:"state"`
}

type SwUpdate2 struct {
	AutoInstall    SwUpdate2AutoInstall `json:"autoinstall"`
	Bridge         SwUpdate2Bridge      `json:"bridge"`
	CheckForUpdate bool                 `json:"checkforupdate"`
	LastChange     string               `json:"lastchange"`
	State          string               `json:"state"`
}

type BridgeConfig struct {
	Name             string                          `json:"name"`
	DatastoreVersion string                          `json:"datastoreversion"`
	SwVersion        string                          `json:"swversion"`
	APIVersion       string                          `json:"apiversion"`
	Mac              string                          `json:"mac"`
	BridgeID         string                          `json:"bridgeid"`
	FactoryNew       bool                            `json:"factorynew"`
	ReplacesBridgeID *string                         `json:"replacesbridgeid"`
	ModelID          string                          `json:"modelid"`
	StarterKitID     string                          `json:"starterkitid"`
	ZigbeeChannel    int                             `json:"zigbeechannel"`
	LinkButton       bool                            `json:"linkbutton"`
	UTC              string                          `json:"UTC"`
	LocalTime        string                          `json:"localtime"`
	Dhcp             bool                            `json:"dhcp"`
	IPAddress        string                          `json:"ipaddress"`
	ProxyAddress     string                          `json:"proxyaddress"`
	ProxyPort        int                             `json:"proxyport"`
	SwUpdate         SwUpdate                        `json:"swupdate"`
	SwUpdate2        SwUpdate2                       `json:"swupdate2"`
	Whitelist        map[string]ConfigWhitelistEntry `json:"whitelist"`
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
	AppData     map[string]any             `json:"appdata"`
	Picture     string                     `json:"picture"`
}

type ScheduleCommand struct {
	Address string `json:"address"`
	Method  string `json:"method"`
	Body    any    `json:"body"`
}

type Schedule struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Command     ScheduleCommand `json:"command"`
	LocalTime   string          `json:"localtime"`
	Status      string          `json:"status"`
}
