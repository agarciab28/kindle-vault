#!/bin/sh
# KindleForge package: removes Vault completely. It signs out at Obsidian first
# (which also removes the downloaded notes), then deletes the app, the session
# and the settings. Nothing is ever changed in Obsidian.
APP=/mnt/us/documents/vault-app

killall vaultd 2>/dev/null || true
if [ -f "$APP/ob" ]; then
  sh "$APP/ob" logout >/dev/null 2>&1 || true
fi
if [ -f "$APP/uninstall" ]; then
  sh "$APP/uninstall" --all || true
else
  ID=com.alexgar.vault
  sqlite3 /var/local/appreg.db "DELETE FROM associations WHERE handlerId='$ID'; DELETE FROM properties WHERE handlerId='$ID'; DELETE FROM handlerIds WHERE handlerId='$ID';" || true
  rm -rf /var/local/mesquite/vault /var/local/mesquite/$ID /mnt/us/.active_content_sandbox/vault
  rm -rf /mnt/us/vault /mnt/us/ob-home /mnt/us/vault-data
  rm -rf /mnt/us/documents/Vault.sh /mnt/us/documents/Vault.sh.sdr /mnt/us/documents/vault-app
fi

exit 0
