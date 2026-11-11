package main

import (
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"
)

// Page is what the WAF shell receives for every route.
type Page struct {
	Title string `json:"title"`
	HTML  string `json:"html"`
}

var esc = html.EscapeString

// vaultInfo is the vault attached to this Kindle, as far as vaultd knows.
type vaultInfo struct {
	Known bool   // false until the client has been asked once
	ID    string // "" when no vault has been chosen
	Name  string
}

// RemoteVault is a vault of the signed-in account.
type RemoteVault struct{ ID, Name string }

func btnLink(href, label string) string {
	return fmt.Sprintf(`<a class="btn" href="%s">%s</a>`, href, esc(label))
}

// langBox lets the reader switch the interface language. Notes are not affected.
func langBox(l Lang) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<h2>%s</h2><p class="muted">%s</p><p>`, esc(l.T("language")), esc(l.T("languageNote")))
	for _, code := range []Lang{EN, ES} {
		class := "btn"
		if code == l {
			class = "btn on"
		}
		fmt.Fprintf(&b, `<a class="%s" href="javascript:app.lang('%s')">%s</a> `, class, code, esc(l.T("lang."+string(code))))
	}
	b.WriteString(`</p>`)
	return b.String()
}

func syncBox(l Lang, st SyncState, vault vaultInfo) string {
	var b strings.Builder
	b.WriteString(`<div class="syncbox" id="syncbox">`)
	if vault.Known && vault.ID == "" {
		// First run: nothing to sync until a vault is chosen.
		fmt.Fprintf(&b, `<p><strong>%s</strong></p><p class="muted">%s</p>`, esc(l.T("noVault")), esc(l.T("noVaultHint")))
		fmt.Fprintf(&b, `<p>%s %s</p></div>`, btnLink("#/vault", l.T("chooseVault")), btnLink("#/account", l.T("account")))
		return b.String()
	}
	switch {
	case st.Running:
		fmt.Fprintf(&b, `<p><strong>%s</strong></p>`, esc(l.T("syncing")))
	case !st.LastOK && !st.LastAttempt.IsZero():
		fmt.Fprintf(&b, `<p><strong>%s</strong> (%s)</p><pre class="err">%s</pre>`, esc(l.T("syncFailed")), esc(l.Ago(st.LastAttempt)), esc(st.LastError))
		fmt.Fprintf(&b, `<p class="muted">%s</p>`, esc(l.T("lastGoodSync", l.Date(st.LastSync))))
	default:
		fmt.Fprintf(&b, `<p>%s <strong>%s</strong> · %s</p>`, esc(l.T("synced")), esc(l.Ago(st.LastSync)), esc(l.Date(st.LastSync)))
		if st.LastChanged > 0 {
			fmt.Fprintf(&b, `<p class="muted">%s</p>`, esc(l.N("filesUpdated", st.LastChanged)))
		}
	}
	if !st.Running && !st.LastOK && authError(st.LastError) {
		fmt.Fprintf(&b, `<p><strong>%s</strong></p><p>%s %s</p></div>`, esc(l.T("sessionExpired")),
			btnLink("#/account/login", l.T("signIn")), btnLink("#/sync", l.T("retry")))
		return b.String()
	}
	fmt.Fprintf(&b, `<p>%s %s</p></div>`, btnLink("#/sync", l.T("syncNow")), btnLink("#/account", l.T("account")))
	return b.String()
}

func loginForm(l Lang) string {
	return `<form class="login" onsubmit="return app.login(this)" autocomplete="off">` +
		`<label for="lf-email">` + esc(l.T("email")) + `</label>` +
		`<input id="lf-email" name="email" type="text" autocapitalize="off" autocorrect="off" spellcheck="false">` +
		`<label for="lf-pass">` + esc(l.T("password")) + `</label>` +
		`<input id="lf-pass" name="password" type="password">` +
		`<label for="lf-mfa">` + esc(l.T("mfa")) + ` <span class="muted">` + esc(l.T("mfaHint")) + `</span></label>` +
		`<input id="lf-mfa" name="mfa" type="text" maxlength="10">` +
		`<div class="alert notice"><p><strong>` + esc(l.T("notice")) + `</strong> ` + esc(l.T("noticeBody")) + `</p>` +
		`<input name="accept" type="hidden" value="">` +
		`<p><a class="btn" id="lf-accept" href="javascript:app.accept()" data-off="` + esc(l.T("acceptTap")) + `" data-on="` + esc(l.T("accepted")) + `">` +
		esc(l.T("acceptTap")) + `</a></p></div>` +
		`<p><button type="submit">` + esc(l.T("signIn")) + `</button></p></form>` +
		`<div id="loginmsg"></div>` +
		`<p class="muted">` + esc(l.T("passwordNote")) + `</p>`
}

func vaultLine(l Lang, vault vaultInfo) string {
	if vault.ID == "" {
		return fmt.Sprintf(`<p>%s</p><p>%s</p>`, esc(l.T("noVault")), btnLink("#/vault", l.T("chooseVault")))
	}
	return fmt.Sprintf(`<p>%s <strong>%s</strong></p><p>%s</p>`, esc(l.T("vaultHere")), esc(vault.Name), btnLink("#/vault", l.T("changeVault")))
}

// SettingsPage is what the gear at the top right opens: the interface language,
// and the way to the account and the vault.
func SettingsPage(l Lang, vault vaultInfo) Page {
	var b strings.Builder
	b.WriteString(langBox(l))
	fmt.Fprintf(&b, `<h2>%s</h2>`, esc(l.T("vault")))
	b.WriteString(vaultLine(l, vault))
	fmt.Fprintf(&b, `<h2>%s</h2><p>%s</p>`, esc(l.T("account")), btnLink("#/account", l.T("account")))
	return Page{Title: l.T("settings"), HTML: b.String()}
}

// AccountPage shows the Obsidian session. email is "" when there is none;
// msg explains why it could not be checked (no Wi-Fi, timeout).
func AccountPage(l Lang, email string, checked bool, msg string, formOnly bool, vault vaultInfo) Page {
	var b strings.Builder
	switch {
	case formOnly:
		fmt.Fprintf(&b, `<h2>%s</h2>`, esc(l.T("signInTitle")))
		b.WriteString(loginForm(l))
	case msg != "":
		fmt.Fprintf(&b, `<div class="syncbox"><p><strong>%s</strong></p></div>`, esc(msg))
		fmt.Fprintf(&b, `<p>%s %s</p>`, btnLink("javascript:app.retry()", l.T("retry")), btnLink("#/account/login", l.T("signIn")))
	case email != "":
		fmt.Fprintf(&b, `<div class="syncbox"><p>%s <strong>%s</strong></p><p class="muted">%s</p></div>`,
			esc(l.T("signedInAs")), esc(email), esc(l.T("readOnlyNote")))
		b.WriteString(vaultLine(l, vault))
		fmt.Fprintf(&b, `<p>%s</p>`, btnLink("#/account/login", l.T("signInAgain")))
		fmt.Fprintf(&b, `<p>%s</p><div id="loginmsg"></div>`, btnLink("javascript:app.logout()", l.T("signOut")))
	case checked:
		fmt.Fprintf(&b, `<div class="syncbox"><p><strong>%s</strong></p><p class="muted">%s</p></div>`, esc(l.T("noSession")), esc(l.T("noSessionHint")))
		b.WriteString(loginForm(l))
	}
	return Page{Title: l.T("account"), HTML: b.String()}
}

var vaultIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// VaultPage lists the account's vaults so one can be attached to this Kindle.
// errMsg explains why the list could not be loaded.
func VaultPage(l Lang, current vaultInfo, vaults []RemoteVault, errMsg string) Page {
	var b strings.Builder
	b.WriteString(`<div class="syncbox">`)
	if current.ID == "" {
		fmt.Fprintf(&b, `<p><strong>%s</strong></p>`, esc(l.T("noVault")))
	} else {
		fmt.Fprintf(&b, `<p>%s <strong>%s</strong></p>`, esc(l.T("vaultHere")), esc(current.Name))
	}
	fmt.Fprintf(&b, `<p class="muted">%s</p></div>`, esc(l.T("vaultHint")))
	switch {
	case errMsg != "":
		fmt.Fprintf(&b, `<p class="alert"><strong>%s</strong></p><p>%s %s</p>`, esc(errMsg),
			btnLink("javascript:app.retry()", l.T("retry")), btnLink("#/account", l.T("account")))
	case len(vaults) == 0:
		fmt.Fprintf(&b, `<p class="alert"><strong>%s</strong></p>`, esc(l.T("noVaults")))
	default:
		fmt.Fprintf(&b, `<h2>%s</h2><ul class="list">`, esc(l.T("yourVaults")))
		for _, rv := range vaults {
			if !vaultIDRe.MatchString(rv.ID) {
				continue
			}
			meta, change := "", current.ID != "" && current.ID != rv.ID
			if current.ID == rv.ID {
				meta = l.T("current")
			}
			b.WriteString(row(fmt.Sprintf("javascript:app.vault('%s',%v)", rv.ID, change), "", rv.Name, meta, ""))
		}
		b.WriteString(`</ul>`)
		fmt.Fprintf(&b, `<form class="login" onsubmit="return false" autocomplete="off"><label for="vp">%s <span class="muted">%s</span></label>`+
			`<input id="vp" name="vpass" type="password"></form>`, esc(l.T("vaultPassword")), esc(l.T("vaultPasswordHint")))
	}
	fmt.Fprintf(&b, `<div id="vaultmsg" data-confirm="%s" data-yes="%s" data-wait="%s"></div>`,
		esc(l.T("changeConfirm")), esc(l.T("changeYes")), esc(l.T("settingUp")))
	return Page{Title: l.T("vault"), HTML: b.String()}
}

func noteItem(l Lang, n *Note) string {
	where := n.Dir
	if where == "" {
		where = l.T("root")
	}
	return row(noteHref(n.Path), "", n.Title, where, "")
}

func folderItem(l Lang, v *Vault, d string) string {
	return row(folderHref(d), "folder", path.Base(d), l.N("note", v.CountIn(d)), "")
}

// row renders a list item whose whole area (full width and height) is one link.
func row(href, class, title, meta, extra string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<li class="%s"><a class="row" href="%s"><span class="t">%s</span>`, class, href, esc(title))
	if meta != "" {
		fmt.Fprintf(&b, `<span class="muted"> · %s</span>`, esc(meta))
	}
	b.WriteString(extra)
	b.WriteString(`</a></li>`)
	return b.String()
}

