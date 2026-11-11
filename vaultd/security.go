package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// The API is reachable from anything running on the Kindle, including web pages
// opened in the Kindle browser. Three checks keep the vault private:
//
//  1. Token: a random value generated at every start and written to token.js
//     inside the WAF folder. Only the Vault app loads that file; a web page
//     cannot read local files, so it cannot learn the token.
//  2. Origin: browsers always send Origin on cross-origin requests. Only the
//     WAF (file://, reported as "null") or no Origin at all is accepted.
//  3. Host: must be the loopback address we listen on, which defeats DNS
//     rebinding (a public name that resolves to 127.0.0.1).

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("token: %v", err)
	}
	return hex.EncodeToString(b)
}

// writeTokenJS publishes the token for the WAF. The file is replaced atomically.
func writeTokenJS(path, token string) error {
	if path == "" {
		return nil
	}
	tmp := filepath.Join(filepath.Dir(path), ".token.js.tmp")
	js := fmt.Sprintf("window.VAULT_TOKEN = %q;\n", token)
	if err := os.WriteFile(tmp, []byte(js), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// postOnly lists the only endpoints that change state. The password travels in
// the POST body, never in the URL (URLs can end up in logs).
var postOnly = map[string]bool{"/api/login": true, "/api/logout": true, "/api/setup": true}

func allowedOrigin(origin string) bool {
	return origin == "" || origin == "null" || origin == "file://"
}

func allowedHost(host, listenAddr string) bool {
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	_, lport, _ := net.SplitHostPort(listenAddr)
	return port == lport && (h == "127.0.0.1" || h == "localhost")
}

func guard(next http.Handler, token, listenAddr string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !allowedOrigin(origin) || !allowedHost(r.Host, listenAddr) {
			log.Printf("rejected request: origin=%q host=%q path=%s", origin, r.Host, r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Read-only API, except the account actions, which are POST only.
		if postOnly[r.URL.Path] != (r.Method == http.MethodPost) || (r.Method != http.MethodGet && r.Method != http.MethodPost) {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		got := r.URL.Query().Get("k")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			log.Printf("rejected request without a valid token: path=%s", r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
