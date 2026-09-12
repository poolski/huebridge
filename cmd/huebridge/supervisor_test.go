package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchIngressPort(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantPort int
		wantErr  bool
	}{
		{
			name: "reports the assigned port",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer token123" {
					t.Errorf("Authorization header = %q, want Bearer token123", got)
				}
				if r.URL.Path != "/addons/self/info" {
					t.Errorf("path = %q, want /addons/self/info", r.URL.Path)
				}
				fmt.Fprint(w, `{"data":{"ingress_port":54321}}`)
			},
			wantPort: 54321,
		},
		{
			name: "errors on a non-200 response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			wantErr: true,
		},
		{
			name: "errors when supervisor hasn't assigned a port yet",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"data":{"ingress_port":0}}`)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			port, err := fetchIngressPort(ctx, srv.Client(), srv.URL, "token123")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got nil error, want one")
				}
				return
			}
			if err != nil {
				t.Fatalf("fetchIngressPort: %v", err)
			}
			if port != tt.wantPort {
				t.Fatalf("got port %d, want %d", port, tt.wantPort)
			}
		})
	}
}
