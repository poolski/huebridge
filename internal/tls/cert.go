// Package bridgetls generates the self-signed TLS certificate and
// MAC-derived bridge id the official Hue app expects a genuine bridge to
// present, following the same approach diyHue and Bifrost use.
package bridgetls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
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

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: bridgeID},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
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
