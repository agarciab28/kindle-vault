#!/bin/sh
# Builds what goes on the Kindle, in build/:
#   pkg/Vault.sh   the launcher, with the library cover embedded as its "# Icon:" line
#   pkg/vault-app/     the app: vaultd, the sync client and its wrapper, the interface, a CA bundle
#   vault.zip      both, ready to unzip into the Kindle's "documents" folder
# The sync client comes from client/build.sh (run automatically if it is missing).
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
BUILD="$HERE/build"
PKG="$BUILD/pkg"
export GOTOOLCHAIN=local CGO_ENABLED=0

[ -f "$BUILD/ob-kindle" ] || "$HERE/client/build.sh" "$BUILD"

(cd "$HERE/vaultd" && go vet ./... && go test ./... &&
  GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w -buildid=" -o "$BUILD/vaultd" .)

# CA bundle for the sync client (the Kindle only ships Amazon's CAs): this
# computer's own, unless CACERT points to another one.
CA=""
for c in "${CACERT:-}" /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem; do
  if [ -n "$c" ] && [ -s "$c" ]; then CA="$c"; break; fi
done
[ -n "$CA" ] || { echo "ERROR: no CA bundle found; set CACERT=/path/to/cacert.pem" >&2; exit 1; }

rm -rf "$PKG" "$BUILD/vault.zip"
mkdir -p "$PKG/vault-app"
cp "$BUILD/vaultd" "$BUILD/ob-kindle" "$PKG/vault-app/"
cp "$HERE"/waf/config.xml "$HERE"/waf/index.html "$HERE"/waf/app.css "$HERE"/waf/app.js "$PKG/vault-app/"
cp "$HERE/kindle/ob" "$HERE/kindle/uninstall" "$HERE/kindle/vault.conf.example" "$HERE/LICENSE" "$PKG/vault-app/"
cp "$CA" "$PKG/vault-app/cacert.pem"
VERSION=$(git -C "$HERE" describe --tags --always --dirty 2>/dev/null || echo unknown)
cat > "$PKG/vault-app/SOURCE" <<TXT
Vault $VERSION, an unofficial read-only Obsidian vault reader for Kindle.
License: GNU GPL v3 (see LICENSE). No warranty.

Source code: https://github.com/agarciab28/kindle-vault (this version: $VERSION)

"ob-kindle" is obsidian-headless v0.3.6
(https://github.com/Belphemur/obsidian-headless, commit
2afab3b2dcfe4ef525a8107e2712b24c09498a87, GPL v3) with the patch in client/ of
the repository above. "vaultd" is built from vaultd/ in the same repository.
TXT
awk -v icon="# Icon: data:image/png;base64,$(base64 < "$HERE/kindle/cover.png" | tr -d '\n')" \
  '{ print } /^# Author:/ { print icon }' "$HERE/kindle/Vault.sh" > "$PKG/Vault.sh"
chmod +x "$PKG/Vault.sh" "$PKG/vault-app/vaultd" "$PKG/vault-app/ob-kindle"
# Fixed timestamps, so the same sources and toolchain give the same zip.
find "$PKG" -exec touch -t 202601010000 {} +
(cd "$PKG" && zip -q -X -r "$BUILD/vault.zip" Vault.sh vault-app)
if command -v sha256sum >/dev/null 2>&1; then SUM=$(sha256sum "$BUILD/vault.zip" | cut -d' ' -f1); else SUM=$(shasum -a 256 "$BUILD/vault.zip" | cut -d' ' -f1); fi
echo "$SUM  vault.zip" > "$BUILD/vault.zip.sha256"
echo "built: $BUILD/vault.zip ($VERSION, sha256 $SUM)"
