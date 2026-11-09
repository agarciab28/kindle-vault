// vaultd serves a synced Obsidian vault to the Kindle "Bóveda" app (a WAF).
//
// It listens on localhost only, renders Markdown notes to HTML fragments,
// and runs the read-only Obsidian Sync client on demand and periodically.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
	_ "time/tzdata"
)

func main() {
	vaultDir := flag.String("vault", "/mnt/us/vault", "synced vault folder")
	addr := flag.String("addr", "127.0.0.1:8317", "listen address (keep it on localhost)")
	state := flag.String("state", "/mnt/us/vault-app/state.json", "sync state file")
	syncCmd := flag.String("sync", "/bin/sh /mnt/us/ob-kindle.sh sync", "sync command")
	accountCmd := flag.String("account", "/bin/sh /mnt/us/ob-kindle.sh", "client wrapper for login / logout / whoami")
	wifiCmd := flag.String("wifi", "lipc-get-prop com.lab126.wifid cmState", "command that prints CONNECTED when online (empty: always online)")
	homeCmd := flag.String("home", "lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home", "command that returns to the Kindle home")
	tz := flag.String("tz", "America/Monterrey", "time zone for dates")
	every := flag.Duration("auto", 30*time.Minute, "auto-sync when the last attempt is older than this (0 disables)")
	tokenJS := flag.String("tokenjs", "/var/local/mesquite/vault/token.js", "where to publish the API token for the WAF")
	flag.Parse()

	token := newToken()
	if err := writeTokenJS(*tokenJS, token); err != nil {
		log.Fatalf("cannot publish the API token: %v", err)
	}

	if loc, err := time.LoadLocation(*tz); err == nil {
		time.Local = loc
	}

	v := NewVault(*vaultDir)
	if err := v.Reindex(); err != nil {
		log.Printf("index: %v", err)
	}
	log.Printf("indexed %d notes from %s", v.Count(), *vaultDir)

	s := NewSyncer(*state, strings.Fields(*syncCmd), strings.Fields(*wifiCmd), func() {
		if err := v.Reindex(); err != nil {
			log.Printf("reindex: %v", err)
		}
		log.Printf("sync finished; %d notes", v.Count())
	})

	if *every > 0 {
		go func() {
			time.Sleep(20 * time.Second)
			for {
				s.MaybeStart(*every)
				time.Sleep(5 * time.Minute)
			}
		}()
	}

	mux := http.NewServeMux()
	page := func(w http.ResponseWriter, p Page, ok bool) {
		if !ok {
			p = Page{Title: "No encontrado", HTML: `<p>No existe esa nota o carpeta.</p><p><a href="#/">Volver al inicio</a></p>`}
		}
		writeJSON(w, p)
	}
	mux.HandleFunc("/api/home", func(w http.ResponseWriter, r *http.Request) {
		s.MaybeStart(10 * time.Minute) // opening the app refreshes if it has been a while
		page(w, HomePage(v, s.State()), true)
	})
	mux.HandleFunc("/api/folder", func(w http.ResponseWriter, r *http.Request) {
		p, ok := FolderPage(v, r.URL.Query().Get("p"))
		page(w, p, ok)
	})
	mux.HandleFunc("/api/note", func(w http.ResponseWriter, r *http.Request) {
		p, ok := NotePage(v, r.URL.Query().Get("p"))
		page(w, p, ok)
	})
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) { page(w, TagsPage(v), true) })
	mux.HandleFunc("/api/tag", func(w http.ResponseWriter, r *http.Request) {
		page(w, TagPage(v, r.URL.Query().Get("t")), true)
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		page(w, SearchPage(v, r.URL.Query().Get("q")), true)
	})
	mux.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
		if !s.Online() {
			writeJSON(w, map[string]any{"started": false, "error": "Sin Wi-Fi", "state": s.State()})
			return
		}
		started := s.Start()
		writeJSON(w, map[string]any{"started": started, "state": s.State()})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"state": s.State(), "notes": v.Count(), "online": s.Online()})
	})
	acct := &Account{command: strings.Fields(*accountCmd), syncer: s}
	mux.HandleFunc("/api/account", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("form") == "1" {
			page(w, AccountPage("", false, "", true), true)
			return
		}
		if !s.Online() {
			page(w, AccountPage("", false, "Sin Wi-Fi: no se puede revisar la sesión ahora.", false), true)
			return
		}
		email, err := acct.Status()
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		page(w, AccountPage(email, true, msg, false), true)
	})
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeJSON(w, AccountResult{Message: "Datos inválidos."})
			return
		}
		res := acct.Login(r.PostForm.Get("email"), r.PostForm.Get("password"), r.PostForm.Get("mfa"))
		log.Printf("account: login ok=%v", res.OK) // never the email or password
		writeJSON(w, res)
	})
	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		res := acct.Logout()
		log.Printf("account: logout ok=%v", res.OK)
		writeJSON(w, res)
	})
	mux.HandleFunc("/api/exit", func(w http.ResponseWriter, r *http.Request) {
		if f := strings.Fields(*homeCmd); len(f) > 0 {
			_ = exec.Command(f[0], f[1:]...).Start()
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/log", func(w http.ResponseWriter, r *http.Request) {
		m := r.URL.Query().Get("m")
		if len(m) > 500 {
			m = m[:500]
		}
		log.Printf("app: %s", m)
		writeJSON(w, map[string]any{"ok": true})
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           guard(mux, token, *addr),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Printf("listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
