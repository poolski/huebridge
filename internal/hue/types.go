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
	ZigbeeChannel    int                             `json:"zigbeechannel"`
	BridgeID         string                          `json:"bridgeid"`
	Mac              string                          `json:"mac"`
	Dhcp             bool                            `json:"dhcp"`
	IPAddress        string                          `json:"ipaddress"`
	Netmask          string                          `json:"netmask"`
	Gateway          string                          `json:"gateway"`
	ProxyAddress     string                          `json:"proxyaddress"`
	ProxyPort        int                             `json:"proxyport"`
	UTC              string                          `json:"UTC"`
	LocalTime        string                          `json:"localtime"`
	Timezone         string                          `json:"timezone"`
	ModelID          string                          `json:"modelid"`
	DatastoreVersion string                          `json:"datastoreversion"`
	SwVersion        string                          `json:"swversion"`
	APIVersion       string                          `json:"apiversion"`
	SwUpdate2        ConfigSwUpdate2                 `json:"swupdate2"`
	LinkButton       bool                            `json:"linkbutton"`
	PortalServices   bool                            `json:"portalservices"`
	AnalyticsConsent bool                            `json:"analyticsconsent"`
	PortalConnection string                          `json:"portalconnection"`
	PortalState      ConfigPortalState               `json:"portalstate"`
	InternetServices ConfigInternetServices          `json:"internetservices"`
	FactoryNew       bool                            `json:"factorynew"`
	ReplacesBridgeID *string                         `json:"replacesbridgeid"`
	StarterKitID     string                          `json:"starterkitid"`
	Backup           ConfigBackup                    `json:"backup"`
	HTTPBlocked      bool                            `json:"httpblocked"`
	Whitelist        map[string]ConfigWhitelistEntry `json:"whitelist,omitempty"`
}

// strippedBridgeConfig is the fixed 10-field response real bridges return
// from GET /api/config (no {username}) and from GET /api/{username}/config
// for an unrecognized username — no network, portal, backup, or whitelist
// details. See docs/superpowers/specs/hue-clip-v1-api-reference.md, "Config".
type strippedBridgeConfig struct {
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

// ConfigSwUpdate2Bridge is the "bridge" sub-object of ConfigSwUpdate2.
type ConfigSwUpdate2Bridge struct {
	State       string `json:"state"`
	LastInstall string `json:"lastinstall"`
}

// ConfigSwUpdate2AutoInstall is the "autoinstall" sub-object of
// ConfigSwUpdate2.
type ConfigSwUpdate2AutoInstall struct {
	UpdateTime string `json:"updatetime"`
	On         bool   `json:"on"`
}

// ConfigSwUpdate2 is the "swupdate2" field of BridgeConfig, reporting
// firmware update check/install state.
type ConfigSwUpdate2 struct {
	CheckForUpdate bool                       `json:"checkforupdate"`
	LastChange     string                     `json:"lastchange"`
	Bridge         ConfigSwUpdate2Bridge      `json:"bridge"`
	State          string                     `json:"state"`
	AutoInstall    ConfigSwUpdate2AutoInstall `json:"autoinstall"`
}

// ConfigPortalState is the "portalstate" field of BridgeConfig, reporting
// remote/cloud portal connection state.
type ConfigPortalState struct {
	SignedOn      bool   `json:"signedon"`
	Incoming      bool   `json:"incoming"`
	Outgoing      bool   `json:"outgoing"`
	Communication string `json:"communication"`
}

// ConfigInternetServices is the "internetservices" field of BridgeConfig,
// reporting connectivity of various cloud-dependent services.
type ConfigInternetServices struct {
	Internet     string `json:"internet"`
	RemoteAccess string `json:"remoteaccess"`
	Time         string `json:"time"`
	SwUpdate     string `json:"swupdate"`
}

// ConfigBackup is the "backup" field of BridgeConfig, reporting the status
// of the last backup/restore operation.
type ConfigBackup struct {
	Status    string `json:"status"`
	ErrorCode int    `json:"errorcode"`
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
