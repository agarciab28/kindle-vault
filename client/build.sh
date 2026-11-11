#!/bin/sh
# Builds the patched Obsidian Sync client (obsidian-headless, Go port by Belphemur)
# for the Kindle (ARMv7, static) from a clean upstream checkout, after running the
# patch's own tests. Set NATIVE=1 to also build it for this computer.
#
# Patch (obsidian-headless-v0.3.6-kindle.patch):
#   1. "pull"/"mirror" modes are really read-only in one-shot sync (upstream ignored them).
#   2. Legacy encryption v0 vaults are treated as encrypted (managed password from the API).
#   3. v0 path decryption takes the 12-byte IV prefix (like content decryption).
#   4. A piped password is read as a whole line (spaces included).
#   5. Tests for 1 and 3 (*_patch_test.go).
#   6. sync-setup refuses an encrypted vault when no password is given.
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
OUT="${1:-$HERE/../build}"
TAG=v0.3.6
# The commit the patch was written and reviewed against. A tag can be moved;
# a commit hash cannot, so the build refuses anything else.
COMMIT=2afab3b2dcfe4ef525a8107e2712b24c09498a87
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

git clone --quiet --depth 1 --branch "$TAG" https://github.com/Belphemur/obsidian-headless.git "$WORK/ob"
GOT=$(git -C "$WORK/ob" rev-parse HEAD)
if [ "$GOT" != "$COMMIT" ]; then
  echo "ERROR: $TAG points to $GOT, expected $COMMIT. Review upstream before building." >&2
  exit 1
fi
git -C "$WORK/ob" apply "$HERE/obsidian-headless-v0.3.6-kindle.patch"

mkdir -p "$OUT"
cd "$WORK/ob/src"
export GOFLAGS=-mod=mod GOTOOLCHAIN=local CGO_ENABLED=0
# The read-only guarantee is tested on every build.
go test ./internal/sync/ ./internal/encryption/ -run 'ReadOnlyPatch|V0PathPatch'
GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w -buildid=" -o "$OUT/ob-kindle" ./cmd/ob-go
echo "built: $OUT/ob-kindle"
if [ "${NATIVE:-}" = 1 ]; then
  go build -trimpath -ldflags "-s -w" -o "$OUT/ob-$(go env GOOS)" ./cmd/ob-go
  echo "built: $OUT/ob-$(go env GOOS)"
fi
