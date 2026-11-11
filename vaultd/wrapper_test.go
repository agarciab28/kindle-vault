package main

// Tests of the client wrapper (../kindle/ob) itself, with a fake sync client:
// this Kindle must only ever hold the notes of the vault it is attached to.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeClient plays ob-kindle. The account has "v1" (Uno, no password) and
// "v2" (Dos, encryption password "pw"). Links are lines "id<TAB>name<TAB>path".
const fakeClient = `#!/bin/sh
S="$HOME/fake"; mkdir -p "$S"; touch "$S/links"
cmd="$1"; shift
vault=""; path=""; mode=""
while [ $# -gt 0 ]; do
  case "$1" in
    --vault) vault="$2"; shift ;;
    --path) path="$2"; shift ;;
    --mode) mode="$2"; shift ;;
  esac
  shift
done
case "$cmd" in
  sync-list-local) cat "$S/links" ;;
  sync-setup)
    IFS= read -r pass || true
    case "$vault" in
      v1|Uno) id=v1; name=Uno ;;
      v2|Dos) id=v2; name=Dos
        if [ -z "$pass" ]; then echo "Vault password: "; echo "Error: this vault needs its encryption password" >&2; exit 1; fi
        if [ "$pass" != pw ]; then echo "Error: vault access denied" >&2; exit 1; fi ;;
      *) echo "Error: vault not found" >&2; exit 1 ;;
    esac
    grep -v "^$id	" "$S/links" > "$S/links.new"; mv "$S/links.new" "$S/links"
    printf '%s\t%s\t%s\n' "$id" "$name" "$path" >> "$S/links"
    mkdir -p "$path"; echo "Configured $name at $path" ;;
  sync-unlink)
    grep -q "	$path\$" "$S/links" || { echo "Error: no sync config for $path" >&2; exit 1; }
    grep -v "	$path\$" "$S/links" > "$S/links.new"; mv "$S/links.new" "$S/links" ;;
  sync-config)
    if [ -n "$mode" ]; then
      mkdir -p "$HOME/.config/obsidian-headless/sync/x"
      printf '{"syncMode": "%s"}\n' "$mode" > "$HOME/.config/obsidian-headless/sync/x/config.json"
    fi ;;
  sync) id=$(grep "	$path\$" "$S/links" | cut -f1); echo "from $id" > "$path/note-$id.md"; echo "downloaded file note-$id.md" ;;
  logout) echo "Logged out." ;;
esac
`

type wrapper struct {
	t           *testing.T
	script, env string
	vault, home string
}

func newWrapper(t *testing.T) *wrapper {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "kindle", "ob"))
	if err != nil {
		t.Fatal(err)
	}
	w := &wrapper{t: t, script: filepath.Join(dir, "app", "ob"), vault: filepath.Join(dir, "vault"), home: filepath.Join(dir, "home")}
	if err := os.MkdirAll(filepath.Dir(w.script), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(w.script, src, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "app", "ob-kindle"), []byte(fakeClient), 0o755)
	return w
}

// run calls the wrapper with stdin; it returns its output and whether it succeeded.
func (w *wrapper) run(stdin string, args ...string) (string, bool) {
	w.t.Helper()
	cmd := exec.Command("/bin/sh", append([]string{w.script}, args...)...)
	cmd.Env = append(os.Environ(), "VAULT_NOTES_DIR="+w.vault, "VAULT_CLIENT_HOME="+w.home, "VAULT_NAME=")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// notes lists what is in the vault folder.
func (w *wrapper) notes() string {
	entries, _ := os.ReadDir(w.vault)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

func (w *wrapper) current() string {
	out, _ := w.run("", "current")
	return strings.TrimSpace(out)
}

func (w *wrapper) must(ok bool, what, out string) {
	w.t.Helper()
	if !ok {
		w.t.Fatalf("%s failed: %s", what, out)
	}
}

func TestWrapperKeepsOnlyTheAttachedVaultsNotes(t *testing.T) {
	w := newWrapper(t)

	// Nothing attached yet: no sync.
	if out, ok := w.run("", "sync"); ok || !strings.Contains(out, "STOP: no vault chosen") {
		t.Fatalf("sync without a vault: %q", out)
	}

	// First vault. Notes left in the folder by something else are not kept.
	_ = os.MkdirAll(filepath.Join(w.vault, "old folder"), 0o755)
	_ = os.WriteFile(filepath.Join(w.vault, "old folder", "leftover.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(w.vault, ".hidden.md"), []byte("x"), 0o644)
	out, ok := w.run("\n", "setup", "v1")
	w.must(ok, "setup v1", out)
	if w.current() != "v1\tUno" || w.notes() != "" {
		t.Fatalf("after the first setup: current %q, notes %q", w.current(), w.notes())
	}
	out, ok = w.run("", "sync")
	w.must(ok, "sync", out)
	if w.notes() != "note-v1.md" {
		t.Fatalf("notes after syncing v1: %q", w.notes())
	}

	// Choosing the same vault again keeps everything.
	out, ok = w.run("\n", "setup", "Uno")
	w.must(ok, "setup of the same vault", out)
	if w.current() != "v1\tUno" || w.notes() != "note-v1.md" {
		t.Fatalf("choosing the same vault again: current %q, notes %q", w.current(), w.notes())
	}

	// A vault with its own password, and none given: an error, and nothing changes.
	out, ok = w.run("\n", "setup", "v2")
	if ok || !strings.Contains(out, "needs its encryption password") {
		t.Fatalf("setup of a protected vault without its password: ok=%v %q", ok, out)
	}
	if w.current() != "v1\tUno" || w.notes() != "note-v1.md" {
		t.Fatalf("a failed change must leave the current vault as it was: current %q, notes %q", w.current(), w.notes())
	}
	// Same with a wrong password.
	if out, ok = w.run("nope\n", "setup", "v2"); ok {
		t.Fatalf("setup with a wrong password succeeded: %q", out)
	}
	if w.current() != "v1\tUno" || w.notes() != "note-v1.md" {
		t.Fatalf("a wrong password must leave the current vault as it was: current %q, notes %q", w.current(), w.notes())
	}
	if _, err := os.Stat(filepath.Join(w.home, "setup-check")); err == nil {
		t.Error("the scratch folder of the check was left behind")
	}

	// Changing vault for real: the first vault's notes are gone before the new ones arrive.
	out, ok = w.run("pw\n", "setup", "v2")
	w.must(ok, "setup v2", out)
	if w.current() != "v2\tDos" || w.notes() != "" {
		t.Fatalf("after changing vault: current %q, notes %q (the other vault's notes must be gone)", w.current(), w.notes())
	}
	links, _ := os.ReadFile(filepath.Join(w.home, "fake", "links"))
	if strings.Count(string(links), "\n") != 1 {
		t.Fatalf("only one vault may be attached: %q", links)
	}
	out, ok = w.run("", "sync")
	w.must(ok, "sync", out)
	if w.notes() != "note-v2.md" {
		t.Fatalf("notes after syncing v2: %q", w.notes())
	}

	// Signing out removes the notes and the link.
	out, ok = w.run("", "logout")
	w.must(ok, "logout", out)
	if !strings.Contains(out, "Logged out.") || w.current() != "" || w.notes() != "" {
		t.Fatalf("after signing out: %q, current %q, notes %q", out, w.current(), w.notes())
	}
	// And again, with nothing attached, it still works.
	if out, ok = w.run("", "logout"); !ok {
		t.Fatalf("signing out twice: %q", out)
	}
}
