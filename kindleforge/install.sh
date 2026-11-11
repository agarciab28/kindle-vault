#!/bin/sh
# KindleForge package: downloads the latest release, checks that the download
# is complete, and puts it in "documents". Installing over an older version
# keeps the notes, the session and the settings.
set -e

TMPDIR=/mnt/us/KFPM-Temporary
BASE=https://github.com/agarciab28/kindle-vault/releases/latest/download
rm -rf "$TMPDIR"
mkdir -p "$TMPDIR"

curl -fSL --progress-bar -o "$TMPDIR/vault.zip" "$BASE/vault.zip"
curl -fsSL -o "$TMPDIR/vault.zip.sha256" "$BASE/vault.zip.sha256"
if ! (cd "$TMPDIR" && sha256sum -c vault.zip.sha256 >/dev/null); then
  echo "vault.zip does not match its checksum; nothing was installed."
  rm -rf "$TMPDIR"
  exit 1
fi
unzip -oq "$TMPDIR/vault.zip" -d "$TMPDIR"

# The service must not be running while its program is replaced; the launcher starts it again.
killall vaultd 2>/dev/null || true
mkdir -p /mnt/us/documents/vault-app
cp -r "$TMPDIR/vault-app"/* /mnt/us/documents/vault-app/
cp "$TMPDIR/Vault.sh" /mnt/us/documents/
chmod +x /mnt/us/documents/Vault.sh /mnt/us/documents/vault-app/vaultd /mnt/us/documents/vault-app/ob-kindle

rm -rf "$TMPDIR"

exit 0
