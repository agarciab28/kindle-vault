#!/bin/sh
# Bóveda: wrapper del cliente de sincronización (obsidian-headless parcheado, solo lectura).
# Uso desde kTerm o SSH:  sh /mnt/us/ob-kindle.sh login | logout | whoami | vaults | setup | sync | status
BIN=/mnt/us/ob-kindle
VAULT=/mnt/us/vault
export HOME=/mnt/us/ob-home
# El Kindle solo trae CAs de Amazon: usamos un bundle estándar copiado desde el Mac.
export SSL_CERT_FILE=/mnt/us/ob-cacert.pem
mkdir -p "$HOME" "$VAULT"
# Ajustes opcionales (kindle/boveda.conf.example). Lo que venga del entorno gana.
ENV_VAULT_NAME="${VAULT_NAME:-}"
[ -f /mnt/us/vault-app/boveda.conf ] && . /mnt/us/vault-app/boveda.conf
[ -n "$ENV_VAULT_NAME" ] && VAULT_NAME="$ENV_VAULT_NAME"

mirror_ok() {
  # En Linux la config vive en ~/.config/obsidian-headless (en macOS, ~/.obsidian-headless).
  grep -q '"syncMode": *"mirror"' "$HOME"/.config/obsidian-headless/sync/*/config.json "$HOME"/.obsidian-headless/sync/*/config.json 2>/dev/null
}

case "$1" in
  login)
    # Interactive from kTerm/SSH. The app calls it as
    #   login --accept-disclaimer --email X [--mfa N]   with the password on stdin.
    shift
    "$BIN" login "$@"
    ;;
  logout)
    "$BIN" logout
    ;;
  whoami)
    # Prints "Logged in as: <email>" if the saved session is valid; fails otherwise.
    # stdin is empty, so it never starts a new login.
    "$BIN" login < /dev/null
    ;;
  vaults)
    # Las bóvedas de la cuenta: id, nombre y servidor.
    "$BIN" sync-list-remote
    ;;
  setup)
    # Qué bóveda: VAULT_NAME="Mis notas" sh /mnt/us/ob-kindle.sh setup   (o en boveda.conf)
    # Lo que siga a "setup" pasa al cliente, p. ej. --password para una bóveda con contraseña propia.
    shift
    if [ -z "${VAULT_NAME:-}" ]; then
      echo "Falta el nombre de la bóveda. Estas son las de tu cuenta (id, nombre, servidor):"
      "$BIN" sync-list-remote
      echo 'Luego corre:  VAULT_NAME="Nombre" sh /mnt/us/ob-kindle.sh setup'
      exit 1
    fi
    "$BIN" sync-setup --vault "$VAULT_NAME" --path "$VAULT" --device-name "${DEVICE_NAME:-Kindle}" "$@" || exit 1
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
    echo "Uso: sh /mnt/us/ob-kindle.sh login | logout | whoami | vaults | setup | sync | status"
    ;;
esac
