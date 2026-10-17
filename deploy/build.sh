#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source_dir="$(pwd -P)"
version="global-$(git rev-parse --short=7 HEAD)"

# A container avoids host native-addon cache permission issues on ARM64.
docker run --rm --name redex-web-build \
  -e NEXT_TELEMETRY_DISABLED=1 -e NEXT_PUBLIC_LOCALE=${REDEX_UI_LOCALE:-ko} \
  -e NODE_OPTIONS=--max-old-space-size=4096 \
  -v "$source_dir:/src" -w /src/web node:22-bookworm-slim \
  sh -c 'npm ci --include=dev && npm run build:static'

mkdir -p server/webui/dist dist/arm64
rsync -a --delete web/out/ server/webui/dist/
docker run --rm --name redex-go-build \
  -e CGO_ENABLED=0 -e GOMAXPROCS=2 \
  -v "$source_dir:/src" \
  -v redex-go-mod:/go/pkg/mod \
  -v redex-go-cache:/root/.cache/go-build \
  -w /src golang:1.26.3-bookworm \
  go build -buildvcs=false -p 2 -tags embedui \
  -ldflags "-s -w -X main.version=$version" \
  -o dist/arm64/redex ./cmd/redex

docker compose -f compose.production.yml build redex
