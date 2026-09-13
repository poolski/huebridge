// Package bridgetls generates the self-signed TLS certificate and
// MAC-derived bridge id the official Hue app expects a genuine bridge to
// present, following the same approach diyHue and Bifrost use.
package bridgetls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

// BridgeID derives the 16-character bridge id from a MAC address the same
// way the real Hue bridge does: OUI + fixed EUI-64 padding + NIC bytes,
// uppercased. See docs/superpowers/specs/hue-clip-v1-api-reference.md,
// "Config" section.
func BridgeID(mac net.HardwareAddr) string {
	raw := hex.EncodeToString(mac)
	return strings.ToUpper(raw[0:6] + "fffe" + raw[6:12])
}

// GenerateCertificate builds a self-signed EC certificate for bridgeID,
// valid for 10 years, suitable for tls.Config.Certificates.
func GenerateCertificate(bridgeID string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	// The official Hue app verifies the certificate's serial number against
	// the bridge id it discovered via mDNS/SSDP — diyHue's own setup docs
	// call this out explicitly ("the official Hue app verifies the cert
	// against the interface MAC"). A random serial passes ordinary X.509
	// validation (TLS handshakes and third-party apps don't care), but the
	// official app silently distrusts the bridge and never starts polling
	// for the link-button press.
	serial, ok := new(big.Int).SetString(bridgeID, 16)
	if !ok {
		return tls.Certificate{}, fmt.Errorf("parse bridgeID %q as hex for the certificate serial", bridgeID)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   bridgeID,
			Organization: []string{"Philips Hue"},
			Country:      []string{"NL"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:        false,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}

// LoadOrGenerateCertificate returns the certificate persisted at path, or
// generates one for bridgeID and persists it there if none exists yet.
// Real bridges (and diyHue/Bifrost) keep a stable certificate across
// restarts; generating fresh key material every time the process starts
// makes the bridge look like a different device to any client that
// remembers what it saw on a previous connection.
func LoadOrGenerateCertificate(path, bridgeID string) (tls.Certificate, error) {
	if cert, err := loadCertificate(path); err == nil {
		switch existingID, ok := certCommonName(cert); {
		case !ok:
			// Loaded fine but couldn't be parsed to check its CN — treat it
			// like "no cert yet" and regenerate silently below.
		case existingID == bridgeID:
			return cert, nil
		default:
			log.Printf("regenerating TLS certificate at %s: its CN %q no longer matches the current bridge id %q (the MAC address it was derived from must have changed) — the official Hue app verifies the certificate against the bridge id it discovers via mDNS/SSDP, so a stale cert would silently fail pairing", path, existingID, bridgeID)
		}
	}

	cert, err := GenerateCertificate(bridgeID)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := saveCertificate(path, cert); err != nil {
		return tls.Certificate{}, fmt.Errorf("persist certificate to %s: %w", path, err)
	}
	return cert, nil
}

// certCommonName returns the Subject CommonName of cert's leaf certificate —
// the bridge id it was generated for, per GenerateCertificate — or false if
// it can't be parsed.
func certCommonName(cert tls.Certificate) (string, bool) {
	if len(cert.Certificate) == 0 {
		return "", false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return "", false
	}
	return leaf.Subject.CommonName, true
}

// loadCertificate reads a certificate+key pair written by saveCertificate.
func loadCertificate(path string) (tls.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, err
	}

	var certPEM, keyPEM []byte
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
		case "PRIVATE KEY":
			keyPEM = pem.EncodeToMemory(block)
		}
	}
	if certPEM == nil || keyPEM == nil {
		return tls.Certificate{}, fmt.Errorf("%s does not contain both a certificate and a private key", path)
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// saveCertificate writes cert as a PEM-encoded certificate followed by its
// PKCS#8 private key, readable only by the owner since the key is
// sensitive.
func saveCertificate(path string, cert tls.Certificate) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	var buf bytes.Buffer
	for _, der := range cert.Certificate {
		if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
			return err
		}
	}
	if err := pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}
