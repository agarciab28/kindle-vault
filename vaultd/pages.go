package main

import (
	"fmt"
	"html"
	"path"
	"strings"
	"time"
)

// Page is what the WAF shell receives for every route.
type Page struct {
	Title string `json:"title"`
	HTML  string `json:"html"`
}

var esc = html.EscapeString

var meses = []string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

func fecha(t time.Time) string {
	if t.IsZero() {
		return "nunca"
	}
	t = t.Local()
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), meses[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

func hace(t time.Time) string {
	if t.IsZero() {
		return "nunca"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "hace un momento"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("hace %d días", int(d.Hours()/24))
	}
}

func syncBox(st SyncState) string {
	var b strings.Builder
	b.WriteString(`<div class="syncbox" id="syncbox">`)
	switch {
	case st.Running:
		b.WriteString(`<p><strong>Sincronizando…</strong></p>`)
	case !st.LastOK && !st.LastAttempt.IsZero():
		fmt.Fprintf(&b, `<p><strong>Falló la última sincronización</strong> (%s)</p><pre class="err">%s</pre>`, esc(hace(st.LastAttempt)), esc(st.LastError))
		fmt.Fprintf(&b, `<p class="muted">Último sync correcto: %s</p>`, esc(fecha(st.LastSync)))
	default:
		fmt.Fprintf(&b, `<p>Sincronizada <strong>%s</strong> · %s</p>`, esc(hace(st.LastSync)), esc(fecha(st.LastSync)))
		if st.LastChanged > 0 {
			fmt.Fprintf(&b, `<p class="muted">%d archivos actualizados en el último sync</p>`, st.LastChanged)
		}
	}
	if !st.Running && !st.LastOK && authError(st.LastError) {
		b.WriteString(`<p><strong>Parece que la sesión de Obsidian venció.</strong></p><p><a class="btn" href="#/account/login">Iniciar sesión</a> <a class="btn" href="#/sync">Reintentar</a></p></div>`)
		return b.String()
	}
	b.WriteString(`<p><a class="btn" href="#/sync">Sincronizar ahora</a> <a class="btn" href="#/account">Cuenta</a></p></div>`)
	return b.String()
}

func loginForm() string {
	return `<form class="login" onsubmit="return app.login(this)" autocomplete="off">` +
		`<label for="lf-email">Correo</label>` +
		`<input id="lf-email" name="email" type="text" autocapitalize="off" autocorrect="off" spellcheck="false">` +
		`<label for="lf-pass">Contraseña</label>` +
		`<input id="lf-pass" name="password" type="password">` +
		`<label for="lf-mfa">Código de verificación <span class="muted">(solo si lo tienes activado)</span></label>` +
		`<input id="lf-mfa" name="mfa" type="text" maxlength="10">` +
		`<p><button type="submit">Iniciar sesión</button></p></form>` +
		`<div id="loginmsg"></div>` +
		`<p class="muted">La contraseña va directo a Obsidian: el Kindle no la guarda, solo la sesión. Si ya había una sesión en este Kindle, se reemplaza.</p>`
}

// AccountPage shows the Obsidian session. email is "" when there is none;
// msg explains why it could not be checked (no Wi-Fi, timeout).
func AccountPage(email string, checked bool, msg string, formOnly bool) Page {
	var b strings.Builder
	switch {
	case formOnly:
		b.WriteString(`<h2>Iniciar sesión en Obsidian</h2>`)
		b.WriteString(loginForm())
	case msg != "":
		fmt.Fprintf(&b, `<div class="syncbox"><p><strong>%s</strong></p></div>`, esc(msg))
		b.WriteString(`<p><a class="btn" href="javascript:app.retry()">Reintentar</a> <a class="btn" href="#/account/login">Iniciar sesión</a></p>`)
	case email != "":
		fmt.Fprintf(&b, `<div class="syncbox"><p>Sesión iniciada como <strong>%s</strong></p><p class="muted">Este Kindle descarga la bóveda en solo lectura: nunca sube ni borra nada.</p></div>`, esc(email))
		b.WriteString(`<p><a class="btn" href="#/account/login">Iniciar sesión de nuevo</a></p>`)
		b.WriteString(`<p><a class="btn" href="javascript:app.logout()">Cerrar sesión en este Kindle</a></p><div id="loginmsg"></div>`)
	case checked:
		b.WriteString(`<div class="syncbox"><p><strong>No hay una sesión activa, o ya venció.</strong></p><p class="muted">Inicia sesión para que la bóveda se siga actualizando. Las notas descargadas siguen aquí.</p></div>`)
		b.WriteString(loginForm())
	}
	return Page{Title: "Cuenta", HTML: b.String()}
}

func noteItem(n *Note) string {
	where := n.Dir
	if where == "" {
		where = "raíz"
	}
	return row(noteHref(n.Path), "", n.Title, where, "")
}

func folderItem(v *Vault, d string) string {
	return row(folderHref(d), "folder", path.Base(d), fmt.Sprintf("%d notas", v.CountIn(d)), "")
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

func HomePage(v *Vault, st SyncState) Page {
	var b strings.Builder
	b.WriteString(syncBox(st))
	b.WriteString(`<h2>Carpetas</h2><ul class="list">`)
	folders, files, _ := v.Listing("")
	for _, d := range folders {
		b.WriteString(folderItem(v, d))
	}
	for _, f := range files {
		if n, ok := v.Note(f); ok {
			b.WriteString(noteItem(n))
		}
	}
	b.WriteString(`</ul><h2>Cambiadas recientemente</h2><ul class="list">`)
	for _, n := range v.Recent(12) {
		b.WriteString(row(noteHref(n.Path), "", n.Title, hace(n.ModTime), ""))
	}
	fmt.Fprintf(&b, `</ul><p class="muted">%d notas · <a href="#/tags">ver tags</a></p>`, v.Count())
	return Page{Title: "Bóveda", HTML: b.String()}
}

func breadcrumb(dir string) string {
	var b strings.Builder
	b.WriteString(`<p class="crumbs"><a href="#/">Bóveda</a>`)
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

func FolderPage(v *Vault, dir string) (Page, bool) {
	folders, files, ok := v.Listing(dir)
	if !ok {
		return Page{}, false
	}
	var b strings.Builder
	b.WriteString(breadcrumb(path.Dir(dir)))
	b.WriteString(`<ul class="list">`)
	for _, d := range folders {
		b.WriteString(folderItem(v, d))
	}
	for _, f := range files {
		if n, ok := v.Note(f); ok {
			b.WriteString(row(noteHref(n.Path), "", n.Title, hace(n.ModTime), ""))
		} else {
			fmt.Fprintf(&b, `<li class="muted">%s (no se puede mostrar)</li>`, esc(path.Base(f)))
		}
	}
	if len(folders)+len(files) == 0 {
		b.WriteString(`<li class="muted">Carpeta vacía</li>`)
	}
	b.WriteString(`</ul>`)
	return Page{Title: path.Base(dir), HTML: b.String()}, true
}

func NotePage(v *Vault, p string) (Page, bool) {
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
	b.WriteString(v.RenderNote(n))
	b.WriteString(`</div>`)
	if bl := v.Backlinks(p); len(bl) > 0 {
		fmt.Fprintf(&b, `<h2 class="backlinks">Enlazado desde (%d)</h2><ul class="list">`, len(bl))
		for _, src := range bl {
			if sn, ok := v.Note(src); ok {
				b.WriteString(noteItem(sn))
			}
		}
		b.WriteString(`</ul>`)
	}
	return Page{Title: n.Title, HTML: b.String()}, true
}

func TagsPage(v *Vault) Page {
	var b strings.Builder
	b.WriteString(`<ul class="list">`)
	for _, tc := range v.Tags() {
		b.WriteString(row(tagHref(tc.Tag), "", "#"+tc.Tag, fmt.Sprintf("%d notas", tc.Count), ""))
	}
	b.WriteString(`</ul>`)
	return Page{Title: "Tags", HTML: b.String()}
}

func TagPage(v *Vault, tag string) Page {
	var b strings.Builder
	notes := v.Tagged(tag)
	fmt.Fprintf(&b, `<p class="muted">%d notas con <strong>#%s</strong> · <a href="#/tags">todos los tags</a></p><ul class="list">`, len(notes), esc(tag))
	for _, n := range notes {
		b.WriteString(noteItem(n))
	}
	b.WriteString(`</ul>`)
	return Page{Title: "#" + tag, HTML: b.String()}
}

func SearchPage(v *Vault, q string) Page {
	var b strings.Builder
	fmt.Fprintf(&b, `<form class="search" onsubmit="return app.search(this)"><input name="q" value="%s" placeholder="Buscar en la bóveda"><button>Buscar</button></form>`, esc(q))
	if strings.TrimSpace(q) != "" {
		hits := v.Search(q, 60)
		fmt.Fprintf(&b, `<p class="muted">%d resultados</p><ul class="list hits">`, len(hits))
		for _, h := range hits {
			b.WriteString(row(noteHref(h.Note.Path), "", h.Note.Title, h.Note.Dir, `<span class="snip">`+esc(h.Snippet)+`</span>`))
		}
		b.WriteString(`</ul>`)
	}
	return Page{Title: "Buscar", HTML: b.String()}
}
