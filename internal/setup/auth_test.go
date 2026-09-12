package setup

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func mustHash(t *testing.T, plaintext string) []byte {
	t.Helper()
	hash, err := HashPassword(plaintext)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return hash
}

func TestRequireAdmin(t *testing.T) {
	hash := mustHash(t, "correct horse battery staple")
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireAdmin(next, func() []byte { return hash })

	tests := []struct {
		name       string
		setAuth    bool
		user, pass string
		wantStatus int
		wantCalled bool
	}{
		{"no credentials", false, "", "", http.StatusUnauthorized, false},
		{"wrong username", true, "root", "correct horse battery staple", http.StatusUnauthorized, false},
		{"wrong password", true, AdminUsername, "wrong", http.StatusUnauthorized, false},
		{"correct credentials", true, AdminUsername, "correct horse battery staple", http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("next called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

func TestHashPasswordProducesVerifiableHash(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("hunter2")); err != nil {
		t.Errorf("hash does not verify against original password: %v", err)
	}
}
