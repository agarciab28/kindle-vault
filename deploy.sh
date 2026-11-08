#!/bin/sh
# Compila vaultd para el Kindle y lo instala por Wi-Fi (servidor SSH de KOReader).
# Uso: ./deploy.sh <IP del Kindle>   (KEY=~/.ssh/otra_llave ./deploy.sh … para otra llave)
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
IP="${1:?uso: ./deploy.sh IP_DEL_KINDLE}"
KEY="${KEY:-$HOME/.ssh/id_ed25519}"
BUILD="$HERE/build"
mkdir -p "$BUILD"

(cd "$HERE/vaultd" && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 GOTOOLCHAIN=local \
  go build -trimpath -ldflags "-s -w" -o "$BUILD/vaultd" .)

k() { ssh -p 2222 -o BatchMode=yes -o ConnectTimeout=10 -o IdentitiesOnly=yes -i "$KEY" "root@$IP" "$@"; }
put() { k "cat > '$2'" < "$1"; }

k "mkdir -p /mnt/us/vault-app/waf; killall vaultd 2>/dev/null; true"
put "$BUILD/vaultd" /mnt/us/vault-app/vaultd
for f in "$HERE"/waf/*; do put "$f" "/mnt/us/vault-app/waf/$(basename "$f")"; done
put "$HERE/kindle/Boveda.sh" /mnt/us/vault-app/Boveda.sh
put "$HERE/kindle/install.sh" /mnt/us/vault-app/install.sh
put "$HERE/kindle/uninstall.sh" /mnt/us/vault-app/uninstall.sh
put "$HERE/kindle/ob-kindle.sh" /mnt/us/ob-kindle.sh
k "sh /mnt/us/vault-app/install.sh"
