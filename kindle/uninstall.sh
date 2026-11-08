#!/bin/sh
# Quita la app Bóveda del Kindle. No toca la bóveda sincronizada (/mnt/us/vault)
# ni la sesión de Obsidian (/mnt/us/ob-home).
ID=com.alexgar.vault
DB=/var/local/appreg.db
killall vaultd 2>/dev/null || true
sqlite3 "$DB" <<SQL
DELETE FROM associations WHERE handlerId='$ID';
DELETE FROM properties WHERE handlerId='$ID';
DELETE FROM handlerIds WHERE handlerId='$ID';
SQL
rm -rf /var/local/mesquite/vault
rm -f /mnt/us/documents/Boveda.sh
echo "Bóveda desinstalada."
