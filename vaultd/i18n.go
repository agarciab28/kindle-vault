package main

import (
	"fmt"
	"strings"
	"time"
)

// Lang is the language of the app's own interface. It never touches the notes:
// their text, titles, folders and tags are shown exactly as written.
type Lang string

const (
	EN Lang = "en" // the default
	ES Lang = "es"
)

func parseLang(s string) Lang {
	if s == string(ES) {
		return ES
	}
	return EN
}

// T returns the interface text for key, formatted with args.
func (l Lang) T(key string, args ...any) string {
	s, ok := text[l][key]
	if !ok {
		if s, ok = text[EN][key]; !ok {
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// N returns a counted phrase: key+".one" for 1, key+".other" otherwise.
func (l Lang) N(key string, n int) string {
	if n == 1 {
		return l.T(key + ".one")
	}
	return l.T(key+".other", n)
}

// Date formats a time like "10 Nov 2026, 20:04".
func (l Lang) Date(t time.Time) string {
	if t.IsZero() {
		return l.T("never")
	}
	t = t.Local()
	months := strings.Split(l.T("months"), ",")
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), months[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// Ago formats how long ago t was: "4 h ago".
func (l Lang) Ago(t time.Time) string {
	if t.IsZero() {
		return l.T("never")
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return l.T("justNow")
	case d < time.Hour:
		return l.T("minAgo", int(d.Minutes()))
	case d < 24*time.Hour:
		return l.T("hAgo", int(d.Hours()))
	default:
		return l.N("dayAgo", int(d.Hours()/24))
	}
}

var text = map[Lang]map[string]string{
	EN: {
		"lang.en": "English", "lang.es": "Español", "language": "Language",
		"settings": "Settings", "languageNote": "Only the app changes language. Your notes stay exactly as they are written.",
		"months": "Jan,Feb,Mar,Apr,May,Jun,Jul,Aug,Sep,Oct,Nov,Dec",
		"never":  "never", "justNow": "just now", "minAgo": "%d min ago", "hAgo": "%d h ago",
		"dayAgo.one": "1 day ago", "dayAgo.other": "%d days ago",

		// Sync box
		"syncing": "Syncing…", "syncFailed": "The last sync failed", "lastGoodSync": "Last successful sync: %s",
		"synced":           "Synced",
		"filesUpdated.one": "1 file updated in the last sync", "filesUpdated.other": "%d files updated in the last sync",
		"sessionExpired": "Your Obsidian session seems to have expired.",
		"signIn":         "Sign in", "retry": "Retry", "syncNow": "Sync now", "account": "Account",
		"noVault":     "No vault chosen yet.",
		"noVaultHint": "Sign in to Obsidian and choose which vault to read on this Kindle.",
		"chooseVault": "Choose vault", "changeVault": "Change vault",

		// Lists and notes
		"folders": "Folders", "recent": "Recently changed", "root": "root",
		"note.one": "1 note", "note.other": "%d notes", "seeTags": "see tags", "allTags": "all tags",
		"cannotShow": "cannot be shown", "emptyFolder": "Empty folder", "linkedFrom": "Linked from (%d)",
		"taggedWith": "%s tagged", "tags": "Tags",
		"search": "Search", "searchHint": "Search the vault", "result.one": "1 result", "result.other": "%d results",
		"notFound": "Not found", "notFoundBody": "That note or folder does not exist.", "backHome": "Back to home",

		// Account
		"signInTitle": "Sign in to Obsidian", "email": "Email", "password": "Password",
		"mfa": "Verification code", "mfaHint": "(only if you have it turned on)",
		"notice":     "Vault is not an official Obsidian app.",
		"noticeBody": "It connects through obsidian-headless, a community-made client that Obsidian does not support. If something breaks, do not contact Obsidian support.",
		"acceptTap":  "Tap here to accept", "accepted": "Accepted",
		"passwordNote": "Your password goes straight to Obsidian: this Kindle keeps only the session, never the password. If there was already a session on this Kindle, it is replaced.",
		"signedInAs":   "Signed in as", "readOnlyNote": "This Kindle downloads the vault read-only: it never uploads or deletes anything.",
		"signInAgain": "Sign in again", "signOut": "Sign out on this Kindle",
		"noSession":     "There is no active session, or it has expired.",
		"noSessionHint": "Sign in so the vault keeps updating. The notes already downloaded stay here until you sign out.",
		"vaultHere":     "Vault on this Kindle:",
		"noWifiSession": "No Wi-Fi: the session cannot be checked right now.",
		"noWifi":        "No Wi-Fi", "invalidData": "Invalid data.",

		// Account messages
		"timeout":     "Obsidian did not answer in time",
		"needCreds":   "Enter your email and your password.",
		"badEmail":    "That email does not look valid.",
		"badPassword": "That password is not valid.",
		"badMfa":      "The verification code is digits only.",
		"needAccept":  "To continue, accept the notice: Vault is not an official Obsidian app.",
		"offline":     "No Wi-Fi. Connect and try again.",
		"busy":        "A sync is running. Wait a few seconds and try again.",
		"signedIn":    "Signed in as %s.",
		"need2fa":     "Your account asks for the verification code. Enter it and try again.",
		"loginFailed": "Could not sign in: %s", "logoutFailed": "Could not sign out: %s",
		"signedOut":    "Signed out. The downloaded notes were removed from this Kindle; nothing changed in Obsidian.",
		"unknownError": "unknown error",

		// Vault
		"vault": "Vault", "yourVaults": "Your vaults", "current": "current",
		"vaultHint":     "Choosing a vault downloads it to this Kindle, read-only. Nothing is ever changed in Obsidian. This Kindle holds one vault at a time.",
		"vaultPassword": "Vault password", "vaultPasswordHint": "Only for vaults with end-to-end encryption and a password of their own. Otherwise leave it empty.",
		"noVaults":      "This account has no vaults in Obsidian Sync.",
		"vaultsFailed":  "Could not get your vaults: %s",
		"changeConfirm": "The notes of the vault that is on this Kindle now will be removed from the Kindle (never from Obsidian), and the other vault will be downloaded.",
		"changeYes":     "Yes, change vault", "settingUp": "Setting up the vault…",
		"badVault":   "That vault is not in your account.",
		"vaultReady": "“%s” is ready.", "setupFailed": "Could not set up the vault: %s",
		"needVaultPassword":  "This vault has its own encryption password. Type it in the field below and choose the vault again.",
		"wrongVaultPassword": "Could not open the vault with that password (%s). Check it and choose the vault again.",

		// Callouts without a title of their own
		"callout.note": "Note", "callout.info": "Info", "callout.tip": "Tip", "callout.important": "Important",
		"callout.warning": "Warning", "callout.danger": "Danger", "callout.error": "Error", "callout.bug": "Bug",
		"callout.example": "Example", "callout.quote": "Quote", "callout.question": "Question",
		"callout.success": "Done", "callout.failure": "Failed", "callout.missing": "Missing",
		"callout.abstract": "Summary", "callout.todo": "To do",
	},
	ES: {
		"lang.en": "English", "lang.es": "Español", "language": "Idioma",
		"settings": "Ajustes", "languageNote": "Solo cambia el idioma de la app. Tus notas se quedan tal como están escritas.",
		"months": "ene,feb,mar,abr,may,jun,jul,ago,sep,oct,nov,dic",
		"never":  "nunca", "justNow": "hace un momento", "minAgo": "hace %d min", "hAgo": "hace %d h",
		"dayAgo.one": "hace 1 día", "dayAgo.other": "hace %d días",

		"syncing": "Sincronizando…", "syncFailed": "Falló la última sincronización", "lastGoodSync": "Última sincronización correcta: %s",
		"synced":           "Sincronizada",
		"filesUpdated.one": "1 archivo actualizado en la última sincronización", "filesUpdated.other": "%d archivos actualizados en la última sincronización",
		"sessionExpired": "Parece que la sesión de Obsidian venció.",
		"signIn":         "Iniciar sesión", "retry": "Reintentar", "syncNow": "Sincronizar ahora", "account": "Cuenta",
		"noVault":     "Todavía no has elegido una bóveda.",
		"noVaultHint": "Inicia sesión en Obsidian y elige qué bóveda leer en este Kindle.",
		"chooseVault": "Elegir bóveda", "changeVault": "Cambiar de bóveda",

		"folders": "Carpetas", "recent": "Cambiadas recientemente", "root": "raíz",
		"note.one": "1 nota", "note.other": "%d notas", "seeTags": "ver tags", "allTags": "todos los tags",
		"cannotShow": "no se puede mostrar", "emptyFolder": "Carpeta vacía", "linkedFrom": "Enlazado desde (%d)",
		"taggedWith": "%s con", "tags": "Tags",
		"search": "Buscar", "searchHint": "Buscar en la bóveda", "result.one": "1 resultado", "result.other": "%d resultados",
		"notFound": "No encontrado", "notFoundBody": "No existe esa nota o carpeta.", "backHome": "Volver al inicio",

		"signInTitle": "Iniciar sesión en Obsidian", "email": "Correo", "password": "Contraseña",
		"mfa": "Código de verificación", "mfaHint": "(solo si lo tienes activado)",
		"notice":     "Vault no es una app oficial de Obsidian.",
		"noticeBody": "Se conecta con obsidian-headless, un cliente hecho por la comunidad que Obsidian no respalda. Si algo falla, no escribas al soporte de Obsidian.",
		"acceptTap":  "Toca aquí para aceptar", "accepted": "Aceptado",
		"passwordNote": "La contraseña va directo a Obsidian: este Kindle solo guarda la sesión, nunca la contraseña. Si ya había una sesión en este Kindle, se reemplaza.",
		"signedInAs":   "Sesión iniciada como", "readOnlyNote": "Este Kindle descarga la bóveda en solo lectura: nunca sube ni borra nada.",
		"signInAgain": "Iniciar sesión de nuevo", "signOut": "Cerrar sesión en este Kindle",
		"noSession":     "No hay una sesión activa, o ya venció.",
		"noSessionHint": "Inicia sesión para que la bóveda se siga actualizando. Las notas descargadas siguen aquí hasta que cierres sesión.",
		"vaultHere":     "Bóveda en este Kindle:",
		"noWifiSession": "Sin Wi-Fi: no se puede revisar la sesión ahora.",
		"noWifi":        "Sin Wi-Fi", "invalidData": "Datos inválidos.",

		"timeout":     "Obsidian no respondió a tiempo",
		"needCreds":   "Escribe tu correo y tu contraseña.",
		"badEmail":    "El correo no es válido.",
		"badPassword": "La contraseña no es válida.",
		"badMfa":      "El código de verificación solo lleva números.",
		"needAccept":  "Para continuar, acepta el aviso: Vault no es una app oficial de Obsidian.",
		"offline":     "Sin Wi-Fi. Conéctate y vuelve a intentarlo.",
		"busy":        "Hay una sincronización en curso. Espera unos segundos y vuelve a intentarlo.",
		"signedIn":    "Sesión iniciada como %s.",
		"need2fa":     "Tu cuenta pide el código de verificación. Escríbelo y vuelve a intentarlo.",
		"loginFailed": "No se pudo iniciar sesión: %s", "logoutFailed": "No se pudo cerrar la sesión: %s",
		"signedOut":    "Sesión cerrada. Las notas descargadas se quitaron de este Kindle; en Obsidian no cambió nada.",
		"unknownError": "error desconocido",

		"vault": "Bóveda", "yourVaults": "Tus bóvedas", "current": "actual",
		"vaultHint":     "Al elegir una bóveda, se descarga a este Kindle en solo lectura. Nunca se cambia nada en Obsidian. Este Kindle guarda una sola bóveda a la vez.",
		"vaultPassword": "Contraseña de la bóveda", "vaultPasswordHint": "Solo para bóvedas con cifrado de extremo a extremo y contraseña propia. Si no, déjala vacía.",
		"noVaults":      "Esta cuenta no tiene bóvedas en Obsidian Sync.",
		"vaultsFailed":  "No se pudieron obtener tus bóvedas: %s",
		"changeConfirm": "Las notas de la bóveda que está ahora en este Kindle se quitarán del Kindle (nunca de Obsidian) y se descargará la otra bóveda.",
		"changeYes":     "Sí, cambiar de bóveda", "settingUp": "Preparando la bóveda…",
		"badVault":   "Esa bóveda no está en tu cuenta.",
		"vaultReady": "«%s» está lista.", "setupFailed": "No se pudo preparar la bóveda: %s",
		"needVaultPassword":  "Esta bóveda tiene su propia contraseña de cifrado. Escríbela en el campo de abajo y vuelve a elegir la bóveda.",
		"wrongVaultPassword": "No se pudo abrir la bóveda con esa contraseña (%s). Revísala y vuelve a elegir la bóveda.",

		"callout.note": "Nota", "callout.info": "Info", "callout.tip": "Consejo", "callout.important": "Importante",
		"callout.warning": "Atención", "callout.danger": "Peligro", "callout.error": "Error", "callout.bug": "Bug",
		"callout.example": "Ejemplo", "callout.quote": "Cita", "callout.question": "Pregunta",
		"callout.success": "Hecho", "callout.failure": "Falló", "callout.missing": "Falta",
		"callout.abstract": "Resumen", "callout.todo": "Pendiente",
	},
}
