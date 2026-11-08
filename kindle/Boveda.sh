#!/bin/sh
# Name: Bóveda
# Author: Obsidian
# Lanzador de la app Bóveda: arranca vaultd (si no corre) y abre la app.
APPDIR=/mnt/us/vault-app
mkdir -p "$APPDIR"
if ! pidof vaultd >/dev/null 2>&1; then
  cd "$APPDIR" || exit 1
  nohup "$APPDIR/vaultd" -state "$APPDIR/state.json" >>"$APPDIR/vaultd.log" 2>&1 &
  sleep 1
fi
lipc-set-prop com.lab126.appmgrd start app://com.alexgar.vault
