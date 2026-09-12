package discovery

import (
	"fmt"

	"github.com/hashicorp/mdns"
)

func mdnsInstanceName(bridgeID string) string {
	return fmt.Sprintf("Philips Hue - %s", bridgeID)
}

// StartMDNS advertises huebridge over mDNS (_hue._tcp), the second
// discovery path Signify's own docs list alongside SSDP. Returns a stop
// function to call on shutdown.
func StartMDNS(bridgeID string, httpsPort int) (stop func(), err error) {
	info := []string{"bridgeid=" + bridgeID}
	service, err := mdns.NewMDNSService(
		mdnsInstanceName(bridgeID),
		"_hue._tcp",
		"", "",
		httpsPort,
		nil,
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
