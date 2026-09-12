package discovery

import (
	"slices"
	"testing"
)

func TestMDNSInstanceName(t *testing.T) {
	got := mdnsInstanceName("AABBCCFFFEDDEEFF")
	want := "Philips Hue - AABBCCFFFEDDEEFF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A real bridge's mDNS TXT record carries both bridgeid and modelid=BSB002;
// the Hue app's local search appears to filter on modelid being present, so
// omitting it silently drops huebridge from "Add bridge" even though its
// SSDP and description.xml responses are otherwise correct.
func TestMDNSTXTRecordIncludesModelID(t *testing.T) {
	got := mdnsTXTRecords("AABBCCFFFEDDEEFF")
	want := []string{"bridgeid=AABBCCFFFEDDEEFF", "modelid=BSB002"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
