package main

import (
	"fmt"
	"net/http"
	"strings"
)

// descriptionXMLTemplate is the UPnP 1.0 root device descriptor SSDP points
// at (LOCATION: https://<ip>:<port>/description.xml). The Hue app fetches it
// during discovery and matches on modelName/manufacturer, so the field
// values mirror what a real bridge returns rather than naming huebridge.
const descriptionXMLTemplate = `<?xml version="1.0" encoding="UTF-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
<specVersion>
<major>1</major>
<minor>0</minor>
</specVersion>
<URLBase>https://%s:%d/</URLBase>
<device>
<deviceType>urn:schemas-upnp-org:device:Basic:1</deviceType>
<friendlyName>Philips hue (%s)</friendlyName>
<manufacturer>Signify</manufacturer>
<manufacturerURL>http://www.philips-hue.com</manufacturerURL>
<modelDescription>Philips hue Personal Wireless Lighting</modelDescription>
<modelName>Philips hue bridge 2015</modelName>
<modelNumber>BSB002</modelNumber>
<modelURL>http://www.philips-hue.com</modelURL>
<serialNumber>%s</serialNumber>
<UDN>uuid:2f402f80-da50-11e1-9b23-%s</UDN>
<presentationURL>index.html</presentationURL>
</device>
</root>
`

// serialNumber is the lower-cased MAC without separators, matching the real
// bridge's serial — the first six and last six hex digits of the bridge id
// with the synthetic "fffe" padding removed.
func serialNumber(bridgeID string) string {
	id := strings.ToLower(bridgeID)
	if len(id) != 16 {
		return id
	}
	return id[:6] + id[10:]
}

func handleDescriptionXML(bridgeID, ip string, port int) http.HandlerFunc {
	serial := serialNumber(bridgeID)
	body := fmt.Sprintf(descriptionXMLTemplate, ip, port, ip, serial, serial)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
		w.Write([]byte(body))
	}
}
