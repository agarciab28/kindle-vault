package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Account manages the Obsidian session and the vault attached to this Kindle,
// through the client wrapper (login / logout / whoami / vaults / current / setup / forget).
//
// This Kindle only ever shows the notes of the vault it is attached to: signing
// out, changing vault or signing in with another account removes the
// downloaded notes (from the Kindle only).
//
// Passwords only travel app -> vaultd (localhost, token-checked POST) -> the
// client's stdin. They are never written to disk, logged, or put on a command
// line; the client keeps only the session token Obsidian returns.
type Account struct {
	command []string // e.g. ["/bin/sh", "/mnt/us/documents/vault-app/ob"]
	syncer  *Syncer
	// onNotesChanged runs after the notes on disk were removed or replaced, so
	// the index never keeps showing what is no longer there.
	onNotesChanged func()

	mu    sync.Mutex
	vault vaultInfo // cached: asking the client means starting a 30 MB binary
}

// AccountResult is what the app shows after login, logout or choosing a vault.
type AccountResult struct {
	OK           bool   `json:"ok"`
	Message      string `json:"message"`
	Need2FA      bool   `json:"need2fa,omitempty"`
	NeedPassword bool   `json:"needPassword,omitempty"`
	Next         string `json:"next,omitempty"` // where the app goes after a success: "vault" or "sync"
}

var loggedInRe = regexp.MustCompile(`Logged in as: (\S+)`)

var errTimeout = errors.New("timeout")

func (a *Account) run(timeout time.Duration, stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.command[0], append(append([]string{}, a.command[1:]...), args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), errTimeout
	}
	return out.String(), err
}

// Status returns the email of the saved session, or "" if there is none or it
// is no longer valid. It needs the network.
func (a *Account) Status() (email string, err error) {
	out, err := a.run(20*time.Second, "", "whoami")
	if m := loggedInRe.FindStringSubmatch(out); m != nil {
		return m[1], nil
	}
	if err == errTimeout {
		return "", err
	}
	return "", nil
}

func (a *Account) notesChanged() {
	a.RefreshVault()
	if a.onNotesChanged != nil {
		a.onNotesChanged()
	}
}

// Vault returns the vault attached to this Kindle (cached).
func (a *Account) Vault() vaultInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.vault
}

// RefreshVault asks the client which vault is attached. It does not use the network.
func (a *Account) RefreshVault() vaultInfo {
	out, err := a.run(20*time.Second, "", "current")
	info := vaultInfo{Known: err == nil}
	if err == nil {
		if id, name, ok := strings.Cut(strings.TrimSpace(lastLines(out, 1)), "\t"); ok {
			info.ID, info.Name = id, name
		}
	}
	a.mu.Lock()
	if err == nil || !a.vault.Known {
		a.vault = info
	}
	info = a.vault
	a.mu.Unlock()
	return info
}

// Vaults lists the account's vaults. It needs the network and a session.
func (a *Account) Vaults() ([]RemoteVault, error) {
	out, err := a.run(25*time.Second, "", "vaults")
	if err != nil {
		if err == errTimeout {
			return nil, err
		}
		return nil, errors.New(clientError(out, err))
	}
	var vaults []RemoteVault
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Split(strings.TrimRight(line, "\r"), "\t"); len(f) >= 2 && vaultIDRe.MatchString(f[0]) {
			vaults = append(vaults, RemoteVault{ID: f[0], Name: f[1]})
		}
	}
	return vaults, nil
}

// validLogin checks the form before anything reaches the client. It returns
// the key of the message to show, or "".
func validLogin(email, password, mfa string) string {
	if email == "" || password == "" {
		return "needCreds"
	}
	if len(email) > 254 || strings.IndexFunc(email, unicode.IsSpace) >= 0 || !strings.Contains(email, "@") || strings.HasPrefix(email, "-") {
		return "badEmail"
	}
	if len(password) > 1024 || strings.ContainsAny(password, "\r\n") {
		return "badPassword"
	}
	if len(mfa) > 10 || strings.IndexFunc(mfa, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return "badMfa"
	}
	return ""
}

