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

func TestLoadOrGenerateCertificate_UsesFreshBridgeIDOnlyWhenGenerating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cert.pem")

	first, err := LoadOrGenerateCertificate(path, "AABBCCFFFEDDEEFF")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() error: %v", err)
	}
	firstLeaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatalf("parse first certificate: %v", err)
	}

	// A different bridgeID is ignored once a certificate is already
	// persisted at path — the point of persistence is stability.
	second, err := LoadOrGenerateCertificate(path, "1122334455667788")
	if err != nil {
		t.Fatalf("LoadOrGenerateCertificate() on existing file error: %v", err)
	}
	secondLeaf, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatalf("parse second certificate: %v", err)
	}

	if secondLeaf.Subject.CommonName != firstLeaf.Subject.CommonName {
		t.Fatalf("got CommonName=%q after reload, want the persisted %q", secondLeaf.Subject.CommonName, firstLeaf.Subject.CommonName)
	}
}
