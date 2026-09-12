package setup

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// AdminUsername is the fixed HTTP Basic Auth username for the standalone
// admin/setup UI — only the password is user-configured.
const AdminUsername = "admin"

// HashPassword bcrypt-hashes plaintext for storage in Config.AdminPasswordHash.
func HashPassword(plaintext string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
}

// RequireAdmin wraps next in HTTP Basic Auth. passwordHash is called on
// every request rather than captured once, so a config reload (e.g. after
// the wizard runs) is picked up without restarting the server.
func RequireAdmin(next http.Handler, passwordHash func() []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != AdminUsername || bcrypt.CompareHashAndPassword(passwordHash(), []byte(pass)) != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="huebridge admin"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