func HomePage(l Lang, v *Vault, st SyncState, vault vaultInfo) Page {
	var b strings.Builder
	b.WriteString(syncBox(l, st, vault))
	// The gear at the top opens the settings too; this is the same link in words.
	settings := fmt.Sprintf(`<a href="#/settings">%s</a>`, esc(l.T("settings")))
	if v.Count() == 0 && vault.Known && vault.ID == "" {
		fmt.Fprintf(&b, `<p class="muted">%s</p>`, settings)
		return Page{Title: "Vault", HTML: b.String()}
	}
	fmt.Fprintf(&b, `<h2>%s</h2><ul class="list">`, esc(l.T("folders")))
	folders, files, _ := v.Listing("")
	for _, d := range folders {
		b.WriteString(folderItem(l, v, d))
	}
	for _, f := range files {
		if n, ok := v.Note(f); ok {
			b.WriteString(noteItem(l, n))
		}
	}
	fmt.Fprintf(&b, `</ul><h2>%s</h2><ul class="list">`, esc(l.T("recent")))
	for _, n := range v.Recent(12) {
		b.WriteString(row(noteHref(n.Path), "", n.Title, l.Ago(n.ModTime), ""))
	}
	fmt.Fprintf(&b, `</ul><p class="muted">%s · <a href="#/tags">%s</a> · %s</p>`, esc(l.N("note", v.Count())), esc(l.T("seeTags")), settings)
	return Page{Title: "Vault", HTML: b.String()}
}

