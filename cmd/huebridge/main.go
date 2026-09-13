// Command huebridge runs the Hue Bridge emulator: CLIP v1 HTTP API, SSDP +
// mDNS discovery, and an admin/entity-picker web UI, backed by a Home
// Assistant instance reached over REST + WebSocket. It runs either as a
// Home Assistant Supervisor add-on (SUPERVISOR_TOKEN set) or standalone,
// configured through its own setup wizard (see isStandalone).
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"huebridge/internal/backend/cache"
	"huebridge/internal/backend/homeassistant"
	"huebridge/internal/discovery"
	"huebridge/internal/hue"
	"huebridge/internal/ingress"
	"huebridge/internal/logging"
	"huebridge/internal/registry"
	"huebridge/internal/setup"
	bridgetls "huebridge/internal/tls"
)

const (
	// defaultBridgePort is used when HUEBRIDGE_API_PORT isn't set. The real
	// Hue app conventionally expects the bridge on 443, but that's often
	// already taken by another host_network add-on (a reverse proxy, an SSL
	// terminator, ...), so the api_port option lets a user free that up on
	// their own terms rather than huebridge claiming it outright.
	defaultBridgePort = 8299

	// defaultIngressPort is used if Supervisor can't be asked which port it
	// assigned (see fetchIngressPort) — ingress won't work, but the rest of
	// the bridge still can.
	defaultIngressPort = 8298

	// defaultAdminPort is standalone mode's setup-wizard/entity-picker port
	// — there's no Supervisor ingress proxy to assign one dynamically.
	defaultAdminPort = 8300

	// plainHTTPPort is fixed, not configurable: real Hue bridges serve the
	// CLIP API over plain HTTP on 80 as well as HTTPS, and the official
	// app's pairing flow depends on exactly that port — it isn't something
	// a user picks. Best-effort, like SSDP/mDNS: this commonly fails when
	// something else already owns 80 on a shared host.
	plainHTTPPort = 80

	tickTimeout          = 30 * time.Second
	shutdownTimeout      = 10 * time.Second
	supervisorAPITimeout = 10 * time.Second
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func main() {
	dataDir := envOrDefault("HUEBRIDGE_DATA_DIR", "/data")
	bridgePort := envIntOrDefault("HUEBRIDGE_API_PORT", defaultBridgePort)
	logLevel := logging.ParseLevel(os.Getenv("HUEBRIDGE_LOG_LEVEL"))

	if isStandalone(os.Getenv) {
		runStandalone(dataDir, bridgePort, logLevel)
		return
	}
	runAddon(dataDir, bridgePort, logLevel)
}

// runAddon is huebridge's original entry point: Supervisor supplies the HA
// URL/token via env vars and the ingress port via its API, and its ingress
// proxy already authenticates access to the admin UI, so it's served bare.
func runAddon(dataDir string, bridgePort int, logLevel logging.Level) {
	haURL := mustEnv("HUEBRIDGE_HA_URL")
	haToken := mustEnv("SUPERVISOR_TOKEN")

	// config.yaml sets ingress_port: 0 so Supervisor assigns a free host
	// port at install time (avoiding a collision with other host_network
	// add-ons), but it doesn't pass that port to the container by any
	// other means — Supervisor's own docs say to read it back via its API.
	ingressPort := defaultIngressPort
	{
		fetchCtx, cancel := context.WithTimeout(context.Background(), supervisorAPITimeout)
		port, err := fetchIngressPort(fetchCtx, http.DefaultClient, "http://supervisor", haToken)
		cancel()
		if err != nil {
			log.Printf("warning: fetch assigned ingress port from Supervisor: %v (falling back to %d, ingress will likely not work)", err, defaultIngressPort)
		} else {
			ingressPort = port
		}
	}

	runBridge(bridgeDeps{
		dataDir:    dataDir,
		haURL:      haURL,
		haToken:    haToken,
		bridgePort: bridgePort,
		adminPort:  ingressPort,
		adminTLS:   false,
		wrapAdmin:  func(h http.Handler) http.Handler { return h },
		logLevel:   logLevel,
	})
}

// runStandalone is the new entry point for running without Supervisor. It
// loads (or, on first run, collects via the setup wizard) the HA URL/token
// and admin password, then serves the same bridge runAddon does, except
// the admin UI gets its own TLS listener and HTTP Basic Auth since there's
// no Supervisor ingress proxy providing either.
func runStandalone(dataDir string, bridgePort int, logLevel logging.Level) {
	adminPort := envIntOrDefault("HUEBRIDGE_ADMIN_PORT", defaultAdminPort)
	cfgStore := setup.NewStore(filepath.Join(dataDir, "standalone.json"))

	cfg, err := cfgStore.Load()
	if err != nil {
		log.Fatalf("load standalone config: %v", err)
	}
	if !cfg.Complete() {
		cfg = runSetupWizard(cfgStore, adminPort)
	}

	runBridge(bridgeDeps{
		dataDir:    dataDir,
		haURL:      cfg.HAURL,
		haToken:    cfg.HAToken,
		bridgePort: bridgePort,
		adminPort:  adminPort,
		adminTLS:   true,
		wrapAdmin: func(h http.Handler) http.Handler {
			return setup.RequireAdmin(h, func() []byte { return cfg.AdminPasswordHash })
		},
		logLevel: logLevel,
	})
}

// runSetupWizard blocks, serving the first-run setup UI over HTTPS on
// adminPort, until a working config has been collected and persisted. The
// wizard's own cert is throwaway — bridgeID only needs to be stable once
// runBridge starts, not during setup.
func runSetupWizard(cfgStore *setup.Store, adminPort int) setup.Config {
	mac := lookupMAC()
	cert, err := bridgetls.GenerateCertificate(bridgetls.BridgeID(mac))
	if err != nil {
		log.Fatalf("generate setup wizard certificate: %v", err)
	}

	wizard := setup.NewWizard(cfgStore, discovery.DiscoverHomeAssistant)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", adminPort),
		Handler:   wizard.Handler(),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("setup wizard server: %v", err)
		}
	}()

	log.Printf("huebridge is not configured yet — open https://<this host>:%d/ to set it up", adminPort)
	<-wizard.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shut down setup wizard server: %v", err)
	}

	cfg, err := cfgStore.Load()
	if err != nil {
		log.Fatalf("reload standalone config after setup: %v", err)
	}
	// Belt-and-suspenders: the wizard should never signal Done() without
	// having persisted a complete config (see handleHASubmit's own
	// validation), but if it ever did, starting the bridge with an empty
	// HA token would fail silently and, since the file now exists, the
	// wizard would never run again to let the operator recover. Fail loud
	// instead.
	if !cfg.Complete() {
		log.Fatalf("setup wizard completed but produced an incomplete config — this should not happen")
	}
	return cfg
}

