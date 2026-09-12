package logging

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"", LevelInfo},
		{"nonsense", LevelInfo},
	}
	for _, tt := range tests {
		if got := ParseLevel(tt.in); got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestMiddleware_InfoLogsMethodPathAndStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	handler := Middleware(logger, LevelInfo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest("PUT", "/lights/1/state", strings.NewReader(`{"on":true}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	out := buf.String()
	if !strings.Contains(out, "PUT") || !strings.Contains(out, "/lights/1/state") || !strings.Contains(out, "418") {
		t.Fatalf("expected method, path and status in log output, got %q", out)
	}
	if strings.Contains(out, "on\":true") {
		t.Fatalf("info level should not log the request body, got %q", out)
	}
}

func TestMiddleware_DebugLogsRequestAndResponseBodies(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	handler := Middleware(logger, LevelDebug)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readAll(r)
		if string(body) != `{"on":true}` {
			t.Fatalf("handler did not receive the original request body, got %q", body)
		}
		w.Write([]byte(`{"success":true}`))
	}))

	req := httptest.NewRequest("PUT", "/lights/1/state", strings.NewReader(`{"on":true}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	out := buf.String()
	if !strings.Contains(out, `{"on":true}`) {
		t.Fatalf("expected the request body in debug log output, got %q", out)
	}
	if !strings.Contains(out, `{"success":true}`) {
		t.Fatalf("expected the response body in debug log output, got %q", out)
	}
}

func readAll(r *http.Request) ([]byte, error) {
	buf := make([]byte, r.ContentLength)
	_, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf, nil
}
