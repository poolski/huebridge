package main

import "testing"

func TestIsStandalone(t *testing.T) {
	tests := []struct {
		name          string
		supervisorTok string
		want          bool
	}{
		{"supervisor token set", "some-token", false},
		{"supervisor token unset", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := func(key string) string {
				if key == "SUPERVISOR_TOKEN" {
					return tt.supervisorTok
				}
				return ""
			}
			if got := isStandalone(env); got != tt.want {
				t.Errorf("isStandalone() = %v, want %v", got, tt.want)
			}
		})
	}
}
