// Package logging provides the request logging middleware shared by the
// bridge API and ingress servers, gated by the add-on's log_level option.
package logging

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Level controls how much a request is logged.
type Level int

const (
	// LevelInfo logs one line per request: method, path, status, duration.
	LevelInfo Level = iota
	// LevelDebug additionally logs request headers and the request/response
	// bodies, for diagnosing what a client actually sent and got back.
	LevelDebug
)

// ParseLevel maps the log_level add-on option to a Level, defaulting to
// LevelInfo for anything other than an exact (case-insensitive) "debug".
func ParseLevel(s string) Level {
	if strings.EqualFold(s, "debug") {
		return LevelDebug
	}
	return LevelInfo
}

// statusRecorder mirrors what's written through it into an optional buffer
// so the middleware can log the response body without altering behaviour
// for the real client.
type statusRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.body != nil {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

// redactedHeaders lists header names (matched case-insensitively via
// http.Header's own accessors) whose values are trivially-reversible or
// plaintext credentials, not safe to write to logs even at debug level:
// standalone's Basic Auth credentials, and the add-on ingress path's HA
// session cookie, if either is ever present.
var redactedHeaders = []string{"Authorization", "Cookie"}

// redactHeaders returns a shallow copy of h with the values of
// redactedHeaders replaced by a placeholder, for safe inclusion in debug
// logs.
func redactHeaders(h http.Header) http.Header {
	clone := h.Clone()
	for _, name := range redactedHeaders {
		if clone.Get(name) != "" {
			clone.Set(name, "[redacted]")
		}
	}
	return clone
}

// Middleware logs every request at the given level. At LevelDebug it reads
// the request body up front and replaces r.Body with a fresh reader over
// the same bytes, so downstream handlers see it unchanged.
func Middleware(logger *log.Logger, level Level) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			var reqBody []byte
			if level == LevelDebug && r.Body != nil {
				reqBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			if level == LevelDebug {
				rec.body = &bytes.Buffer{}
			}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)
			if level == LevelDebug {
				logger.Printf("DEBUG %s %s status=%d duration=%s headers=%v request_body=%s response_body=%s",
					r.Method, r.URL.Path, rec.status, duration, redactHeaders(r.Header), reqBody, rec.body.Bytes())
			} else {
				logger.Printf("%s %s status=%d duration=%s", r.Method, r.URL.Path, rec.status, duration)
			}
		})
	}
}
