# Bóveda — Obsidian vault on a jailbroken Kindle

A native Kindle app (WAF, like the Kindle Store or Help) that shows an Obsidian
vault synced directly from Obsidian Sync, **read-only**.

Tested on a Kindle Paperwhite 12th gen (PW6), firmware 5.20.2, jailbroken with Véra.

```
Obsidian Sync ──(ob-kindle sync, mirror mode)──▶ /mnt/us/vault/*.md
                                                     │
                                  vaultd (Go, 127.0.0.1:8317): index, Markdown → HTML,
                                  wikilinks, callouts, backlinks, tags, search, auto-sync
                                                     │ XHR (local-port-access)
                        WAF com.alexgar.vault (/var/local/mesquite/vault): the UI
                                                     ▲
                       library "book" Boveda.sh: starts vaultd, opens the WAF
```

## Layout

| Path | What |
|---|---|
| `client/` | Patch for [obsidian-headless (Go)](https://github.com/Belphemur/obsidian-headless) v0.3.6 and `build.sh` (ARMv7 + macOS). The patch makes `pull`/`mirror` really read-only, supports legacy v0-encrypted vaults, fixes v0 path decryption and reads a piped password as a whole line (spaces included). |
| `vaultd/` | The local service (Go, no CGO). `go build` for the Mac; `deploy.sh` cross-compiles for the Kindle. |
| `waf/` | The app UI: `config.xml`, `index.html`, `app.js` (ES5), `app.css` (e-ink). |
| `kindle/` | `Boveda.sh` (library launcher), `install.sh` / `uninstall.sh`, `ob-kindle.sh` (client wrapper: `login`, `logout`, `whoami`, `setup`, `sync`; `sync` refuses to run unless the mode is `mirror`). |
| `deploy.sh` | Builds `vaultd` and installs everything over Wi-Fi (KOReader's SSH server, port 2222, key auth). Also copies `build/ob-kindle` when it changed. |

## First-time setup

1. `client/build.sh` → copy `build/ob-kindle` to `/mnt/us/ob-kindle` and a CA bundle to `/mnt/us/ob-cacert.pem` (the Kindle only ships Amazon CAs).
2. On the Kindle (SSH): `sh /mnt/us/ob-kindle.sh login`, then `setup`, then `sync`.
3. On the Mac: `./deploy.sh <kindle-ip>`.
4. Open **Bóveda** from the Kindle library.

Later, if the Obsidian session expires (e.g. after a password change), sign in again from the app: **Cuenta → Iniciar sesión**. No SSH needed.

## Kindle notes (learned the hard way)

- WAFs run as `/usr/bin/mesquite -l <id> -c file:///var/local/mesquite/<dir>/`, registered in `/var/local/appreg.db` (`handlerIds`, `properties.command`, `associations application/none`). `install.sh` backs the DB up to `/mnt/us/vault-app/appreg.db.orig` once.
- `local-port-access` permission lets the WAF call `http://127.0.0.1`.
- **Title-bar back arrow** (new "Decanter" chrome): KPPChrome sets the LIPC property `invokeAction = KPP_BACK` on the app. `kindle.messaging.receiveMessage('invokeAction', cb)` publishes that property; without it the arrow silently fails (`LIPC access error status=8`).
- `switchViewMode=fullscreen` does **not** hide the system bar.
- KOReader in the foreground hides the WAF; close it to see the app.
- Obsidian's config dir on Linux is `~/.config/obsidian-headless/` (macOS: `~/.obsidian-headless/`); `sync-setup` resets the sync mode, so set `mirror` again before syncing.

## Security

- **Read-only sync.** The patched client never pushes or deletes remotely in `pull`/`mirror` mode (plan rewrite + the session refuses `push`/`delete`). `ob-kindle.sh` refuses to sync unless the mode is `mirror`.
- **Local API is private.** `vaultd` listens on `127.0.0.1` only and every request needs:
  - a random **token** generated at each start and written to `/var/local/mesquite/vault/token.js` (only the WAF loads it; web pages cannot read local files);
  - an **Origin** of the WAF (`null`/`file://`) or none, so pages in the Kindle browser are rejected;
  - a loopback **Host**, against DNS rebinding.
- **Login from the app.** The only state-changing endpoints, `/api/login` and `/api/logout`, accept POST only (everything else is GET only), with the same token/origin/host checks and a 4 KB body limit. The password travels in the POST body, never in a URL; `vaultd` hands it to the client on stdin (never on a command line), logs only `ok=true/false`, and the client stores only the session token. Login/logout never run during a sync.
- **No raw HTML from notes.** goldmark runs without `WithUnsafe`: `<script>`, `<iframe>`… in a note are dropped and `javascript:` links are not rendered.
- **Pinned upstream.** `client/build.sh` refuses to build unless the `v0.3.6` tag still points to the reviewed commit.
- **On the device:** the Obsidian session and vault key live in `/mnt/us/ob-home` (encrypted with a key stored on the same device; `/mnt/us` has no Unix permissions). If the Kindle is lost, sign that device out from your Obsidian account. KOReader's SSH server gives `root` to the authorized key; stop it when not needed.
- **Never commit** `token.js`, `state.json`, logs, `appreg.db*` or anything from `ob-home` (see `.gitignore`).

## Uninstall

`ssh … 'sh /mnt/us/vault-app/uninstall.sh'` (keeps `/mnt/us/vault` and the Obsidian session in `/mnt/us/ob-home`).
