# ARTEX deployment: artex.noog2infra.site

Source: https://github.com/wogho/artex-ko.git

Installed source revision: `777c41c` (branch `offensive`, built from commit above).
The UI and Go binary are compiled from this revision for Linux ARM64.
The web UI is built with `NEXT_PUBLIC_LOCALE=ko` (Korean default, switchable to en/zh/es/ja in Settings).
The upstream runtime tools and Playwright Chromium are included in the image.

Runtime mode: `ARTEX_MODE=offensive` — the intercept approval gate is bypassed
and every pentest prompt carries the exploit-first directive. Image also ships
sqlmap/hydra/masscan/whatweb/dirb/wfuzz/gobuster/nikto/ffuf/nuclei/subfinder/
impacket/chisel and `/opt/wordlists`. See `docs/offensive-mode.md`.

## Operations

Installation directory: `/opt/artex-ko`.

```sh
cd /opt/artex-ko
docker compose -f compose.production.yml ps
docker compose -f compose.production.yml logs --tail=100 artex
docker compose -f compose.production.yml up -d
```

The app is published only at `127.0.0.1:8787`, behind host nginx.
PostgreSQL and the traffic-recording proxy have no published ports.
Containers restart automatically after host reboot.

First visit: https://artex.noog2infra.site/setup/
Use `ARTEX_SETUP_TOKEN` from `.env` to set the password for username `ARTEX`.
The `.env` file has mode 0600 and is excluded from Git and Docker build contexts.
Configure an LLM provider and API key through the authenticated UI.

## Persistence and backups

PostgreSQL data lives in Docker volume `artex_pgdata`.
Workspaces and traffic data live in `/opt/artex-ko/data`.
Back up both, along with `.env` and `.secrets/jwt.key`.
The JWT signing key is persisted outside the browsable data directory
and mounted read-only at `/app/jwt.key`.
Never use `docker compose down -v` unless intentionally deleting the database.

## Rebuild

```sh
cd /opt/artex-ko
bash deploy/build.sh
docker compose -f compose.production.yml up -d artex
```

The deployment configuration pins the local image tag to the installed revision (`artex-global:777c41c`).
Before deploying a different revision, update that tag and this record together.

## HTTPS

Host nginx configuration: `/etc/nginx/sites-available/artex`.
Certificate: `/etc/letsencrypt/live/artex.noog2infra.site/`.
Certbot's scheduled renewal uses `/var/www/html` for HTTP-01 challenges.
The deploy hook `/etc/letsencrypt/renewal-hooks/deploy/artex-reload-nginx`
reloads nginx after renewing this certificate.

## Validation scope

Deployment checks cover the static UI, health endpoint, authentication boundary,
PostgreSQL connection, container health, and HTTPS redirect/certificate.
LLM calls require the owner's provider settings and API key.
Installation does not launch any reconnaissance or testing tasks.

The source lockfile's npm audit currently reports 24 advisories, including two
critical package entries (`next`, `proxy-addr`). This deployment exports the UI
to static files and serves it from Go; it runs no Next.js/Express HTTP server.
The dependency versions have been preserved from the requested source revision.

Verified on 2026-10-12: both containers healthy; HTTPS hostname/chain valid;
HTTP redirects to HTTPS; `/api/health` returns version `ko-9f8f91d`;
unauthenticated `/api/stats` returns 401; invalid setup token returns 403.
Chromium loaded the Korean setup form with three inputs and no page errors.
The administrator password remains unset for the owner to choose on first visit.
