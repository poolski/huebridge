// Package discovery answers SSDP and mDNS discovery broadcasts so the Hue
// app can find huebridge on the LAN, the same way it finds a real bridge.
package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/koron/go-ssdp"
)

func ssdpLocationURL(ip string, port int) string {
	return fmt.Sprintf("https://%s:%d/description.xml", ip, port)
}

// ssdpUSN builds the fixed Hue-bridge-shaped UUID the app expects, seeded
// with the bridge's first 6 and last 6 hex digits the same way diyHue/Bifrost do.
func ssdpUSN(bridgeID string) string {
	suffix := strings.ToLower(bridgeID[:6] + bridgeID[len(bridgeID)-6:])
	return fmt.Sprintf("uuid:2f402f80-da50-11e1-9b23-%s::upnp:rootdevice", suffix)
}

// StartSSDP advertises huebridge as a Hue Bridge over SSDP. It returns a
// stop function to call on shutdown.
func StartSSDP(bridgeID string, localIP string, httpsPort int) (stop func(), err error) {
	ad, err := ssdp.Advertise(
		"upnp:rootdevice",
		ssdpUSN(bridgeID),
		ssdpLocationURL(localIP, httpsPort),
		"huebridge/1.0 UPnP/1.0 IpBridge/1.61.0",
		1800,
	)
	if err != nil {
		return nil, fmt.Errorf("start SSDP advertiser: %w", err)
	}

	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(900 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				if err := ad.Alive(); err != nil {
					return
				}
			}
		}
	}()

	return func() {
		close(stopCh)
		ad.Bye()
		ad.Close()
	}, nil
}
