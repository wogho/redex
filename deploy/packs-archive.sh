#!/usr/bin/env bash
# Archive local source packs to OneDrive (the master copy lives externally).
#
#   bash deploy/packs-archive.sh            # archive ./data/packs
#   bash deploy/packs-archive.sh /path/dir  # archive another REDEX_PACKS_DIR
set -eu

SRC="${1:-./data}"
REMOTE="${REDEX_PACKS_REMOTE:-onedrive:redex/packs-archive}"
ARCHIVE="redex-packs.tar.zst"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ ! -d "$SRC/packs" ]; then
  echo "[packs] nothing to archive: $SRC/packs not found" >&2
  exit 1
fi

echo "[packs] archiving $SRC/packs"
tar --zstd -cf "$TMP/$ARCHIVE" -C "$SRC" packs
ls -lh "$TMP/$ARCHIVE"

echo "[packs] uploading to $REMOTE"
rclone copy "$TMP" "$REMOTE" --stats-one-line

echo "[packs] verifying remote"
rclone ls "$REMOTE" | grep -F "$ARCHIVE" || {
  echo "[packs] upload verification failed" >&2
  exit 1
}
echo "[packs] done"
