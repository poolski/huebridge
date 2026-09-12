// Command huebridge runs the Hue Bridge emulator: CLIP v1 HTTP API, SSDP +
// mDNS discovery, and the ingress web UI, all backed by a Home Assistant
// instance reached over REST + WebSocket.
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
	"syscall"
	"time"

	"huebridge/internal/backend/cache"
	"huebridge/internal/backend/homeassistant"
	"huebridge/internal/discovery"
	"huebridge/internal/hue"
	"huebridge/internal/ingress"
	"huebridge/internal/registry"
	bridgetls "huebridge/internal/tls"
)

const (
	// defaultBridgePort is used when HUEBRIDGE_API_PORT isn't set. The real
	// Hue app conventionally expects the bridge on 443, but that's often
	// already taken by another host_network add-on (a reverse proxy, an SSL
	// terminator, ...), so the add-on's api_port option lets a user free
	// that up on their own terms rather than huebridge claiming it outright.
	defaultBridgePort = 8299

	// defaultIngressPort is used outside the add-on (e.g. local dev), where
	// INGRESS_PORT isn't set.
	defaultIngressPort = 8298

	tickTimeout     = 30 * time.Second
	shutdownTimeout = 10 * time.Second
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
	haURL := mustEnv("HUEBRIDGE_HA_URL")
	haToken := mustEnv("SUPERVISOR_TOKEN")

	// config.yaml sets ingress_port: 0, so Supervisor picks a free host port
	// at startup (avoiding collisions with other host_network add-ons) and
	// hands it back via INGRESS_PORT.
	ingressPort := envIntOrDefault("INGRESS_PORT", defaultIngressPort)
	bridgePort := envIntOrDefault("HUEBRIDGE_API_PORT", defaultBridgePort)

	mac := lookupMAC()
	bridgeID := bridgetls.BridgeID(mac)

	reg, err := registry.NewRegistry(filepath.Join(dataDir, "registry.json"))
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}
	whitelist := hue.NewWhitelist(filepath.Join(dataDir, "whitelist.json"))
	pairingWindow := &hue.PairingWindow{}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The Hue app polls /lights about once a second; the cache turns those
	// polls into reads of a WebSocket-fed snapshot instead of a REST call
	// per entity per poll.
	be := cache.New(homeassistant.New(haURL, haToken))
	go be.Run(ctx)

	// Scene and schedule stores are constructed once here and shared with
	// both the HTTP handlers and the ticker — two stores over one file each
	// keep their own copy of its contents and silently overwrite each
	// other.
	scenes := hue.NewSceneStore(filepath.Join(dataDir, "scenes.json"))
	schedules := hue.NewScheduleStore(filepath.Join(dataDir, "schedules.json"))

	mux := hue.NewServer(reg, be, whitelist, pairingWindow, bridgeID, mac, scenes, schedules)

	ip := resolveLocalIP()
	mux.HandleFunc("GET /description.xml", handleDescriptionXML(bridgeID, ip, bridgePort))

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
			log.Printf("list Home Assistant entities for the ingress picker: %v", err)
			return nil
		}
		return entities
	})

	bridgeMux := http.NewServeMux()
	bridgeMux.Handle("/ingress/", http.StripPrefix("/ingress", ingressHandler))
	bridgeMux.Handle("/", mux)

	cert, err := bridgetls.GenerateCertificate(bridgeID)
	if err != nil {
		log.Fatalf("generate certificate: %v", err)
	}

	bridgeServer := &http.Server{
		Addr:      fmt.Sprintf(":%d", bridgePort),
		Handler:   bridgeMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	// Supervisor's ingress proxy speaks plain HTTP to the add-on, so the
	// UI gets its own listener rather than sharing the TLS one.
	ingressServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", ingressPort),
		Handler: ingressHandler,
	}

	if stopSSDP, err := discovery.StartSSDP(bridgeID, ip, bridgePort); err != nil {
		log.Printf("warning: SSDP discovery did not start: %v", err)
	} else {
		defer stopSSDP()
	}

	if stopMDNS, err := discovery.StartMDNS(bridgeID, bridgePort); err != nil {
		log.Printf("warning: mDNS discovery did not start: %v", err)
	} else {
		defer stopMDNS()
	}

	serverErrs := make(chan error, 2)
	go func() {
		if err := bridgeServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- fmt.Errorf("bind bridge port %d (change the api_port add-on option if something else on the host already uses it): %w", bridgePort, err)
		}
	}()
	go func() {
		if err := ingressServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrs <- err
		}
	}()

	log.Printf("huebridge starting: bridgeID=%s ip=%s api=:%d ingress=:%d", bridgeID, ip, bridgePort, ingressPort)

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
	if err := ingressServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shut down ingress server: %v", err)
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
