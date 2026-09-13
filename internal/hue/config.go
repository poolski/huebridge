package hue

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"huebridge/internal/store"
)

// currentDatastoreVersion/currentSwVersion/currentAPIVersion: the official
// Hue app validates this triple against a real released firmware
// combination and, if it doesn't recognize it, nags for an update that a
// non-genuine bridge can never actually deliver — see
// docs/superpowers/notes/2026-09-13-tls-pairing-failure-log.md for the full
// investigation. Bumping this triple to match a real live bridge's reported
// values did not unblock pairing (see
// docs/superpowers/notes/2026-09-13-android-emulator-repro-log.md, Finding 2),
// so these are reverted to the original defaults pending further
// investigation. Overridable at runtime via SetVersionOverrides, or from the
// admin UI (see VersionStore) which restricts the choice to KnownVersions.
var (
	versionMu               sync.RWMutex
	currentDatastoreVersion = "126"
	currentSwVersion        = "1000000000"
	currentAPIVersion       = "1.61.0"
)

// VersionTriple is a datastoreversion/swversion/apiversion combination as
// reported by GET /api/config. The official Hue app checks these three as a
// matched set, so they're only ever changed together.
type VersionTriple struct {
	DatastoreVersion string
	SwVersion        string
	APIVersion       string
}

// KnownVersions are real Signify Hue Bridge v2 (BSB002) version triples,
// offered in the admin UI as the only values selectable there so a typo
// can't produce a combination the Hue app has never seen. Each is
// cross-checked against Signify's own release notes
// (https://www.philips-hue.com/en-us/support/release-notes/bridge, which
// publishes swversion/date but not datastoreversion or apiversion) and a
// real bridge's GET /api/config response captured by third-party projects:
//   - 70/1806051111/1.24.0 and 70/1809121051/1.24.0: diyHue's recorded
//     live-bridge capture and its own config.json fallback default; both
//     swversions match Signify's Jun 28 2018 and Oct 3 2018 releases.
//   - 94/1940042020/1.40.0: reported in openhue-cli issue #88; swversion
//     matches Signify's Aug 3 2020 release.
//   - 165/1961135030/1.61.0: homebridge-hue's ph-Tutorial wiki page;
//     swversion matches Signify's Dec 7 2023 release.
//   - 197/1978293000/1.79.0: swversion matches Signify's most recent
//     published release (Aug 27 2026, listed there as "1.79.1978293000").
//     facc766 recorded apiversion 1.78.0 for this same swversion from a
//     live bridge queried before that release; Signify's own notes are
//     trusted here as the more current source.
var KnownVersions = []VersionTriple{
	{DatastoreVersion: "70", SwVersion: "1806051111", APIVersion: "1.24.0"},
	{DatastoreVersion: "70", SwVersion: "1809121051", APIVersion: "1.24.0"},
	{DatastoreVersion: "94", SwVersion: "1940042020", APIVersion: "1.40.0"},
	{DatastoreVersion: "165", SwVersion: "1961135030", APIVersion: "1.61.0"},
	{DatastoreVersion: "197", SwVersion: "1978293000", APIVersion: "1.79.0"},
}

// isKnownVersion reports whether v is one of KnownVersions.
func isKnownVersion(v VersionTriple) bool {
	for _, k := range KnownVersions {
		if k == v {
			return true
		}
	}
	return false
}

// currentVersion returns the version triple currently reported by GET
// /api/config.
func currentVersion() VersionTriple {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return VersionTriple{
		DatastoreVersion: currentDatastoreVersion,
		SwVersion:        currentSwVersion,
		APIVersion:       currentAPIVersion,
	}
}

// setCurrentVersion replaces the version triple reported by GET
// /api/config. Unlike SetVersionOverrides, this is safe to call after the
// server has started serving requests.
func setCurrentVersion(v VersionTriple) {
	versionMu.Lock()
	defer versionMu.Unlock()
	currentDatastoreVersion, currentSwVersion, currentAPIVersion = v.DatastoreVersion, v.SwVersion, v.APIVersion
}

// SetVersionOverrides replaces currentDatastoreVersion/currentSwVersion/
// currentAPIVersion with any non-empty argument, leaving the corresponding
// default in place otherwise. Meant to be called at most once, at startup,
// before the server accepts connections — these three aren't behind a
// mutex, so mutating them after that point is a data race.
// SetVersionOverrides replaces currentDatastoreVersion/currentSwVersion/
// currentAPIVersion with any non-empty argument, leaving the corresponding
// default in place otherwise. Meant to be called at most once, at startup,
// before the server accepts connections. Unlike VersionStore.Set, it does
// not validate against KnownVersions — it exists for testing pairing
// against a specific Signify version combo without a rebuild.
func SetVersionOverrides(datastoreVersion, swVersion, apiVersion string) {
	versionMu.Lock()
	defer versionMu.Unlock()
	if datastoreVersion != "" {
		currentDatastoreVersion = datastoreVersion
	}
	if swVersion != "" {
		currentSwVersion = swVersion
	}
	if apiVersion != "" {
		currentAPIVersion = apiVersion
	}
}

