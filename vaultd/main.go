// vaultd serves a synced Obsidian vault to the Kindle "Vault" app (a WAF).
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
	state := flag.String("state", "/mnt/us/vault-data/state.json", "sync state file")
	accountCmd := flag.String("client", "/bin/sh /mnt/us/documents/vault-app/ob", "client wrapper (login, logout, whoami, vaults, current, setup, sync)")
	wifiCmd := flag.String("wifi", "lipc-get-prop com.lab126.wifid cmState", "command that prints CONNECTED when online (empty: always online)")
	homeCmd := flag.String("home", "lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home", "command that returns to the Kindle home")
	tz := flag.String("tz", "", "time zone for dates, e.g. Europe/Madrid (default: the device's)")
	every := flag.Duration("auto", 30*time.Minute, "auto-sync when the last attempt is older than this (0 disables)")
	tokenJS := flag.String("tokenjs", "/var/local/mesquite/vault/token.js", "where to publish the API token for the WAF")
	flag.Parse()

	token := newToken()
	if err := writeTokenJS(*tokenJS, token); err != nil {
		log.Fatalf("cannot publish the API token: %v", err)
	}

	if *tz != "" {
		if loc, err := time.LoadLocation(*tz); err == nil {
			time.Local = loc
		} else {
			log.Printf("unknown time zone %q: using the device's", *tz)
		}
	}

	v := NewVault(*vaultDir)
	if err := v.Reindex(); err != nil {
		log.Printf("index: %v", err)
	}
	log.Printf("indexed %d notes from %s", v.Count(), *vaultDir)

	client := strings.Fields(*accountCmd)
	s := NewSyncer(*state, append(append([]string{}, client...), "sync"), strings.Fields(*wifiCmd), func() {
		if err := v.Reindex(); err != nil {
			log.Printf("reindex: %v", err)
		}
		log.Printf("sync finished; %d notes", v.Count())
	})

	acct := &Account{command: client, syncer: s, onNotesChanged: func() {
		if err := v.Reindex(); err != nil {
			log.Printf("reindex: %v", err)
		}
	}}
	go acct.RefreshVault()

	if *every > 0 {
		go func() {
			time.Sleep(20 * time.Second)
			for {
				if acct.Vault().ID != "" {
					s.MaybeStart(*every)
				}
				time.Sleep(5 * time.Minute)
			}
		}()
	}

	mux := http.NewServeMux()
	// The interface language comes with every request (the app remembers it).
	lang := func(r *http.Request) Lang { return parseLang(r.URL.Query().Get("l")) }
	page := func(w http.ResponseWriter, r *http.Request, p Page, ok bool) {
		if !ok {
			l := lang(r)
			p = Page{Title: l.T("notFound"), HTML: `<p>` + esc(l.T("notFoundBody")) + `</p><p><a href="#/">` + esc(l.T("backHome")) + `</a></p>`}
		}
		writeJSON(w, p)
	}
	mux.HandleFunc("/api/home", func(w http.ResponseWriter, r *http.Request) {
		vault := acct.Vault()
		if vault.ID != "" {
			s.MaybeStart(10 * time.Minute) // opening the app refreshes if it has been a while
		}
		page(w, r, HomePage(lang(r), v, s.State(), vault), true)
	})
	mux.HandleFunc("/api/folder", func(w http.ResponseWriter, r *http.Request) {
		p, ok := FolderPage(lang(r), v, r.URL.Query().Get("p"))
		page(w, r, p, ok)
	})
	mux.HandleFunc("/api/note", func(w http.ResponseWriter, r *http.Request) {
		p, ok := NotePage(lang(r), v, r.URL.Query().Get("p"))
		page(w, r, p, ok)
	})
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) { page(w, r, TagsPage(lang(r), v), true) })
	mux.HandleFunc("/api/tag", func(w http.ResponseWriter, r *http.Request) {
		page(w, r, TagPage(lang(r), v, r.URL.Query().Get("t")), true)
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		page(w, r, SearchPage(lang(r), v, r.URL.Query().Get("q")), true)
	})
	mux.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
		if !s.Online() {
			writeJSON(w, map[string]any{"started": false, "error": lang(r).T("noWifi"), "state": s.State()})
			return
		}
		if acct.Vault().ID == "" && acct.RefreshVault().ID == "" {
			writeJSON(w, map[string]any{"started": false, "error": lang(r).T("noVault"), "needVault": true, "state": s.State()})
			return
		}
		started := s.Start()
		writeJSON(w, map[string]any{"started": started, "state": s.State()})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"state": s.State(), "notes": v.Count(), "online": s.Online()})
	})
	mux.HandleFunc("/api/account", func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		if r.URL.Query().Get("form") == "1" {
			page(w, r, AccountPage(l, "", false, "", true, acct.Vault()), true)
			return
		}
		if !s.Online() {
			page(w, r, AccountPage(l, "", false, l.T("noWifiSession"), false, acct.Vault()), true)
			return
		}
		email, err := acct.Status()
		msg := ""
		if err != nil {
			msg = acct.errText(l, err)
		}
		page(w, r, AccountPage(l, email, true, msg, false, acct.RefreshVault()), true)
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		page(w, r, SettingsPage(lang(r), acct.Vault()), true)
	})
	mux.HandleFunc("/api/vaults", func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		if !s.Online() {
			page(w, r, VaultPage(l, acct.Vault(), nil, l.T("offline")), true)
			return
		}
		vaults, err := acct.Vaults()
		msg := ""
		if err != nil {
			msg = l.T("vaultsFailed", acct.errText(l, err))
		}
		page(w, r, VaultPage(l, acct.RefreshVault(), vaults, msg), true)
	})
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		if err := r.ParseForm(); err != nil {
			writeJSON(w, AccountResult{Message: l.T("invalidData")})
			return
		}
		res := acct.Login(l, r.PostForm.Get("email"), r.PostForm.Get("password"), r.PostForm.Get("mfa"),
			r.PostForm.Get("accept") == "1")
		log.Printf("account: login ok=%v", res.OK) // never the email or password
		writeJSON(w, res)
	})
	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		res := acct.Logout(lang(r))
		log.Printf("account: logout ok=%v", res.OK)
		writeJSON(w, res)
	})
	mux.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		if err := r.ParseForm(); err != nil {
			writeJSON(w, AccountResult{Message: l.T("invalidData")})
			return
		}
		res := acct.Setup(l, r.PostForm.Get("vault"), r.PostForm.Get("password"))
		log.Printf("account: vault setup ok=%v", res.OK) // never the vault or its password
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
