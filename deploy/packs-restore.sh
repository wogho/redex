#!/usr/bin/env bash
# Restore source packs from the OneDrive archive (master copy lives externally).
#
#   bash deploy/packs-restore.sh            # restore into ./data/packs
#   bash deploy/packs-restore.sh /path/dir  # restore into another REDEX_PACKS_DIR
#
# The archive is produced by deploy/packs-archive.sh and stored at
# onedrive:redex/packs-archive/redex-packs.tar.zst.
set -eu

DEST="${1:-./data}"
REMOTE="${REDEX_PACKS_REMOTE:-onedrive:redex/packs-archive}"
ARCHIVE="redex-packs.tar.zst"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "[packs] fetching $REMOTE/$ARCHIVE"
rclone copy "$REMOTE/$ARCHIVE" "$TMP"

echo "[packs] verifying archive"
tar --zstd -tf "$TMP/$ARCHIVE" >/dev/null

mkdir -p "$DEST"
echo "[packs] extracting into $DEST/packs"
tar --zstd -xf "$TMP/$ARCHIVE" -C "$DEST"

echo "[packs] done: $(du -sh "$DEST/packs" 2>/dev/null | cut -f1) in $DEST/packs"
