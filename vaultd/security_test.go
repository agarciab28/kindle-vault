package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuard(t *testing.T) {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("/api/home", ok)
	mux.HandleFunc("/api/login", ok)
	mux.HandleFunc("/api/setup", ok)
	h := guard(mux, "tok", "127.0.0.1:8317")

	for _, c := range []struct {
		name, method, target, host, origin string
		want                               int
	}{
		{"app request", "GET", "/api/home?k=tok", "127.0.0.1:8317", "", 200},
		{"app request, file origin", "GET", "/api/home?k=tok", "127.0.0.1:8317", "file://", 200},
		{"app request, null origin", "GET", "/api/home?k=tok", "127.0.0.1:8317", "null", 200},
		{"login is a POST", "POST", "/api/login?k=tok", "127.0.0.1:8317", "null", 200},
		{"choosing a vault is a POST", "POST", "/api/setup?k=tok", "127.0.0.1:8317", "null", 200},
		{"choosing a vault by GET", "GET", "/api/setup?k=tok", "127.0.0.1:8317", "", 405},
		{"no token", "GET", "/api/home", "127.0.0.1:8317", "", 401},
		{"wrong token", "GET", "/api/home?k=nope", "127.0.0.1:8317", "", 401},
		{"a web page", "GET", "/api/home?k=tok", "127.0.0.1:8317", "https://evil.example", 403},
		{"DNS rebinding", "GET", "/api/home?k=tok", "evil.example:8317", "", 403},
		{"login by GET", "GET", "/api/login?k=tok", "127.0.0.1:8317", "", 405},
		{"POST to a read endpoint", "POST", "/api/home?k=tok", "127.0.0.1:8317", "", 405},
		{"other methods", "DELETE", "/api/home?k=tok", "127.0.0.1:8317", "", 405},
	} {
		r := httptest.NewRequest(c.method, c.target, strings.NewReader(""))
		r.Host = c.host
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.want)
		}
	}
}
