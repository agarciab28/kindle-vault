# Bóveda — read your Obsidian vault on a jailbroken Kindle

An **unofficial**, read-only reader for an Obsidian vault, as a native Kindle app
(a WAF, like the Kindle Store or Help). It syncs straight from Obsidian Sync and
never uploads or deletes anything. The interface is in Spanish.

> **Not an Obsidian product.** Bóveda is not affiliated with, endorsed by or
> supported by Obsidian. It connects through
> [obsidian-headless](https://github.com/Belphemur/obsidian-headless), a
> community-made client, with a small patch. If something breaks, do not contact
> Obsidian support.

**Status:** a personal project, published as source for people comfortable with
a terminal. It has been used on **one** device (Kindle Paperwhite 12th gen,
firmware 5.20.2, jailbroken) with **one** vault. There are no prebuilt binaries
and no one-tap installer yet: you build it and install it over SSH.

## Read this before you use it

- **Your notes sit unencrypted on the Kindle.** The synced vault (`/mnt/us/vault`)
  and the Obsidian session and vault key (`/mnt/us/ob-home`) live on the storage
  the Kindle exposes over USB, which has no file permissions. Anyone who has the
  unlocked Kindle can copy them. Set a device passcode, and if the Kindle is lost,
  sign that device out from your Obsidian account.
- **Read-only depends on this project's patch.** The client is patched so that
  the `mirror` mode can only download (see [Security](#security)), and the
  wrapper refuses to sync in any other mode. It has been checked against one
  vault; there is no automated test of it yet. Keep your usual backups, and note
  that Obsidian Sync keeps a version history.
- **You type your Obsidian password into a third-party app.** It goes to
  Obsidian's servers through the client and is not stored; only the session
  token is. Read the code and build it yourself: that is the point of publishing it.
- **Untested combinations:** other Kindle models and firmware, and vaults with
  their own end-to-end encryption password.
- **Obsidian's terms.** Whether third-party Sync clients are allowed is between
  you and Obsidian; check their terms. The risk is on your account.
- **It runs as root.** Apps started from the Kindle library do. The local
  service listens on `127.0.0.1` only and is locked down (see below), but treat
  it as what it is.

No warranty: see the [license](LICENSE).

## How it works

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

## Requirements

- An **Obsidian Sync** subscription.
- A **jailbroken Kindle** with the KindleModding hotfix (it lets `.sh` files in
  the library run as apps) and **KOReader**, for its SSH server.
- A computer with **Go**, **git** and **ssh**.

## Install

1. Build the sync client: `client/build.sh`. It clones obsidian-headless at a
   pinned commit, applies the patch and cross-compiles it for the Kindle.
2. On the Kindle, start KOReader's SSH server (port 2222) with your public key
   authorized. Prefer key login over the passwordless option.
3. `./deploy.sh <kindle-ip>` builds `vaultd` and copies everything, including a
   CA bundle for the client (the Kindle only ships Amazon's).
4. Close KOReader, open **Bóveda** from the library, go to **Cuenta** and sign in.
   The form shows the third-party notice; you have to accept it.
5. Choose the vault, once, over SSH:

   ```sh
   ssh -p 2222 root@<kindle-ip>
   sh /mnt/us/ob-kindle.sh vaults                       # lists your vaults
   VAULT_NAME="My notes" sh /mnt/us/ob-kindle.sh setup  # attaches it, in mirror (read-only) mode
   ```

   For a vault with its own encryption password, add `--password '…'` after `setup`.
6. Back in the app, tap **Sincronizar ahora**. After that it syncs on its own
   about every 30 minutes.

If the session expires later (for example, after a password change), sign in
again from **Cuenta**. No SSH needed.

### Settings

Optional, in `/mnt/us/vault-app/boveda.conf` on the Kindle (see
[`kindle/boveda.conf.example`](kindle/boveda.conf.example)): the vault name, a
time zone for the dates the app shows, and the device name Obsidian lists.

### Uninstall

`ssh -p 2222 root@<kindle-ip> 'sh /mnt/us/vault-app/uninstall.sh'` removes the app.
It keeps the synced notes (`/mnt/us/vault`) and the Obsidian session
(`/mnt/us/ob-home`); sign out from **Cuenta** first, or delete those two folders.

## Layout

| Path | What |
|---|---|
| `client/` | The patch for [obsidian-headless (Go)](https://github.com/Belphemur/obsidian-headless) v0.3.6 and `build.sh` (ARMv7 + macOS). The patch makes `pull`/`mirror` really read-only, supports legacy v0-encrypted vaults, fixes v0 path decryption and reads a piped password as a whole line (spaces included). |
| `vaultd/` | The local service (Go, no CGO). `go test ./...` runs its tests; `deploy.sh` cross-compiles it for the Kindle. |
| `waf/` | The app UI: `config.xml`, `index.html`, `app.js` (ES5), `app.css` (e-ink). |
| `kindle/` | `Boveda.sh` (library launcher), `install.sh` / `uninstall.sh`, `ob-kindle.sh` (client wrapper: `login`, `logout`, `whoami`, `vaults`, `setup`, `sync`, `status`; `sync` refuses to run unless the mode is `mirror`), `boveda.conf.example`. |
| `kindle/cover.svg`, `cover.png` | The library cover. `deploy.sh` embeds the PNG in the launcher as its `# Icon:` header line. |
| `deploy.sh` | Builds `vaultd` and installs everything over Wi-Fi (KOReader's SSH server, port 2222, key auth). Also copies `build/ob-kindle` when it changed. |

## Security

- **Read-only sync.** The patched client never pushes or deletes remotely in `pull`/`mirror` mode (plan rewrite + the session refuses `push`/`delete`). `ob-kindle.sh` refuses to sync unless the mode is `mirror`.
- **Local API is private.** `vaultd` listens on `127.0.0.1` only and every request needs:
  - a random **token** generated at each start and written to `/var/local/mesquite/vault/token.js` (only the WAF loads it; web pages cannot read local files);
  - an **Origin** of the WAF (`null`/`file://`) or none, so pages in the Kindle browser are rejected;
  - a loopback **Host**, against DNS rebinding.
- **Login from the app.** The only state-changing endpoints, `/api/login` and `/api/logout`, accept POST only (everything else is GET only), with the same token/origin/host checks and a 4 KB body limit. The password travels in the POST body, never in a URL; `vaultd` hands it to the client on stdin (never on a command line), logs only `ok=true/false`, and the client stores only the session token. Login/logout never run during a sync. The client's third-party notice is shown in the form and must be accepted there.
- **No raw HTML from notes.** goldmark runs without `WithUnsafe`: `<script>`, `<iframe>`… in a note are dropped and `javascript:` links are not rendered.
- **Pinned upstream.** `client/build.sh` refuses to build unless the `v0.3.6` tag still points to the reviewed commit.
- **On the device:** the Obsidian session and vault key live in `/mnt/us/ob-home` (encrypted with a key stored on the same device; `/mnt/us` has no Unix permissions). KOReader's SSH server gives `root` to the authorized key; stop it when not needed.
- **Never commit** `token.js`, `state.json`, logs, `appreg.db*`, `boveda.conf` or anything from `ob-home` (see `.gitignore`).

Found a security problem? Please open an issue, or a private security advisory on GitHub if it is sensitive.

## Kindle notes (learned the hard way)

- WAFs run as `/usr/bin/mesquite -l <id> -c file:///var/local/mesquite/<dir>/`, registered in `/var/local/appreg.db` (`handlerIds`, `properties.command`, `associations application/none`). `install.sh` backs the DB up to `/mnt/us/vault-app/appreg.db.orig` once.
- `local-port-access` permission lets the WAF call `http://127.0.0.1`.
- **Title-bar back arrow** (new "Decanter" chrome): KPPChrome sets the LIPC property `invokeAction = KPP_BACK` on the app. `kindle.messaging.receiveMessage('invokeAction', cb)` publishes that property; without it the arrow silently fails (`LIPC access error status=8`).
- `switchViewMode=fullscreen` does **not** hide the system bar.
- KOReader stops the library indexer while it runs, and usually hides other apps; close it to see Bóveda or a new cover.
- Obsidian's config dir on Linux is `~/.config/obsidian-headless/` (macOS: `~/.obsidian-headless/`); `sync-setup` resets the sync mode, so `ob-kindle.sh setup` sets `mirror` again before any sync.
- Library cover: the launcher's `# Icon: data:image/png;base64,…` header (600×900). To change it, edit `kindle/cover.svg`, render it to `cover.png` at 600×900 (e.g. headless Chrome `--screenshot --window-size=600,900`) and deploy.

## License

[GPL-3.0](LICENSE), the same license as obsidian-headless, which the client patch derives from.
