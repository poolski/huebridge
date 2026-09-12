package discovery

import (
	"net"
	"testing"

	"github.com/hashicorp/mdns"
)

func TestHomeAssistantURLsUsesIPv4WhenPresent(t *testing.T) {
	entries := []*mdns.ServiceEntry{
		{AddrV4: net.ParseIP("192.168.1.10"), Port: 8123},
	}
	got := homeAssistantURLs(entries)
	want := []string{"http://192.168.1.10:8123"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHomeAssistantURLsFallsBackToIPv6(t *testing.T) {
	entries := []*mdns.ServiceEntry{
		{AddrV6: net.ParseIP("fe80::1"), Port: 8123},
	}
	got := homeAssistantURLs(entries)
	want := []string{"http://fe80::1:8123"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHomeAssistantURLsSkipsEntriesWithNoAddressOrPort(t *testing.T) {
	entries := []*mdns.ServiceEntry{
		{Port: 8123},                          // no address
		{AddrV4: net.ParseIP("192.168.1.11")}, // no port
	}
	got := homeAssistantURLs(entries)
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}
