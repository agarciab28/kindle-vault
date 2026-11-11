package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var verbRe = regexp.MustCompile(`%[a-z]`)

// Both languages have the same keys, with the same placeholders.
func TestLanguagesMatch(t *testing.T) {
	for key, en := range text[EN] {
		es, ok := text[ES][key]
		if !ok {
			t.Errorf("missing in Spanish: %q", key)
			continue
		}
		if a, b := strings.Join(verbRe.FindAllString(en, -1), ""), strings.Join(verbRe.FindAllString(es, -1), ""); a != b {
			t.Errorf("%q: placeholders differ (%q vs %q)", key, a, b)
		}
	}
	for key := range text[ES] {
		if _, ok := text[EN][key]; !ok {
			t.Errorf("missing in English: %q", key)
		}
	}
	if parseLang("") != EN || parseLang("fr") != EN || parseLang("es") != ES {
		t.Error("English must be the default language")
	}
}

func testVault(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"Notas/Café.md":  "---\ntags: [cocina]\n---\n# Café de olla\n\nHierve el agua con canela. Ver [[Receta base]].\n\n> [!warning]\n> Cuidado con el piloncillo.\n\n> [!tip] Mi truco\n> Usa olla de barro.\n",
		"Receta base.md": "Texto en español, sin cambios.\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v := NewVault(dir)
	if err := v.Reindex(); err != nil {
		t.Fatal(err)
	}
	return v
}

// The interface language changes the app's own words and nothing of the notes.
func TestLanguageDoesNotTouchTheNotes(t *testing.T) {
	v := testVault(t)
	vault := vaultInfo{Known: true, ID: "aaa111", Name: "Mis notas"}
	st := SyncState{LastOK: true, LastSync: time.Now().Add(-2 * time.Hour), LastAttempt: time.Now().Add(-2 * time.Hour)}

	en, es := HomePage(EN, v, st, vault).HTML, HomePage(ES, v, st, vault).HTML
	for _, want := range []string{"Synced", "2 h ago", "Folders", "Recently changed", "Sync now", `href="#/settings">Settings<`} {
		if !strings.Contains(en, want) {
			t.Errorf("English home page lacks %q", want)
		}
	}
	for _, want := range []string{"Sincronizada", "hace 2 h", "Carpetas", "Cambiadas recientemente", "Sincronizar ahora", `href="#/settings">Ajustes<`} {
		if !strings.Contains(es, want) {
			t.Errorf("Spanish home page lacks %q", want)
		}
	}
	for _, page := range []string{en, es} { // note and folder names, as written
		if !strings.Contains(page, "Notas") || !strings.Contains(page, "Receta base") {
			t.Error("a home page lost a folder or note name")
		}
	}

	pEN, ok1 := NotePage(EN, v, "Notas/Café.md")
	pES, ok2 := NotePage(ES, v, "Notas/Café.md")
	if !ok1 || !ok2 {
		t.Fatal("note not found")
	}
	if pEN.Title != pES.Title {
		t.Errorf("the note title changed with the language: %q vs %q", pEN.Title, pES.Title)
	}
	for _, page := range []string{pEN.HTML, pES.HTML} {
		for _, want := range []string{"Café de olla", "Hierve el agua con canela", "Cuidado con el piloncillo", "Mi truco", "Usa olla de barro", "#cocina"} {
			if !strings.Contains(page, want) {
				t.Errorf("a note page lacks %q", want)
			}
		}
	}
	// Only what the app adds differs: the default title of a callout that has none.
	if !strings.Contains(pEN.HTML, `callout-title">Warning<`) || !strings.Contains(pES.HTML, `callout-title">Atención<`) {
		t.Error("default callout titles should follow the interface language")
	}
	strip := strings.NewReplacer(`callout-title">Warning<`, "", `callout-title">Atención<`, "", "Linked from (1)", "", "Enlazado desde (1)", "")
	if a, b := strip.Replace(pEN.HTML), strip.Replace(pES.HTML); a != b {
		t.Errorf("the note differs between languages beyond the app's own labels:\n%s\n---\n%s", a, b)
	}
}

// The language is chosen on the settings page (the gear at the top right).
func TestSettingsPage(t *testing.T) {
	en := SettingsPage(EN, vaultInfo{Known: true, ID: "aaa111", Name: "Mis notas"})
	es := SettingsPage(ES, vaultInfo{Known: true})
	if en.Title != "Settings" || es.Title != "Ajustes" {
		t.Errorf("titles: %q, %q", en.Title, es.Title)
	}
	for _, want := range []string{"Language", `class="btn on" href="javascript:app.lang('en')"`, `class="btn" href="javascript:app.lang('es')"`, "Mis notas", "Change vault", `href="#/account"`} {
		if !strings.Contains(en.HTML, want) {
			t.Errorf("English settings page lacks %q", want)
		}
	}
	for _, want := range []string{"Idioma", `class="btn on" href="javascript:app.lang('es')"`, "Elegir bóveda", "Tus notas se quedan tal como están escritas"} {
		if !strings.Contains(es.HTML, want) {
			t.Errorf("Spanish settings page lacks %q", want)
		}
	}
}

func TestFirstRunAsksForAVault(t *testing.T) {
	v := NewVault(t.TempDir())
	_ = v.Reindex()
	home := HomePage(EN, v, SyncState{}, vaultInfo{Known: true}).HTML
	if !strings.Contains(home, "No vault chosen yet") || !strings.Contains(home, `href="#/vault"`) || strings.Contains(home, "Sync now") {
		t.Errorf("first-run home page: %s", home)
	}
	page := VaultPage(EN, vaultInfo{Known: true, ID: "aaa111", Name: "Work"}, []RemoteVault{{"aaa111", "Work"}, {"bbb222", "Personal <b>"}, {"bad id'", "x"}}, "").HTML
	if !strings.Contains(page, "app.vault('aaa111',false)") || !strings.Contains(page, "app.vault('bbb222',true)") {
		t.Errorf("vault page: changing to another vault must ask first: %s", page)
	}
	if strings.Contains(page, "bad id") || strings.Contains(page, "<b>") {
		t.Errorf("vault page did not escape or filter: %s", page)
	}
}
