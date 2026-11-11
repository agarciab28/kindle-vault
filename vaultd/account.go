package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Account manages the Obsidian session on the Kindle from the app, through
// ob-kindle.sh (login / logout / whoami).
//
// The password only travels app -> vaultd (localhost, token-checked POST) ->
// the client's stdin. It is never written to disk, logged, or put on a command
// line; the client keeps only the session token Obsidian returns.
type Account struct {
	command []string // e.g. ["/bin/sh", "/mnt/us/ob-kindle.sh"]
	syncer  *Syncer
}

// AccountResult is what the app shows after login/logout.
type AccountResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Need2FA bool   `json:"need2fa,omitempty"`
}

var loggedInRe = regexp.MustCompile(`Logged in as: (\S+)`)

func (a *Account) run(timeout time.Duration, stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.command[0], append(append([]string{}, a.command[1:]...), args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), errors.New("Obsidian no respondió a tiempo")
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
	if err != nil && strings.Contains(err.Error(), "a tiempo") {
		return "", err
	}
	return "", nil
}

// validLogin checks the form before anything reaches the client.
func validLogin(email, password, mfa string) string {
	if email == "" || password == "" {
		return "Escribe tu correo y tu contraseña."
	}
	if len(email) > 254 || strings.IndexFunc(email, unicode.IsSpace) >= 0 || !strings.Contains(email, "@") {
		return "El correo no es válido."
	}
	if len(password) > 1024 || strings.ContainsAny(password, "\r\n") {
		return "La contraseña no es válida."
	}
	if len(mfa) > 10 || strings.IndexFunc(mfa, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return "El código de verificación solo lleva números."
	}
	return ""
}

// Login signs in. On success it starts a sync.
//
// accepted says the person ticked the third-party notice in the form. The sync
// client asks every user to accept that it is not an Obsidian product; the app
// shows that notice itself and only then passes --accept-disclaimer.
func (a *Account) Login(email, password, mfa string, accepted bool) AccountResult {
	email, mfa = strings.TrimSpace(email), strings.TrimSpace(mfa)
	if !accepted {
		return AccountResult{Message: "Para continuar, acepta el aviso: Bóveda no es una app oficial de Obsidian."}
	}
	if msg := validLogin(email, password, mfa); msg != "" {
		return AccountResult{Message: msg}
	}
	if !a.syncer.Online() {
		return AccountResult{Message: "Sin Wi-Fi. Conéctate y vuelve a intentarlo."}
	}
	args := []string{"login", "--accept-disclaimer", "--email", email}
	if mfa != "" {
		args = append(args, "--mfa", mfa) // one-time code, expires in seconds
	}
	var out string
	var err error
	if !a.syncer.Exclusive(func() { out, err = a.run(25*time.Second, password+"\n", args...) }) {
		return AccountResult{Message: "Hay una sincronización en curso. Espera unos segundos y vuelve a intentarlo."}
	}
	if err == nil && strings.Contains(out, "Login successful") {
		a.syncer.Start()
		return AccountResult{OK: true, Message: "Sesión iniciada como " + email + "."}
	}
	low := strings.ToLower(out)
	if strings.Contains(low, "2fa") && mfa == "" {
		return AccountResult{Need2FA: true, Message: "Tu cuenta pide el código de verificación. Escríbelo y vuelve a intentarlo."}
	}
	return AccountResult{Message: "No se pudo iniciar sesión: " + clientError(out, err)}
}

// Logout closes the session on this Kindle. Downloaded notes stay.
func (a *Account) Logout() AccountResult {
	var out string
	var err error
	if !a.syncer.Exclusive(func() { out, err = a.run(20*time.Second, "", "logout") }) {
		return AccountResult{Message: "Hay una sincronización en curso. Espera unos segundos y vuelve a intentarlo."}
	}
	if err != nil || !strings.Contains(out, "Logged out") {
		return AccountResult{Message: "No se pudo cerrar la sesión: " + clientError(out, err)}
	}
	return AccountResult{OK: true, Message: "Sesión cerrada en este Kindle. Las notas descargadas se quedan, pero ya no se actualizan."}
}

// clientError keeps the client's last meaningful line, without prompts.
func clientError(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		l = strings.TrimSpace(strings.TrimPrefix(l, "Password:"))
		l = strings.TrimSpace(strings.TrimPrefix(l, "login failed:"))
		if l != "" {
			return l
		}
	}
	if err != nil {
		return err.Error()
	}
	return "error desconocido"
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
