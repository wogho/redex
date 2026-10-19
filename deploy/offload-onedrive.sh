#!/usr/bin/env bash
# Offload Redex-local heavy artifacts to onedrive:redex, verify, then free local disk.
# Rule: our internal artifacts live on OneDrive as the archive; the host keeps only
# what the running deployment needs (current image + source tree + live DB).
set -euo pipefail
STAGE=/home/ubuntu/redex-offload
LOG=/tmp/redex-offload.log
exec > >(tee -a "$LOG") 2>&1

rm -rf "$STAGE"
mkdir -p "$STAGE"/{images,source,db,artifacts}
cd /opt/artex-ko
C=$(git rev-parse --short=7 HEAD)
IMG=$(sudo docker inspect redex-redex-1 --format "{{.Config.Image}}")
TAG=${IMG##*:}
echo "[offload] commit=$C image=$IMG $(date -Is)"

echo "[offload] current image"
sudo docker save "$IMG" | zstd -T4 -3 -q -o "$STAGE/images/redex-global-$TAG.tar.zst"

echo "[offload] source"
git bundle create "$STAGE/source/redex-$C.bundle" --all
tar --exclude=.git --exclude=web/node_modules --exclude=web/.next --exclude=web/out \
    --exclude=server/webui/dist --exclude=dist --exclude=data --exclude=.secrets --exclude=.env \
    -czf "$STAGE/source/redex-worktree-$C.tar.gz" .
cp compose.production.yml "$STAGE/source/"

echo "[offload] build artifact"
tar -czf "$STAGE/artifacts/redex-bin-$TAG.tar.gz" -C dist arm64 2>/dev/null || true

echo "[offload] db dump"
sudo docker exec redex-postgres-1 pg_dump -U artex -d artex | gzip > "$STAGE/db/redex-db-$(date +%Y%m%d-%H%M).sql.gz"

echo "[upload] start"
rclone copy "$STAGE" onedrive:redex --transfers 2 --checkers 4 --stats 30s --stats-one-line -v
echo "[upload] verify"
rclone check "$STAGE" onedrive:redex --size-only --one-way
{
  echo "# redex offload manifest $(date -Is)"
  echo "# commit $C"
  rclone ls onedrive:redex
} > "$STAGE/MANIFEST-$TAG.txt"
rclone copy "$STAGE/MANIFEST-$TAG.txt" onedrive:redex/ 2>/dev/null || true
echo "[upload] done"

echo "[cleanup] old images"
sudo docker rmi redex-global:4caad68 redex-global:3b5ff6e artex-global:fdfb7a1 artex-runtime:local 2>&1 | tail -3 || true

echo "[cleanup] build outputs"
rm -rf dist web/out server/webui/dist

echo "[cleanup] builder cache"
sudo docker builder prune -f 2>&1 | tail -2 || true

echo "[done] $(date -Is)"
df -h / | sed -n '2p'
