package bridgetls

import (
	"crypto/x509"
	"net"
	"path/filepath"
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

func TestLoadOrGenerateCertificate_GeneratesAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cert.pem")

	first, err := LoadOrGenerateCertificate(path, "AABBCCFFFEDDEEFF")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() error: %v", err)
	}

	second, err := LoadOrGenerateCertificate(path, "AABBCCFFFEDDEEFF")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() on existing file error: %v", err)
	}

	firstLeaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatalf("parse first certificate: %v", err)
	}
	secondLeaf, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatalf("parse second certificate: %v", err)
	}

	if firstLeaf.SerialNumber.Cmp(secondLeaf.SerialNumber) != 0 {
		t.Fatalf("got different serial numbers across calls, want the persisted certificate reused: %v vs %v", firstLeaf.SerialNumber, secondLeaf.SerialNumber)
	}
	if firstLeaf.Subject.CommonName != secondLeaf.Subject.CommonName {
		t.Fatalf("got different CommonName across calls: %q vs %q", firstLeaf.Subject.CommonName, secondLeaf.Subject.CommonName)
	}
}

func TestLoadOrGenerateCertificate_RegeneratesWhenBridgeIDChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cert.pem")

	first, err := LoadOrGenerateCertificate(path, "AABBCCFFFEDDEEFF")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() error: %v", err)
	}
	firstLeaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatalf("parse first certificate: %v", err)
	}

	// A stale cert (e.g. generated for a MAC address the process no longer
	// sees, such as after a container's interface gets reassigned) must not
	// keep being served under a bridgeID it doesn't match — the official
	// Hue app verifies the cert's CN against the bridgeID it discovers via
	// mDNS/SSDP and silently distrusts a mismatched one.
	second, err := LoadOrGenerateCertificate(path, "1122334455667788")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() on existing file error: %v", err)
	}
	secondLeaf, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatalf("parse second certificate: %v", err)
	}

	if secondLeaf.Subject.CommonName != "1122334455667788" {
		t.Fatalf("got CommonName=%q after a bridgeID change, want the new bridgeID 1122334455667788", secondLeaf.Subject.CommonName)
	}
	if secondLeaf.SerialNumber.Cmp(firstLeaf.SerialNumber) == 0 {
		t.Fatal("expected a freshly generated certificate (different serial) after a bridgeID mismatch")
	}

	// The regenerated certificate must also be persisted, so a third call
	// with the same (new) bridgeID reuses it rather than regenerating again.
	third, err := LoadOrGenerateCertificate(path, "1122334455667788")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() on regenerated file error: %v", err)
	}
	thirdLeaf, err := x509.ParseCertificate(third.Certificate[0])
	if err != nil {
		t.Fatalf("parse third certificate: %v", err)
	}
	if thirdLeaf.SerialNumber.Cmp(secondLeaf.SerialNumber) != 0 {
		t.Fatal("expected the regenerated certificate to be persisted and reused on the next call")
	}
}
