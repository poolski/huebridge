package discovery

import "testing"

func TestMDNSInstanceName(t *testing.T) {
	got := mdnsInstanceName("AABBCCFFFEDDEEFF")
	want := "Philips Hue - AABBCCFFFEDDEEFF"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
