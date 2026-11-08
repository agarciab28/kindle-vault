#!/bin/sh
# Bóveda de Obsidian en el Kindle: wrapper del cliente ob (Go, parcheado, solo lectura).
# Uso desde kTerm:  sh /mnt/us/ob-kindle.sh login | setup | sync | status
BIN=/mnt/us/ob-kindle
VAULT=/mnt/us/vault
export HOME=/mnt/us/ob-home
# El Kindle solo trae CAs de Amazon: usamos un bundle estándar copiado desde el Mac.
export SSL_CERT_FILE=/mnt/us/ob-cacert.pem
mkdir -p "$HOME" "$VAULT"

mirror_ok() {
  # En Linux la config vive en ~/.config/obsidian-headless (en macOS, ~/.obsidian-headless).
  grep -q '"syncMode": *"mirror"' "$HOME"/.config/obsidian-headless/sync/*/config.json "$HOME"/.obsidian-headless/sync/*/config.json 2>/dev/null
}

case "$1" in
  login)
    "$BIN" login
    ;;
  setup)
    # Nombre de la bóveda remota: VAULT_NAME="Otra" sh /mnt/us/ob-kindle.sh setup
    "$BIN" sync-setup --vault "${VAULT_NAME:-My Vault}" --path "$VAULT" --device-name "Kindle Paperwhite" || exit 1
    # sync-setup reinicia el modo: hay que poner mirror ANTES de cualquier sync.
    "$BIN" sync-config --path "$VAULT" --mode mirror --configs "" || exit 1
    "$BIN" sync-config --path "$VAULT"
    ;;
  sync)
    if ! mirror_ok; then
      echo "ALTO: el modo no es mirror. Corre: sh /mnt/us/ob-kindle.sh setup"
      exit 1
    fi
    "$BIN" sync --path "$VAULT"
    ;;
  status)
    "$BIN" sync-config --path "$VAULT"
    ;;
  *)
    echo "Uso: sh /mnt/us/ob-kindle.sh login | setup | sync | status"
    ;;
esac
