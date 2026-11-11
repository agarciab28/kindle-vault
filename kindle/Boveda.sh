#!/bin/sh
# Name: Bóveda
# Author: agarciab28
# Launcher for Bóveda (an unofficial, read-only Obsidian vault reader):
# starts vaultd if it is not running and opens the app.
APPDIR=/mnt/us/vault-app
mkdir -p "$APPDIR"
# Optional settings: see kindle/boveda.conf.example.
[ -f "$APPDIR/boveda.conf" ] && . "$APPDIR/boveda.conf"
if ! pidof vaultd >/dev/null 2>&1; then
  cd "$APPDIR" || exit 1
  nohup "$APPDIR/vaultd" -state "$APPDIR/state.json" ${TZ_NAME:+-tz "$TZ_NAME"} >>"$APPDIR/vaultd.log" 2>&1 &
  sleep 1
fi
lipc-set-prop com.lab126.appmgrd start app://com.alexgar.vault