func breadcrumb(dir string) string {
	var b strings.Builder
	b.WriteString(`<p class="crumbs"><a href="#/">Vault</a>`)
	acc := ""
	for _, part := range strings.Split(dir, "/") {
		if part == "" || part == "." {
			continue
		}
		acc = strings.TrimPrefix(acc+"/"+part, "/")
		fmt.Fprintf(&b, ` / <a href="%s">%s</a>`, folderHref(acc), esc(part))
	}
	b.WriteString(`</p>`)
	return b.String()
}

func FolderPage(l Lang, v *Vault, dir string) (Page, bool) {
	folders, files, ok := v.Listing(dir)
	if !ok {
		return Page{}, false
	}
	var b strings.Builder
	b.WriteString(breadcrumb(path.Dir(dir)))
	b.WriteString(`<ul class="list">`)
	for _, d := range folders {
		b.WriteString(folderItem(l, v, d))
	}
	for _, f := range files {
		if n, ok := v.Note(f); ok {
			b.WriteString(row(noteHref(n.Path), "", n.Title, l.Ago(n.ModTime), ""))
		} else {
			fmt.Fprintf(&b, `<li class="muted">%s (%s)</li>`, esc(path.Base(f)), esc(l.T("cannotShow")))
		}
	}
	if len(folders)+len(files) == 0 {
		fmt.Fprintf(&b, `<li class="muted">%s</li>`, esc(l.T("emptyFolder")))
	}
	b.WriteString(`</ul>`)
	return Page{Title: path.Base(dir), HTML: b.String()}, true
}

