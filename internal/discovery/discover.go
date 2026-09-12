package discovery

import (
	"fmt"
	"time"

	"github.com/hashicorp/mdns"
)

// homeAssistantURLs turns raw mDNS service entries for _home-assistant._tcp
// into candidate base URLs, skipping any entry missing an address or port.
func homeAssistantURLs(entries []*mdns.ServiceEntry) []string {
	urls := make([]string, 0, len(entries))
	for _, e := range entries {
		ip := e.AddrV4
		if ip == nil {
			ip = e.AddrV6
		}
		if ip == nil || e.Port == 0 {
			continue
		}
		urls = append(urls, fmt.Sprintf("http://%s:%d", ip, e.Port))
	}
	return urls
}

// DiscoverHomeAssistant browses for _home-assistant._tcp on the LAN (the
// service HA Core itself advertises) and returns candidate base URLs to
// prefill the setup wizard's HA URL field. An empty result is not an
// error — the user typing the URL in by hand is always a valid outcome,
// so callers should never treat "found nothing" as fatal.
func DiscoverHomeAssistant(timeout time.Duration) ([]string, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 8)
	var entries []*mdns.ServiceEntry
	collected := make(chan struct{})
	go func() {
		for e := range entriesCh {
			entries = append(entries, e)
		}
		close(collected)
	}()

	params := mdns.DefaultParams("_home-assistant._tcp")
	params.Timeout = timeout
	params.Entries = entriesCh
	// Docker's default bridge network has no IPv6 multicast route, so
	// querying over IPv6 there just logs a scary-looking (but harmless)
	// "Failed to bind to udp6 port" error on every standalone run. IPv4
	// mDNS is what home networks actually use for this, so skip IPv6
	// outright rather than let the underlying library fail into it.
	params.DisableIPv6 = true

	err := mdns.Query(params)
	close(entriesCh)
	<-collected
	if err != nil {
		return nil, fmt.Errorf("browse for home assistant: %w", err)
	}
	return homeAssistantURLs(entries), nil
}