// currentTimezone is the IANA zone name reported as "timezone" in
// GET /api/{username}/config and used to compute "localtime". Real bridges
// pick this up from the network at setup time; huebridge has no equivalent
// signal, so it defaults to Europe/London and is overridable via
// SetTimezoneOverride.
var (
	timezoneMu      sync.RWMutex
	currentTimezone = "Europe/London"
)

// currentTZ returns the timezone currently reported by GET
// /api/{username}/config.
func currentTZ() string {
	timezoneMu.RLock()
	defer timezoneMu.RUnlock()
	return currentTimezone
}

// SetTimezoneOverride replaces the timezone reported by GET
// /api/{username}/config. Empty tz leaves the default in place. Meant to be
// called at most once, at startup, before the server accepts connections —
// like SetVersionOverrides, mutating it after that point is a data race.
// tz isn't validated against the IANA database here; handleGetConfig falls
// back to UTC for "localtime" if it turns out not to be loadable, while
// still reporting the configured string as "timezone".
func SetTimezoneOverride(tz string) {
	if tz == "" {
		return
	}
	timezoneMu.Lock()
	defer timezoneMu.Unlock()
	currentTimezone = tz
}

func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow, wl *Whitelist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := currentVersion()

		// GET /api/config has no {username} at all, and an unrecognized
		// {username} must fall back to the same stripped response — only a
		// recognized user gets the full config (network details, portal/
		// backup state, and whitelist). Without this, some clients (e.g.
		// Hue Essentials) can't confirm their new username was actually
		// registered after POST /api succeeds, conclude pairing failed, and
		// restart the whole flow from scratch in a loop.
		var recognized bool
		if username := r.PathValue("username"); username != "" {
			_, recognized = wl.Lookup(username)
		}

		w.Header().Set("Content-Type", "application/json")

		if !recognized {
			json.NewEncoder(w).Encode(strippedBridgeConfig{
				Name:             "huebridge",
				DatastoreVersion: v.DatastoreVersion,
				SwVersion:        v.SwVersion,
				APIVersion:       v.APIVersion,
				Mac:              mac.String(),
				BridgeID:         bridgeID,
				FactoryNew:       false,
				ModelID:          "BSB002",
			})
			return
		}

		ip, netmask, gateway := localNetworkConfig()
		nowUTC := time.Now().UTC()
		tz := currentTZ()
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
		utc := nowUTC.Format("2006-01-02T15:04:05")
		localtime := nowUTC.In(loc).Format("2006-01-02T15:04:05")

		cfg := BridgeConfig{
			Name:             "huebridge",
			ZigbeeChannel:    25,
			BridgeID:         bridgeID,
			Mac:              mac.String(),
			Dhcp:             true,
			IPAddress:        ip,
			Netmask:          netmask,
			Gateway:          gateway,
			ProxyAddress:     "none",
			ProxyPort:        0,
			UTC:              utc,
			LocalTime:        localtime,
			Timezone:         tz,
			ModelID:          "BSB002",
			DatastoreVersion: v.DatastoreVersion,
			SwVersion:        v.SwVersion,
			APIVersion:       v.APIVersion,
			SwUpdate2: ConfigSwUpdate2{
				CheckForUpdate: false,
				LastChange:     utc,
				Bridge:         ConfigSwUpdate2Bridge{State: "noupdates", LastInstall: utc},
				State:          "noupdates",
				AutoInstall:    ConfigSwUpdate2AutoInstall{UpdateTime: "T04:00:00", On: true},
			},
			LinkButton:       win.IsOpen(),
			PortalServices:   false,
			AnalyticsConsent: false,
			PortalConnection: "disconnected",
			PortalState: ConfigPortalState{
				Communication: "disconnected",
			},
			InternetServices: ConfigInternetServices{
				Internet:     "disconnected",
				RemoteAccess: "disconnected",
				Time:         "disconnected",
				SwUpdate:     "disconnected",
			},
			FactoryNew:   false,
			StarterKitID: "",
			Backup:       ConfigBackup{Status: "idle", ErrorCode: 0},
			HTTPBlocked:  false,
			Whitelist:    make(map[string]ConfigWhitelistEntry),
		}
		for u, entry := range wl.All() {
			createDate := entry.CreateDate.UTC().Format("2006-01-02T15:04:05")
			cfg.Whitelist[u] = ConfigWhitelistEntry{
				Name:        entry.Name,
				CreateDate:  createDate,
				LastUseDate: createDate,
			}
		}

		json.NewEncoder(w).Encode(cfg)
	}
}