func NotePage(l Lang, v *Vault, p string) (Page, bool) {
	n, ok := v.Note(p)
	if !ok {
		return Page{}, false
	}
	var b strings.Builder
	b.WriteString(breadcrumb(n.Dir))
	if !strings.HasPrefix(strings.TrimSpace(n.Body), "# ") {
		fmt.Fprintf(&b, `<h1>%s</h1>`, esc(n.Title))
	}
	if len(n.Tags) > 0 || n.Date != "" {
		b.WriteString(`<p class="meta">`)
		for _, t := range n.Tags {
			fmt.Fprintf(&b, `<a class="tag" href="%s">#%s</a> `, tagHref(t), esc(t))
		}
		if n.Date != "" {
			fmt.Fprintf(&b, `<span class="muted">· %s</span>`, esc(n.Date))
		}
		b.WriteString(`</p>`)
	}
	b.WriteString(`<div class="note">`)
	b.WriteString(v.RenderNote(n, l))
	b.WriteString(`</div>`)
	if bl := v.Backlinks(p); len(bl) > 0 {
		fmt.Fprintf(&b, `<h2 class="backlinks">%s</h2><ul class="list">`, esc(l.T("linkedFrom", len(bl))))
		for _, src := range bl {
			if sn, ok := v.Note(src); ok {
				b.WriteString(noteItem(l, sn))
			}
		}
		b.WriteString(`</ul>`)
	}
	return Page{Title: n.Title, HTML: b.String()}, true
}

func TagsPage(l Lang, v *Vault) Page {
	var b strings.Builder
	b.WriteString(`<ul class="list">`)
	for _, tc := range v.Tags() {
		b.WriteString(row(tagHref(tc.Tag), "", "#"+tc.Tag, l.N("note", tc.Count), ""))
	}
	b.WriteString(`</ul>`)
	return Page{Title: l.T("tags"), HTML: b.String()}
}

func TagPage(l Lang, v *Vault, tag string) Page {
	var b strings.Builder
	notes := v.Tagged(tag)
	fmt.Fprintf(&b, `<p class="muted">%s <strong>#%s</strong> · <a href="#/tags">%s</a></p><ul class="list">`,
		esc(l.T("taggedWith", l.N("note", len(notes)))), esc(tag), esc(l.T("allTags")))
	for _, n := range notes {
		b.WriteString(noteItem(l, n))
	}
	b.WriteString(`</ul>`)
	return Page{Title: "#" + tag, HTML: b.String()}
}

func SearchPage(l Lang, v *Vault, q string) Page {
	var b strings.Builder
	fmt.Fprintf(&b, `<form class="search" onsubmit="return app.search(this)"><input name="q" value="%s" placeholder="%s"><button>%s</button></form>`,
		esc(q), esc(l.T("searchHint")), esc(l.T("search")))
	if strings.TrimSpace(q) != "" {
		hits := v.Search(q, 60)
		fmt.Fprintf(&b, `<p class="muted">%s</p><ul class="list hits">`, esc(l.N("result", len(hits))))
		for _, h := range hits {
			b.WriteString(row(noteHref(h.Note.Path), "", h.Note.Title, h.Note.Dir, `<span class="snip">`+esc(h.Snippet)+`</span>`))
		}
		b.WriteString(`</ul>`)
	}
	return Page{Title: l.T("search"), HTML: b.String()}
}
