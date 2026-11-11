# Vault — read your Obsidian vault on a jailbroken Kindle

An **unofficial**, read-only reader for an Obsidian vault, as a native Kindle app
(a WAF, like the Kindle Store or Help). It syncs straight from Obsidian Sync and
never uploads or deletes anything.

- Folders, wikilinks, backlinks, tags, callouts and search, laid out for e-ink.
- Sign in and choose your vault **in the app**; no computer needed after copying it.
- The interface is in **English or Spanish** (English by default). Switch it in
  the settings: the gear at the top right, under the X. That is only the
  app's language: your notes are shown exactly as you wrote them, in whatever
  language they are.

> **Not an Obsidian product.** Vault is not affiliated with, endorsed by or
> supported by Obsidian. It connects through
> [obsidian-headless](https://github.com/Belphemur/obsidian-headless), a
> community-made client, with a small patch. If something breaks, do not contact
> Obsidian support.

**Status:** a personal project. It has been used on **one** device (Kindle
Paperwhite 12th gen, firmware 5.20.2, jailbroken) with **one** vault.

## Read this before you use it

- **Your notes sit unencrypted on the Kindle.** The synced vault (`/mnt/us/vault`)
  and the Obsidian session and vault key (`/mnt/us/ob-home`) live on the storage
  the Kindle exposes over USB, which has no file permissions. Anyone who has the
  unlocked Kindle can copy them. Set a device passcode, and if the Kindle is lost,
  sign that device out from your Obsidian account. Signing out in the app
  removes the notes and the vault key from the Kindle.
- **Read-only depends on this project's patch.** The client is patched so that
  the `mirror` mode can only download (see [Security](#security)), and the
  wrapper refuses to sync in any other mode. The patch has its own tests, run on
  every build, but it has been used against one vault. Keep your usual backups,
  and note that Obsidian Sync keeps a version history.
- **You type your Obsidian password into a third-party app.** It goes to
  Obsidian's servers through the client and is not stored; only the session
  token is. The release is built by GitHub Actions from the tagged source, with
  a checksum, and you can build it yourself.
- **Untested combinations:** other Kindle models and firmware, and vaults with
  their own end-to-end encryption password (supported, never tried on a real one).
- **Obsidian's terms.** Whether third-party Sync clients are allowed is between
  you and Obsidian; check their terms. The risk is on your account.
- **It runs as root.** Apps started from the Kindle library do. The local
  service listens on `127.0.0.1` only and is locked down (see below), but treat
  it as what it is.

No warranty: see the [license](LICENSE).

## Install

You need a **jailbroken Kindle** with the KindleModding hotfix, which lets `.sh`
files in the library run as apps (if KOReader or kTerm open from your library,
you have it), and an **Obsidian Sync** subscription.

1. Download `vault.zip` from the [latest release](https://github.com/agarciab28/kindle-vault/releases/latest)
   and unzip it. You get a file, `Vault.sh`, and a folder, `vault-app`.
2. Connect the Kindle by USB and copy **both** into its `documents` folder.
3. Eject the Kindle and open **Vault** from the library. The first time, the
   launcher installs the app; after that it just opens it.
4. In the app: **Account → Sign in** (you have to accept the third-party
   notice), then **Choose vault**. A vault with its own encryption password
   asks for it there.

After that the vault syncs when you open the app and about every 30 minutes
while it runs. The Kindle holds one vault at a time: **signing out, or choosing
another vault, removes the downloaded notes from the Kindle** (never from
Obsidian), so notes from one vault or account are never shown under another. If the session expires (for example, after a password change),
sign in again from **Account**.

**Update:** copy the new `Vault.sh` and `vault-app` over the old ones. Your notes,
session and settings are kept.

**Uninstall:** run this on the Kindle (kTerm or SSH). It keeps your downloaded
notes and your session; add `--all` to remove those too (sign out in the app first).

```sh
sh /mnt/us/documents/vault-app/uninstall
```

### Settings

Optional, in `/mnt/us/vault-data/vault.conf` on the Kindle (see
[`kindle/vault.conf.example`](kindle/vault.conf.example)): a time zone for the
dates the app shows, and the device name Obsidian lists.

### What it does to your Kindle

- `documents/Vault.sh` and `documents/vault-app/`: the program (what you copied).
- `/var/local/mesquite/vault/`: the app's interface, where the Kindle keeps its
  own web apps, plus the app's own rows in the app registry (`/var/local/appreg.db`).
- `/mnt/us/vault/`: your notes. `/mnt/us/ob-home/`: the Obsidian session.
  `/mnt/us/vault-data/`: sync state, a log and the optional settings.

## How it works

```
Obsidian Sync ──(ob-kindle sync, mirror mode)──▶ /mnt/us/vault/*.md
                                                     │
                                  vaultd (Go, 127.0.0.1:8317): index, Markdown → HTML,
                                  wikilinks, callouts, backlinks, tags, search, auto-sync
                                                     │ XHR (local-port-access)
                        WAF com.alexgar.vault (/var/local/mesquite/vault): the UI
                                                     ▲
                       library "book" Vault.sh: starts vaultd, opens the WAF
```

## Build it yourself

You need Go and git.

```sh
client/build.sh    # clones obsidian-headless at a pinned commit, applies the patch, runs its tests, builds it for the Kindle
./package.sh       # tests and builds vaultd, then makes build/vault.zip
./deploy.sh <ip>   # optional: copies the package over Wi-Fi (KOReader's SSH server, port 2222, key auth)
```

Pushing a tag `v…` makes GitHub Actions (`.github/workflows/build.yml`) do the
same and publish the release with its checksum.

| Path | What |
|---|---|
| `client/` | The patch for [obsidian-headless (Go)](https://github.com/Belphemur/obsidian-headless) v0.3.6 and `build.sh`. The patch makes `pull`/`mirror` really read-only, supports legacy v0-encrypted vaults, fixes v0 path decryption, reads a piped password as a whole line (spaces included), refuses an encrypted vault without its password, and adds tests for the read-only guarantee. |
| `vaultd/` | The local service (Go, no CGO): index, Markdown, search, sync, account. All interface text is in `i18n.go`, one block per language. `go test ./...` runs its tests. |
| `waf/` | The app shell: `config.xml`, `index.html`, `app.js` (ES5), `app.css` (e-ink). |
| `kindle/Vault.sh` | The launcher: installs on first run, starts `vaultd`, opens the app. |
| `kindle/ob` | Wrapper around the sync client: `login`, `logout`, `whoami`, `vaults`, `current`, `setup`, `forget`, `sync`, `status`. `sync` refuses to run unless the mode is `mirror`; `logout`, `forget` and a change of vault remove the downloaded notes. Tested in `vaultd/wrapper_test.go` with a fake client. No `.sh` extension on purpose: the Kindle lists every `.sh` under `documents` as a book. |
| `kindle/uninstall`, `vault.conf.example` | Removal, and the optional settings. |
| `kindle/cover.svg`, `cover.png` | The library cover. `package.sh` embeds the PNG in the launcher as its `# Icon:` line. |

## Security

- **Read-only sync.** The patched client never pushes or deletes remotely in `pull`/`mirror` mode (plan rewrite + the session refuses `push`/`delete`). The wrapper (`kindle/ob`) refuses to sync unless the mode is `mirror`, and `client/build.sh` runs the patch's tests on every build.
- **Local API is private.** `vaultd` listens on `127.0.0.1` only and every request needs:
  - a random **token** generated at each start and written to `/var/local/mesquite/vault/token.js` (only the WAF loads it; web pages cannot read local files);
  - an **Origin** of the WAF (`null`/`file://`) or none, so pages in the Kindle browser are rejected;
  - a loopback **Host**, against DNS rebinding.
- **Login and vault choice from the app.** The only state-changing endpoints, `/api/login`, `/api/logout` and `/api/setup`, accept POST only (everything else is GET only), with the same token/origin/host checks and a 4 KB body limit. Passwords (the account's, or a vault's own) travel in the POST body, never in a URL; `vaultd` hands them to the client on stdin (never on a command line) and logs only `ok=true/false`. The client stores the session token and the vault key, not the account password. A vault can only be chosen from the list the account really has. Login/logout never run during a sync. The client's third-party notice is shown in the form and must be accepted there.
- **No raw HTML from notes.** goldmark runs without `WithUnsafe`: `<script>`, `<iframe>`… in a note are dropped and `javascript:` links are not rendered.
- **Pinned upstream.** `client/build.sh` refuses to build unless the `v0.3.6` tag still points to the reviewed commit. The GitHub Actions used for releases are pinned by commit too.
- **On the device:** the Obsidian session and vault key live in `/mnt/us/ob-home` (encrypted with a key stored on the same device; `/mnt/us` has no Unix permissions). KOReader's SSH server gives `root` to the authorized key; stop it when not needed.
- **Never commit** `token.js`, `state.json`, logs, `appreg.db*`, `vault.conf` or anything from `ob-home` (see `.gitignore`).

Found a security problem? Please open an issue, or a private security advisory on GitHub if it is sensitive.

## Kindle notes (learned the hard way)

- WAFs run as `/usr/bin/mesquite -l <id> -c file:///var/local/mesquite/<dir>/`, registered in `/var/local/appreg.db` (`handlerIds`, `properties.command`, `associations application/none`). The launcher only inserts or replaces the app's own rows.
- `local-port-access` permission lets the WAF call `http://127.0.0.1`.
- **Title-bar back arrow** (new "Decanter" chrome): KPPChrome sets the LIPC property `invokeAction = KPP_BACK` on the app. `kindle.messaging.receiveMessage('invokeAction', cb)` publishes that property; without it the arrow silently fails (`LIPC access error status=8`).
- **No app button next to the X.** In the `title` template, `{ id: 'KPP_MORE', handling: 'notifyApp' }` does add the system's three dots, but a tap opens the Kindle's own menu instead of notifying the app (and a custom icon through `uri` is ignored). So the settings gear lives in the page, at the top right (`waf/gear.svg`, embedded in `app.css`).
- `switchViewMode=fullscreen` does **not** hide the system bar.
- KOReader stops the library indexer while it runs, and usually hides other apps; close it to see Vault or a new cover.
- Obsidian's config dir on Linux is `~/.config/obsidian-headless/` (macOS: `~/.obsidian-headless/`); `sync-setup` resets the sync mode, so `ob setup` sets `mirror` again before any sync.
- Library cover: the launcher's `# Icon: data:image/png;base64,…` header (600×900). To change it, edit `kindle/cover.svg`, render it to `cover.png` at 600×900 (e.g. headless Chrome `--screenshot --window-size=600,900`) and package again.

## License

[GPL-3.0](LICENSE), the same license as obsidian-headless, which the client patch derives from.
