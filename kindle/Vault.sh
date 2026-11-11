#!/bin/sh
# Name: Vault
# Author: agarciab28
# Vault, an unofficial read-only reader for an Obsidian vault. This launcher
# installs the app the first time (and after an update), starts its local
# service and opens it. It only touches its own files and its own rows in the
# Kindle's app registry.
SRC=/mnt/us/documents/vault-app
DATA=/mnt/us/vault-data
WAF=/var/local/mesquite/vault
DB=/var/local/appreg.db
ID=com.alexgar.vault
CMD="/usr/bin/mesquite -l $ID -c file://$WAF/"

if [ ! -f "$SRC/vaultd" ] || [ ! -f "$SRC/ob-kindle" ]; then
  echo "Vault: the 'vault-app' folder must be next to this file (documents/vault-app)."
  echo "Vault: falta la carpeta 'vault-app' junto a este archivo (documents/vault-app)."
  exit 1
fi
mkdir -p "$DATA" "$WAF" || exit 1

# 1. The app's interface, next to the Kindle's own web apps. Copied only when it changes.
#    (token.js in that folder is written by vaultd at each start.)
for f in config.xml index.html app.css app.js; do
  cmp -s "$SRC/$f" "$WAF/$f" 2>/dev/null || cp "$SRC/$f" "$WAF/$f" || exit 1
done

# 2. Register the app the way Amazon's own web apps are (only if it is not there yet).
if [ "$(sqlite3 "$DB" "SELECT value FROM properties WHERE handlerId='$ID' AND name='command'")" != "$CMD" ]; then
  sqlite3 "$DB" <<SQL || exit 1
BEGIN;
INSERT OR IGNORE INTO interfaces(interface) VALUES ('application');
INSERT OR IGNORE INTO handlerIds(handlerId) VALUES ('$ID');
INSERT OR REPLACE INTO properties(handlerId, name, value) VALUES ('$ID', 'command', '$CMD');
INSERT OR REPLACE INTO properties(handlerId, name, value) VALUES ('$ID', 'framework', '');
INSERT OR REPLACE INTO properties(handlerId, name, value) VALUES ('$ID', 'unloadPolicy', 'unloadOnPause');
INSERT OR REPLACE INTO associations(handlerId, interface, contentId, defaultAssoc) VALUES ('$ID', 'application', 'none', 'false');
COMMIT;
SQL
  sleep 1
fi

# 3. The local service. Restarted when the program was updated.
# Optional settings: see vault.conf.example.
[ -f "$DATA/vault.conf" ] && . "$DATA/vault.conf"
STAMP=$(ls -l "$SRC/vaultd" 2>/dev/null)
if [ "$STAMP" != "$(cat "$DATA/vaultd.stamp" 2>/dev/null)" ]; then
  killall vaultd 2>/dev/null
  sleep 1
fi
if ! pidof vaultd >/dev/null 2>&1; then
  cd "$DATA" || exit 1
  nohup "$SRC/vaultd" -state "$DATA/state.json" -client "/bin/sh $SRC/ob" ${TZ_NAME:+-tz "$TZ_NAME"} >>"$DATA/vaultd.log" 2>&1 &
  echo "$STAMP" > "$DATA/vaultd.stamp"
  sleep 1
fi

# 4. Open it.
lipc-set-prop com.lab126.appmgrd start "app://$ID"
