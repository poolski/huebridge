package bridgetls

import (
	"net"
	"testing"
)

func TestBridgeID(t *testing.T) {
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	got := BridgeID(mac)
	want := "AABBCCFFFEDDEEFF"
	if got != want {
		t.Fatalf("BridgeID() = %q, want %q", got, want)
	}
}

func TestGenerateCertificate(t *testing.T) {
	cert, err := GenerateCertificate("AABBCCFFFEDDEEFF")
	if err != nil {
		t.Fatalf("GenerateCertificate() error: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("expected at least one DER certificate in the chain")
	}
	if cert.PrivateKey == nil {
		t.Fatal("expected a non-nil private key")
	}
}
