package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDescriptionXML(t *testing.T) {
	rec := httptest.NewRecorder()
	handleDescriptionXML("AABBCCFFFEDDEEFF", "192.168.1.20", 443)(rec, httptest.NewRequest("GET", "/description.xml", nil))

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/xml") {
		t.Fatalf("got Content-Type=%q, want application/xml", got)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<URLBase>https://192.168.1.20:443/</URLBase>",
		"<deviceType>urn:schemas-upnp-org:device:Basic:1</deviceType>",
		"<modelNumber>BSB002</modelNumber>",
		"<serialNumber>aabbccddeeff</serialNumber>",
		"<UDN>uuid:2f402f80-da50-11e1-9b23-aabbccddeeff</UDN>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("description.xml is missing %q\n%s", want, body)
		}
	}
}

func TestSerialNumber(t *testing.T) {
	if got := serialNumber("AABBCCFFFEDDEEFF"); got != "aabbccddeeff" {
		t.Fatalf("got %q, want aabbccddeeff", got)
	}
	if got := serialNumber("short"); got != "short" {
		t.Fatalf("got %q, want the input returned unchanged", got)
	}
}