// localNetworkConfig reports the local IP address huebridge is reachable on
// (matching cmd/huebridge/main.go's resolveLocalIP approach — dialing out
// without sending data to learn which interface the OS would route through),
// a conventional /24 netmask, and a gateway guessed as that subnet's .1
// address. huebridge doesn't actually control DHCP/networking, so these are
// best-effort values for display, not authoritative network state.
func localNetworkConfig() (ip, netmask, gateway string) {
	const fallbackIP = "0.0.0.0"
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return fallbackIP, "255.255.255.0", fallbackIP
	}
	defer conn.Close()
	localIP := conn.LocalAddr().(*net.UDPAddr).IP.To4()
	if localIP == nil {
		return fallbackIP, "255.255.255.0", fallbackIP
	}
	gw := make(net.IP, len(localIP))
	copy(gw, localIP)
	gw[3] = 1
	return localIP.String(), "255.255.255.0", gw.String()
}

// versionFile is the on-disk shape of a VersionStore, persisted alongside
// the registry/whitelist/scenes/schedules JSON files so an admin-selected
// version triple survives a restart.
type versionFile struct {
	Version VersionTriple `json:"version"`
}

// VersionStore persists an admin-selected VersionTriple and applies it as
// the version reported by GET /api/config. Unset fields in the persisted
// file (e.g. no selection has ever been made) leave the compiled-in
// defaults in internal/hue/config.go untouched.
type VersionStore struct {
	mu   sync.Mutex
	file *store.JSONFile[versionFile]
}

// NewVersionStore builds a VersionStore backed by the JSON file at path.
func NewVersionStore(path string) *VersionStore {
	return &VersionStore{file: store.NewJSONFile[versionFile](path)}
}

// Load applies a previously-persisted version selection, if any, as the
// version reported by GET /api/config. Call once at startup, before
// SetVersionOverrides so an explicit env var override still wins.
func (s *VersionStore) Load() error {
	state, err := s.file.Load(versionFile{})
	if err != nil {
		return err
	}
	if state.Version != (VersionTriple{}) {
		setCurrentVersion(state.Version)
	}
	return nil
}

// Current returns the version triple currently reported by GET
// /api/config.
func (s *VersionStore) Current() VersionTriple {
	return currentVersion()
}

// Set validates v against KnownVersions, persists it, and makes it the
// version reported by GET /api/config from this point on.
func (s *VersionStore) Set(v VersionTriple) error {
	if !isKnownVersion(v) {
		return fmt.Errorf("not a known version triple: %+v", v)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.file.Save(versionFile{Version: v}); err != nil {
		return fmt.Errorf("save version selection: %w", err)
	}
	setCurrentVersion(v)
	return nil
}

// CommonTimezones are offered as suggestions in the admin UI's timezone
// picker — a starting point, not an exhaustive list; any IANA zone name
// time.LoadLocation accepts is a valid TimezoneStore.Set argument.
var CommonTimezones = []string{
	"Europe/London",
	"Europe/Dublin",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Madrid",
	"Europe/Rome",
	"Europe/Amsterdam",
	"Europe/Lisbon",
	"Europe/Athens",
	"Europe/Moscow",
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Sao_Paulo",
	"America/Toronto",
	"Asia/Tokyo",
	"Asia/Shanghai",
	"Asia/Hong_Kong",
	"Asia/Singapore",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Australia/Sydney",
	"Australia/Melbourne",
	"Pacific/Auckland",
	"UTC",
}

// timezoneFile is the on-disk shape of a TimezoneStore, persisted alongside
// the registry/whitelist/scenes/schedules/version JSON files so an
// admin-selected timezone survives a restart.
type timezoneFile struct {
	Timezone string `json:"timezone"`
}

// TimezoneStore persists an admin-selected IANA timezone name and applies it
// as the "timezone" (and, via handleGetConfig's "localtime" computation)
// reported by GET /api/{username}/config. An unset persisted file leaves
// the compiled-in Europe/London default untouched.
type TimezoneStore struct {
	mu   sync.Mutex
	file *store.JSONFile[timezoneFile]
}

// NewTimezoneStore builds a TimezoneStore backed by the JSON file at path.
func NewTimezoneStore(path string) *TimezoneStore {
	return &TimezoneStore{file: store.NewJSONFile[timezoneFile](path)}
}

// Load applies a previously-persisted timezone selection, if any. Call once
// at startup, before SetTimezoneOverride so an explicit env var override
// still wins.
func (s *TimezoneStore) Load() error {
	state, err := s.file.Load(timezoneFile{})
	if err != nil {
		return err
	}
	if state.Timezone != "" {
		timezoneMu.Lock()
		currentTimezone = state.Timezone
		timezoneMu.Unlock()
	}
	return nil
}

// Current returns the timezone currently reported by GET
// /api/{username}/config.
func (s *TimezoneStore) Current() string {
	return currentTZ()
}

// Set validates tz as a loadable IANA zone name, persists it, and makes it
// the timezone reported by GET /api/{username}/config from this point on.
func (s *TimezoneStore) Set(tz string) error {
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("not a recognized IANA timezone: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.file.Save(timezoneFile{Timezone: tz}); err != nil {
		return fmt.Errorf("save timezone selection: %w", err)
	}
	timezoneMu.Lock()
	currentTimezone = tz
	timezoneMu.Unlock()
	return nil
}
