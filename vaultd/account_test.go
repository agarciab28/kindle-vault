package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubClient stands in for the client wrapper: it records its arguments and
// stdin. The account has two vaults; "bbb222" needs the password "vault pw".
func stubClient(t *testing.T) (acct *Account, dir string) {
	t.Helper()
	dir = t.TempDir()
	script := filepath.Join(dir, "ob")
	body := `#!/bin/sh
D="` + dir + `"
case "$1" in
  sync) exit 0 ;;
  current) [ -f "$D/current" ] && cat "$D/current"; exit 0 ;;
  vaults)
    if [ -f "$D/other-account" ]; then printf 'ccc333\tSomeone else\thost\n'; exit 0; fi
    printf 'aaa111\tWork notes\thost\nbbb222\tPersonal\thost\n'; exit 0 ;;
  forget) rm -f "$D/current"; echo forget >> "$D/calls"; echo Forgotten; exit 0 ;;
  logout) rm -f "$D/current"; echo logout >> "$D/calls"; echo "Logged out."; exit 0 ;;
  setup)
    echo "$@" > "$D/args"; cat > "$D/stdin"
    if [ "$2" = bbb222 ] && [ -z "$(cat "$D/stdin")" ]; then
      echo "Vault password: Error: this vault needs its encryption password"; exit 1
    fi
    if [ "$2" = bbb222 ] && [ "$(cat "$D/stdin")" != "vault pw" ]; then
      echo "Error: vault access denied"; exit 1
    fi
    [ "$2" = aaa111 ] && printf 'aaa111\tWork notes\n' > "$D/current"
    [ "$2" = bbb222 ] && printf 'bbb222\tPersonal\n' > "$D/current"
    echo "Configured"; exit 0 ;;
  login) echo "$@" > "$D/args"; cat > "$D/stdin"; echo 'Login successful.' ;;
esac
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSyncer(filepath.Join(dir, "state.json"), []string{"/bin/sh", script, "sync"}, nil, func() {})
	t.Cleanup(func() { // let the sync that follows a login finish before the temp dir goes away
		for i := 0; i < 100 && s.State().Running; i++ {
			time.Sleep(20 * time.Millisecond)
		}
	})
	return &Account{command: []string{"/bin/sh", script}, syncer: s, onNotesChanged: func() {
		f, _ := os.OpenFile(filepath.Join(dir, "calls"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		_, _ = f.WriteString("reindex\n")
		_ = f.Close()
	}}, dir
}

func waitIdle(a *Account) {
	for i := 0; i < 200 && a.syncer.State().Running; i++ {
		time.Sleep(10 * time.Millisecond)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(dir, name))
	return string(b)
}

func TestLoginNeedsTheNoticeAccepted(t *testing.T) {
	acct, dir := stubClient(t)
	res := acct.Login(EN, "me@example.com", "secret pass", "", false)
	if res.OK || !strings.Contains(res.Message, "not an official Obsidian app") {
		t.Fatalf("login without accepting the notice: %+v", res)
	}
	if es := acct.Login(ES, "me@example.com", "secret pass", "", false); !strings.Contains(es.Message, "no es una app oficial") {
		t.Fatalf("the Spanish message: %+v", es)
	}
	if read(t, dir, "args") != "" {
		t.Fatal("the client ran although the notice was not accepted")
	}
}

func TestLoginPassesThePasswordOnStdinOnly(t *testing.T) {
	acct, dir := stubClient(t)
	res := acct.Login(EN, " me@example.com ", "secret pass", "", true)
	if !res.OK || res.Next != "vault" {
		t.Fatalf("first login should succeed and send the reader to choose a vault: %+v", res)
	}
	if got := strings.TrimSpace(read(t, dir, "args")); got != "login --accept-disclaimer --email me@example.com" {
		t.Fatalf("client arguments: %q", got)
	}
	if strings.Contains(read(t, dir, "args"), "secret") {
		t.Fatal("the password reached the command line")
	}
	if stdin := read(t, dir, "stdin"); stdin != "secret pass\n" {
		t.Fatalf("stdin: %q", stdin)
	}
}

func TestValidLogin(t *testing.T) {
	for _, c := range []struct {
		email, password, mfa string
		ok                   bool
	}{
		{"me@example.com", "pw", "", true},
		{"me@example.com", "with spaces ok", "123456", true},
		{"", "pw", "", false},
		{"me@example.com", "", "", false},
		{"no-at-sign", "pw", "", false},
		{"me @example.com", "pw", "", false},
		{"--flag@example.com", "pw", "", false},
		{"me@example.com", "two\nlines", "", false},
		{"me@example.com", "pw", "12a456", false},
		{"me@example.com", "pw", "--flag", false},
	} {
		if key := validLogin(c.email, c.password, c.mfa); (key == "") != c.ok {
			t.Errorf("validLogin(%q, %q, %q) = %q, want ok=%v", c.email, c.password, c.mfa, key, c.ok)
		}
	}
}

func TestVaultSetup(t *testing.T) {
	acct, dir := stubClient(t)
	if v := acct.RefreshVault(); !v.Known || v.ID != "" {
		t.Fatalf("a new Kindle has no vault: %+v", v)
	}
	vaults, err := acct.Vaults()
	if err != nil || len(vaults) != 2 || vaults[1] != (RemoteVault{"bbb222", "Personal"}) {
		t.Fatalf("vaults: %v %v", vaults, err)
	}

	// Only ids of the account's own vaults reach the client.
	for _, id := range []string{"", "zzz999", "--path", "aaa111 --mode bidirectional", "../x"} {
		if res := acct.Setup(EN, id, ""); res.OK {
			t.Errorf("Setup(%q) succeeded", id)
		}
	}
	if read(t, dir, "args") != "" {
		t.Fatalf("the client's setup ran with a bad id: %q", read(t, dir, "args"))
	}

	res := acct.Setup(EN, "aaa111", "")
	if !res.OK || res.Next != "sync" || !strings.Contains(res.Message, "Work notes") {
		t.Fatalf("setup: %+v", res)
	}
	if got := strings.TrimSpace(read(t, dir, "args")); got != "setup aaa111" {
		t.Fatalf("client arguments: %q", got)
	}
	if v := acct.Vault(); v.ID != "aaa111" || v.Name != "Work notes" {
		t.Fatalf("vault after setup: %+v", v)
	}

	waitIdle(acct) // the sync that a successful setup starts

	// A vault with its own password and none given: a clear error, in the reader's language.
	if res := acct.Setup(ES, "bbb222", ""); res.OK || !res.NeedPassword || !strings.Contains(res.Message, "propia contraseña de cifrado") {
		t.Fatalf("setup without the vault password: %+v", res)
	}
	if res := acct.Setup(EN, "bbb222", ""); res.OK || !strings.Contains(res.Message, "has its own encryption password") {
		t.Fatalf("setup without the vault password: %+v", res)
	}
	if res := acct.Setup(EN, "bbb222", "nope"); res.OK || !res.NeedPassword || !strings.Contains(res.Message, "Could not open the vault with that password") {
		t.Fatalf("setup with a wrong vault password: %+v", res)
	}
	if res := acct.Setup(EN, "bbb222", "vault pw"); !res.OK {
		t.Fatalf("setup with the vault password: %+v", res)
	}
	if strings.Contains(read(t, dir, "args"), "vault pw") || read(t, dir, "stdin") != "vault pw\n" {
		t.Fatalf("vault password: args %q, stdin %q", read(t, dir, "args"), read(t, dir, "stdin"))
	}
}

func TestLogoutForgetsTheVaultAndReindexes(t *testing.T) {
	acct, dir := stubClient(t)
	if res := acct.Setup(EN, "aaa111", ""); !res.OK {
		t.Fatalf("setup: %+v", res)
	}
	waitIdle(acct)
	_ = os.Remove(filepath.Join(dir, "calls"))
	res := acct.Logout(EN)
	if !res.OK || !strings.Contains(res.Message, "notes were removed from this Kindle") {
		t.Fatalf("logout: %+v", res)
	}
	if v := acct.Vault(); v.ID != "" {
		t.Fatalf("a vault is still attached after signing out: %+v", v)
	}
	if calls := read(t, dir, "calls"); calls != "logout\nreindex\n" {
		t.Fatalf("after signing out the notes index must be rebuilt: %q", calls)
	}
}

// Signing in with an account that does not own the vault that is on the
// Kindle removes that vault and its notes before anything is shown.
func TestLoginWithAnotherAccountForgetsTheVault(t *testing.T) {
	acct, dir := stubClient(t)
	if res := acct.Setup(EN, "aaa111", ""); !res.OK {
		t.Fatalf("setup: %+v", res)
	}
	waitIdle(acct)
	// Same account again: the vault stays.
	if res := acct.Login(EN, "me@example.com", "pw", "", true); !res.OK || res.Next != "sync" || acct.Vault().ID != "aaa111" {
		t.Fatalf("login again with the same account: %+v, vault %+v", res, acct.Vault())
	}
	waitIdle(acct)
	_ = os.Remove(filepath.Join(dir, "calls"))
	if err := os.WriteFile(filepath.Join(dir, "other-account"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	res := acct.Login(EN, "other@example.com", "pw", "", true)
	if !res.OK || res.Next != "vault" {
		t.Fatalf("login with another account should lead to choosing a vault: %+v", res)
	}
	if v := acct.Vault(); v.ID != "" {
		t.Fatalf("the other account's vault is still attached: %+v", v)
	}
	if calls := read(t, dir, "calls"); calls != "forget\nreindex\n" {
		t.Fatalf("the vault must be forgotten and the index rebuilt: %q", calls)
	}
}
