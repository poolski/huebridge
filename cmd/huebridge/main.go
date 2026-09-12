// Command huebridge runs the Hue Bridge emulator: CLIP v1 HTTP API, SSDP +
// mDNS discovery, and the ingress web UI, all backed by a Home Assistant
// instance reached over REST + WebSocket.
package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"huebridge/internal/backend/homeassistant"
	"huebridge/internal/discovery"
	"huebridge/internal/hue"
	"huebridge/internal/ingress"
	"huebridge/internal/registry"
	bridgetls "huebridge/internal/tls"
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

	mac := lookupMAC()
	bridgeID := bridgetls.BridgeID(mac)

	reg, err := registry.NewRegistry(filepath.Join(dataDir, "registry.json"))
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}
	whitelist := hue.NewWhitelist(filepath.Join(dataDir, "whitelist.json"))
	pairingWindow := &hue.PairingWindow{}

	be := homeassistant.New(haURL, haToken)

	mux := hue.NewServer(
		reg, be, whitelist, pairingWindow, bridgeID, mac,
		filepath.Join(dataDir, "scenes.json"),
		filepath.Join(dataDir, "schedules.json"),
	)

	scheduleStore := hue.NewScheduleStore(filepath.Join(dataDir, "schedules.json"))
	ticker := hue.NewTicker(scheduleStore, be, func(id int) (string, bool) {
		e, ok := reg.ByHueID(id)
		return e.EntityID, ok
	})
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for now := range t.C {
			ticker.Tick(now)
		}
	}()

	ingressHandler := ingress.NewHandler(reg, pairingWindow, func() []string { return nil })
	http.Handle("/ingress/", http.StripPrefix("/ingress", ingressHandler))
	http.Handle("/", mux)

	cert, err := bridgetls.GenerateCertificate(bridgeID)
	if err != nil {
		log.Fatalf("generate certificate: %v", err)
	}

	server := &http.Server{
		Addr:      ":443",
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	ip := resolveLocalIP()

	stopSSDP, err := discovery.StartSSDP(bridgeID, ip, 443)
	if err != nil {
		log.Printf("warning: SSDP discovery did not start: %v", err)
	} else {
		defer stopSSDP()
	}

	stopMDNS, err := discovery.StartMDNS(bridgeID, 443)
	if err != nil {
		log.Printf("warning: mDNS discovery did not start: %v", err)
	} else {
		defer stopMDNS()
	}

	log.Printf("huebridge starting: bridgeID=%s ip=%s", bridgeID, ip)
	log.Fatal(server.ListenAndServeTLS("", ""))
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
