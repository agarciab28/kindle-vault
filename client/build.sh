#!/bin/sh
# Builds the patched Obsidian Sync client (obsidian-headless, Go port by Belphemur)
# for the Kindle (ARMv7, static) and for macOS, from a clean upstream checkout.
#
# Patch (obsidian-headless-v0.3.6-kindle.patch):
#   1. "pull"/"mirror" modes are really read-only in one-shot sync (upstream ignored them).
#   2. Legacy encryption v0 vaults are treated as encrypted (managed password from the API).
#   3. v0 path decryption takes the 12-byte IV prefix (like content decryption).
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
GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w" -o "$OUT/ob-kindle" ./cmd/ob-go
go build -trimpath -ldflags "-s -w" -o "$OUT/ob-darwin" ./cmd/ob-go
echo "built: $OUT/ob-kindle $OUT/ob-darwin"
