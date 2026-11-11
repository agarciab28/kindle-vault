package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubClient stands in for ob-kindle.sh: it records its arguments and stdin.
func stubClient(t *testing.T) (acct *Account, argsFile, stdinFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile, stdinFile = filepath.Join(dir, "args"), filepath.Join(dir, "stdin")
	script := filepath.Join(dir, "ob.sh")
	body := "#!/bin/sh\nif [ \"$1\" = sync ]; then exit 0; fi\necho \"$@\" > " + argsFile + "\ncat > " + stdinFile + "\necho 'Login successful.'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSyncer(filepath.Join(dir, "state.json"), []string{"/bin/sh", script, "sync"}, nil, func() {})
	t.Cleanup(func() { // let the sync that follows a login finish before the temp dir goes away
		for i := 0; i < 100 && s.State().Running; i++ {
			time.Sleep(20 * time.Millisecond)
		}
	})
	return &Account{command: []string{"/bin/sh", script}, syncer: s}, argsFile, stdinFile
}

func TestLoginNeedsTheNoticeAccepted(t *testing.T) {
	acct, argsFile, _ := stubClient(t)
	res := acct.Login("me@example.com", "secret pass", "", false)
	if res.OK || !strings.Contains(res.Message, "no es una app oficial") {
		t.Fatalf("login without accepting the notice: %+v", res)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Fatal("the client ran although the notice was not accepted")
	}
}

func TestLoginPassesThePasswordOnStdinOnly(t *testing.T) {
	acct, argsFile, stdinFile := stubClient(t)
	res := acct.Login(" me@example.com ", "secret pass", "", true)
	if !res.OK {
		t.Fatalf("login failed: %+v", res)
	}
	args, _ := os.ReadFile(argsFile)
	if got := strings.TrimSpace(string(args)); got != "login --accept-disclaimer --email me@example.com" {
		t.Fatalf("client arguments: %q", got)
	}
	if strings.Contains(string(args), "secret") {
		t.Fatal("the password reached the command line")
	}
	if stdin, _ := os.ReadFile(stdinFile); string(stdin) != "secret pass\n" {
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
		{"me@example.com", "two\nlines", "", false},
		{"me@example.com", "pw", "12a456", false},
		{"me@example.com", "pw", "--flag", false},
	} {
		if msg := validLogin(c.email, c.password, c.mfa); (msg == "") != c.ok {
			t.Errorf("validLogin(%q, %q, %q) = %q, want ok=%v", c.email, c.password, c.mfa, msg, c.ok)
		}
	}
}
