package discovery

import (
	"fmt"
	"net"

	"github.com/hashicorp/mdns"
)

func mdnsInstanceName(bridgeID string) string {
	return fmt.Sprintf("Philips Hue - %s", bridgeID)
}

// mdnsTXTRecords builds the TXT record a real bridge advertises alongside
// _hue._tcp. The Hue app's local search silently drops any advertiser
// missing modelid, even one that otherwise responds correctly to SSDP.
func mdnsTXTRecords(bridgeID string) []string {
	return []string{"bridgeid=" + bridgeID, "modelid=BSB002"}
}

// StartMDNS advertises huebridge over mDNS (_hue._tcp), the second
// discovery path Signify's own docs list alongside SSDP. Returns a stop
// function to call on shutdown.
//
// localIP is passed explicitly rather than left for the library to work
// out via the container's own hostname: NewMDNSService falls back to
// net.LookupIP(os.Hostname()) when given no IPs, which fails inside a
// plain (non-host-network) Docker container — nothing resolves a random
// container ID like "d546ca67ced4" to its own address.
func StartMDNS(bridgeID string, localIP string, httpsPort int) (stop func(), err error) {
	info := mdnsTXTRecords(bridgeID)
	service, err := mdns.NewMDNSService(
		mdnsInstanceName(bridgeID),
		"_hue._tcp",
		"", "",
		httpsPort,
		[]net.IP{net.ParseIP(localIP)},
		info,
	)
	if err != nil {
		return nil, fmt.Errorf("build mdns service: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("start mdns server: %w", err)
	}

	return func() { server.Shutdown() }, nil
}
