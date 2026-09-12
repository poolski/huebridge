package discovery

import (
	"testing"
)

func TestSSDPLocationURL(t *testing.T) {
	got := ssdpLocationURL("192.168.1.50", 443)
	want := "https://192.168.1.50:443/description.xml"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSSDPUSN(t *testing.T) {
	got := ssdpUSN("AABBCCFFFEDDEEFF")
	want := "uuid:2f402f80-da50-11e1-9b23-aabbccddeeff::upnp:rootdevice"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