// bridgeDeps is what runBridge needs to serve the bridge, gathered
// differently by runAddon and runStandalone.
type bridgeDeps struct {
	dataDir    string
	haURL      string
	haToken    string
	bridgePort int
	adminPort  int
	// adminTLS selects whether the admin server terminates TLS itself
	// (standalone, no proxy in front of it) or speaks plain HTTP
	// (add-on, where Supervisor's ingress proxy terminates TLS upstream).
	adminTLS  bool
	wrapAdmin func(http.Handler) http.Handler
	logLevel  logging.Level
}

// runBridge wires up and serves the Hue bridge: backend, Hue CLIP v1 API,
// SSDP/mDNS advertising, and the admin/entity-picker UI. Shared by both
// entry points, which differ only in how they obtain deps. Blocks until
// shutdown.
func runBridge(deps bridgeDeps) {
	logMiddleware := logging.Middleware(log.Default(), deps.logLevel)
	// The admin/ingress UI's requests and HTML responses are never useful to
	// dump at debug level — they're the user's own browser traffic, not the
	// Hue app's, and the response bodies are just page HTML. Always log
	// them at the plain one-line level regardless of HUEBRIDGE_LOG_LEVEL.
	adminLogMiddleware := logging.Middleware(log.Default(), logging.LevelInfo)

	mac := lookupMAC()
	bridgeID := bridgetls.BridgeID(mac)

	reg, err := registry.NewRegistry(filepath.Join(deps.dataDir, "registry.json"))
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}
	whitelist := hue.NewWhitelist(filepath.Join(deps.dataDir, "whitelist.json"))
	pairingWindow := &hue.PairingWindow{}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The Hue app polls /lights about once a second; the cache turns those
	// polls into reads of a WebSocket-fed snapshot instead of a REST call
	// per entity per poll.
	be := cache.New(homeassistant.New(deps.haURL, deps.haToken))
	go be.Run(ctx)

	// Scene and schedule stores are constructed once here and shared with
	// both the HTTP handlers and the ticker — two stores over one file each
	// keep their own copy of its contents and silently overwrite each
	// other.
	scenes := hue.NewSceneStore(filepath.Join(deps.dataDir, "scenes.json"))
	schedules := hue.NewScheduleStore(filepath.Join(deps.dataDir, "schedules.json"))

	ip := resolveLocalIP()
	mux := hue.NewServer(reg, be, whitelist, pairingWindow, bridgeID, mac, scenes, schedules, ip)

	mux.HandleFunc("GET /description.xml", handleDescriptionXML(bridgeID, ip, deps.bridgePort))

	ticker := hue.NewTicker(schedules, be, func(id int) (string, bool) {
		e, ok := reg.ByHueID(id)
		return e.EntityID, ok
	})
	go runTicker(ctx, ticker)

	ingressHandler := ingress.NewHandler(reg, pairingWindow, func() []string {
		listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		entities, err := be.ListEntities(listCtx)
		if err != nil {
			log.Printf("list Home Assistant entities for the admin picker: %v", err)
			return nil
		}
		// The Hue bridge this add-on emulates only understands lights, so
		// offering every HA domain in the picker would just bury the ones
		// that matter.
		lights := make([]string, 0, len(entities))
		for _, id := range entities {
			if strings.HasPrefix(id, "light.") {
				lights = append(lights, id)
			}
		}
		return lights
	})

	bridgeMux := http.NewServeMux()
	bridgeMux.Handle("/ingress/", deps.wrapAdmin(adminLogMiddleware(http.StripPrefix("/ingress", ingressHandler))))
	bridgeMux.Handle("/", logMiddleware(mux))

	cert, err := bridgetls.LoadOrGenerateCertificate(filepath.Join(deps.dataDir, "cert.pem"), bridgeID)
	if err != nil {
		log.Fatalf("load or generate certificate: %v", err)
	}

	bridgeServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", deps.bridgePort),
		Handler: bridgeMux,
		// A real Hue bridge's firmware only ever speaks HTTP/1.1; net/http
		// auto-negotiates h2 over TLS otherwise, which the official app's
		// TLS stack has been observed aborting the handshake over.
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"http/1.1"},
			// TEMPORARY: the official Hue app's TLS handshake to this port
			// aborts with a bare EOF (no alert) right after a successful
			// plain-HTTP GET /api/config, and three targeted guesses at why
			// (cert extensions, disabling h2, cert persistence) haven't
			// changed that. Log what it actually offers in its ClientHello
			// instead of guessing further. Remove once the cause is found.
			GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
				log.Printf("TLS ClientHello from %s: server_name=%q versions=%v ciphers=%v curves=%v points=%v alpn=%v",
					hello.Conn.RemoteAddr(), hello.ServerName, hello.SupportedVersions, hello.CipherSuites,
					hello.SupportedCurves, hello.SupportedPoints, hello.SupportedProtos)
				return nil, nil
			},
		},
	}
	adminServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", deps.adminPort),
		Handler: adminLogMiddleware(deps.wrapAdmin(ingressHandler)),
	}
	if deps.adminTLS {
		adminServer.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	// The Hue CLIP API only — not bridgeMux's /ingress/ admin route, which
	// carries Basic Auth credentials in standalone mode that must never go
	// out over plaintext.
	plainHTTPServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", plainHTTPPort),
		Handler: logMiddleware(mux),
	}
	plainHTTPListener, err := net.Listen("tcp", plainHTTPServer.Addr)
	if err != nil {
		log.Printf("warning: plain HTTP on port %d did not start (needed for the official Hue app's pairing flow, not for third-party apps that follow the SSDP-advertised port): %v", plainHTTPPort, err)
		plainHTTPListener = nil
	}

	if stopSSDP, err := discovery.StartSSDP(bridgeID, ip, deps.bridgePort); err != nil {
		log.Printf("warning: SSDP discovery did not start: %v", err)
	} else {
		defer stopSSDP()
	}

	if stopMDNS, err := discovery.StartMDNS(bridgeID, ip, deps.bridgePort); err != nil {
		log.Printf("warning: mDNS discovery did not start: %v", err)
	} else {
		defer stopMDNS()
	}

	serverErrs := make(chan error, 3)
	go func() {
		if err := bridgeServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- fmt.Errorf("bind bridge port %d (change the api_port add-on option if something else on the host already uses it): %w", deps.bridgePort, err)
		}
	}()
	if plainHTTPListener != nil {
		go func() {
			if err := plainHTTPServer.Serve(plainHTTPListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrs <- err
			}
		}()
	}
	go func() {
		var err error
		if deps.adminTLS {
			err = adminServer.ListenAndServeTLS("", "")
		} else {
			err = adminServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- err
		}
	}()

	log.Printf("huebridge starting: bridgeID=%s ip=%s api=:%d admin=:%d log_level=%s", bridgeID, ip, deps.bridgePort, deps.adminPort, os.Getenv("HUEBRIDGE_LOG_LEVEL"))

	select {
	case err := <-serverErrs:
		log.Printf("http server failed: %v", err)
	case <-ctx.Done():
		log.Print("shutting down")
	}

	// Stop reacting to further signals so a second Ctrl-C can still kill a
	// wedged shutdown.
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := bridgeServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shut down api server: %v", err)
	}
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shut down admin server: %v", err)
	}
	if plainHTTPListener != nil {
		if err := plainHTTPServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shut down plain http server: %v", err)
		}
	}
	// The deferred stopSSDP/stopMDNS run from here, which the previous
	// log.Fatal(ListenAndServeTLS(...)) skipped entirely.
}

// runTicker fires due schedules once a minute, giving each tick its own
// bounded context so a wedged Home Assistant can't stall the loop forever.
func runTicker(ctx context.Context, ticker *hue.Ticker) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			tickCtx, cancel := context.WithTimeout(ctx, tickTimeout)
			ticker.Tick(tickCtx, now)
			cancel()
		}
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("environment variable %s must be an integer, got %q", key, v)
	}
	return n
}

// lookupMAC finds the first non-loopback interface's MAC address to derive
// the bridge id from, matching diyHue/Bifrost's approach.
func lookupMAC() net.HardwareAddr {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatalf("list network interfaces: %v", err)
	}
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) == 6 && iface.Flags&net.FlagLoopback == 0 {
			return iface.HardwareAddr
		}
	}
	log.Fatal("no suitable network interface found to derive a bridge id from")
	return nil
}

func resolveLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
