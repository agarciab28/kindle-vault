#!/bin/sh
# Instala la app Bóveda en el Kindle (se ejecuta en el Kindle, como root).
# Requiere que deploy.sh haya copiado los archivos a /mnt/us/vault-app/.
set -e
APPDIR=/mnt/us/vault-app
WAFDIR=/var/local/mesquite/vault
DB=/var/local/appreg.db
ID=com.alexgar.vault

# 1. WAF (la interfaz) donde el Kindle busca sus apps web.
mkdir -p "$WAFDIR"
cp "$APPDIR"/waf/* "$WAFDIR"/

# 2. Copia de seguridad del registro de apps (solo la primera vez).
[ -f "$APPDIR/appreg.db.orig" ] || cp "$DB" "$APPDIR/appreg.db.orig"

# 3. Registrar la app, igual que las WAF de Amazon (Ayuda, Kindle Unlimited...).
sqlite3 "$DB" <<SQL
INSERT OR IGNORE INTO handlerIds VALUES ('$ID');
INSERT OR REPLACE INTO properties VALUES ('$ID', 'command', '/usr/bin/mesquite -l $ID -c file://$WAFDIR/');
INSERT OR REPLACE INTO properties VALUES ('$ID', 'framework', '');
INSERT OR REPLACE INTO properties VALUES ('$ID', 'unloadPolicy', 'unloadOnPause');
INSERT OR REPLACE INTO associations VALUES ('$ID', 'application', 'none', 'false');
SQL

# 4. Lanzador en la biblioteca.
cp "$APPDIR/Boveda.sh" /mnt/us/documents/Boveda.sh
chmod +x "$APPDIR/vaultd" "$APPDIR/Boveda.sh" 2>/dev/null || true

echo "Bóveda instalada. Ábrela desde el libro 'Bóveda' de la biblioteca."
