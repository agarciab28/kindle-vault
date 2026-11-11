#!/bin/sh
# For development: builds the package and copies it to the Kindle over Wi-Fi
# (KOReader's SSH server, port 2222, key auth). Users do not need this: see README.
# Usage: ./deploy.sh <kindle-ip>      (KEY=~/.ssh/other_key ./deploy.sh … for another key)
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
IP="${1:?usage: ./deploy.sh KINDLE_IP}"
KEY="${KEY:-$HOME/.ssh/id_ed25519}"
PKG="$HERE/build/pkg"

"$HERE/package.sh"

k() { ssh -p 2222 -o BatchMode=yes -o ConnectTimeout=10 -o IdentitiesOnly=yes -i "$KEY" "root@$IP" "$@"; }
put() { k "cat > '$2.new' && mv '$2.new' '$2'" < "$1"; }
sum() { if command -v md5sum >/dev/null 2>&1; then md5sum "$1" | cut -d' ' -f1; else md5 -q "$1"; fi; }

k "mkdir -p /mnt/us/documents/vault-app"
for f in "$PKG"/vault-app/*; do
  name=$(basename "$f")
  # The binaries are large: copy them only when they changed.
  if [ "$(k "md5sum '/mnt/us/documents/vault-app/$name' 2>/dev/null | cut -d' ' -f1")" != "$(sum "$f")" ]; then
    echo "copying $name…"
    put "$f" "/mnt/us/documents/vault-app/$name"
  fi
done
put "$PKG/Vault.sh" /mnt/us/documents/Vault.sh
echo "Copied. Open 'Vault' from the Kindle library."