// Login signs in. On success it starts a sync if a vault is already attached.
//
// accepted says the person ticked the third-party notice in the form. The sync
// client asks every user to accept that it is not an Obsidian product; the app
// shows that notice itself and only then passes --accept-disclaimer.
func (a *Account) Login(l Lang, email, password, mfa string, accepted bool) AccountResult {
	email, mfa = strings.TrimSpace(email), strings.TrimSpace(mfa)
	if !accepted {
		return AccountResult{Message: l.T("needAccept")}
	}
	if key := validLogin(email, password, mfa); key != "" {
		return AccountResult{Message: l.T(key)}
	}
	if !a.syncer.Online() {
		return AccountResult{Message: l.T("offline")}
	}
	args := []string{"login", "--accept-disclaimer", "--email", email}
	if mfa != "" {
		args = append(args, "--mfa", mfa) // one-time code, expires in seconds
	}
	var out string
	var err error
	if !a.syncer.Exclusive(func() { out, err = a.run(25*time.Second, password+"\n", args...) }) {
		return AccountResult{Message: l.T("busy")}
	}
	if err == nil && strings.Contains(out, "Login successful") {
		cur := a.RefreshVault()
		if cur.ID != "" {
			// Another account may have signed in: the vault that is here must be one of its own.
			if vaults, verr := a.Vaults(); verr == nil && !hasVault(vaults, cur.ID) {
				a.syncer.Exclusive(func() { _, _ = a.run(30*time.Second, "", "forget") })
				a.notesChanged()
				cur = a.Vault()
			}
		}
		if cur.ID == "" {
			return AccountResult{OK: true, Message: l.T("signedIn", email), Next: "vault"}
		}
		a.syncer.Start()
		return AccountResult{OK: true, Message: l.T("signedIn", email), Next: "sync"}
	}
	if strings.Contains(strings.ToLower(out), "2fa") && mfa == "" {
		return AccountResult{Need2FA: true, Message: l.T("need2fa")}
	}
	return AccountResult{Message: l.T("loginFailed", a.clientMessage(l, out, err))}
}

// Logout closes the session on this Kindle, forgets the vault and removes the
// downloaded notes from it.
func (a *Account) Logout(l Lang) AccountResult {
	var out string
	var err error
	if !a.syncer.Exclusive(func() { out, err = a.run(40*time.Second, "", "logout") }) {
		return AccountResult{Message: l.T("busy")}
	}
	a.notesChanged()
	if err != nil || !strings.Contains(out, "Logged out") {
		return AccountResult{Message: l.T("logoutFailed", a.clientMessage(l, out, err))}
	}
	return AccountResult{OK: true, Message: l.T("signedOut")}
}

func hasVault(vaults []RemoteVault, id string) bool {
	for _, v := range vaults {
		if v.ID == id {
			return true
		}
	}
	return false
}

// Setup attaches one of the account's vaults to this Kindle, read-only, and
// starts a sync. Changing vault removes the previous vault's notes first, and
// only once the new one is known to be reachable. password is only for vaults
// with their own end-to-end encryption password; without it such a vault is
// refused. Like the account password, it goes to the client on stdin.
func (a *Account) Setup(l Lang, id, password string) AccountResult {
	if !vaultIDRe.MatchString(id) || len(password) > 1024 || strings.ContainsAny(password, "\r\n") {
		return AccountResult{Message: l.T("invalidData")}
	}
	if !a.syncer.Online() {
		return AccountResult{Message: l.T("offline")}
	}
	// Only a vault the account really has: the id ends up as a client argument.
	vaults, err := a.Vaults()
	if err != nil {
		return AccountResult{Message: l.T("vaultsFailed", a.errText(l, err))}
	}
	name := ""
	for _, v := range vaults {
		if v.ID == id {
			name = v.Name
		}
	}
	if name == "" {
		return AccountResult{Message: l.T("badVault")}
	}
	var out string
	if !a.syncer.Exclusive(func() { out, err = a.run(90*time.Second, password+"\n", "setup", id) }) {
		return AccountResult{Message: l.T("busy")}
	}
	a.notesChanged()
	if err != nil {
		if strings.Contains(out, "needs its encryption password") {
			return AccountResult{NeedPassword: true, Message: l.T("needVaultPassword")}
		}
		if password != "" {
			return AccountResult{NeedPassword: true, Message: l.T("wrongVaultPassword", a.clientMessage(l, out, err))}
		}
		return AccountResult{Message: l.T("setupFailed", a.clientMessage(l, out, err))}
	}
	a.syncer.Start()
	return AccountResult{OK: true, Message: l.T("vaultReady", name), Next: "sync"}
}

func (a *Account) errText(l Lang, err error) string {
	if err == errTimeout {
		return l.T("timeout")
	}
	return err.Error()
}

func (a *Account) clientMessage(l Lang, out string, err error) string {
	if err == errTimeout {
		return l.T("timeout")
	}
	if msg := clientError(out, err); msg != "" {
		return msg
	}
	return l.T("unknownError")
}

// clientError keeps the client's last meaningful line, without prompts.
func clientError(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		for _, prefix := range []string{"Password:", "Vault password:", "login failed:", "Error:"} {
			l = strings.TrimSpace(strings.TrimPrefix(l, prefix))
		}
		if l != "" {
			return l
		}
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// authError reports whether a sync error looks like a missing or expired session.
func authError(msg string) bool {
	low := strings.ToLower(msg)
	for _, k := range []string{"logged in", "login", "unauthori", "auth", "token", "401"} {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}
