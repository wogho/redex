# syntax=docker/dockerfile:1
#
# 실행 전용 이미지(이미지 안에서 컴파일하지 않음): 상용 도구만 설치하고, **미리 컴파일한 Linux 단일 바이너리**를 넣는다.
# 바이너리는 CI 의 binaries job 이 크로스 컴파일하며(순수 Go, QEMU 없음), 목표 아키텍처별로
# 빌드 컨텍스트의 dist/<TARGETARCH>/artex 에 둔다. 이렇게 하면 다중 아키텍처 빌드에서 arm64 는 apt 계층만
# 에뮬레이션하면 되고, Next/Go 컴파일은 더 이상 에뮬레이션하지 않아 훨씬 빠르다.
#
# 로컬에서 이미지를 수동으로 빌드할 때는 먼저 바이너리를 직접 준비한다:
#   cd web && npm run build:static && cd ..
#   mkdir -p server/webui && cp -r web/out server/webui/dist
#   CGO_ENABLED=0 GOARCH=amd64 go build -tags embedui -o dist/amd64/artex ./cmd/artex
#   docker build -t artex:local .
FROM python:3.12-slim-bookworm
ARG TARGETARCH
# 상용 도구: ripgrep / curl / vim 에 더해 recon 에 자주 쓰는 도구 묶음(필요에 따라 추가·삭제).
# Node 는 NodeSource 에서 20.x 를 설치한다: bookworm 기본 apt nodejs 는 18 이고, Playwright 는 >=20 을 요구한다.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ripgrep curl wget vim git jq unzip \
      dnsutils iputils-ping netcat-openbsd inetutils-telnet whois nmap \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
# Playwright MCP 와 CLI 를 전역으로 미리 설치한다(런타임에 npx 로 네트워크 다운로드하지 않도록).
# @playwright/mcp: browser MCP 는 `npx @playwright/mcp` 로 바로 실행한다(전역 설치 완료, -y/@latest 불필요).
# @playwright/cli: playwright-cli 를 제공하며, 설치 후 --help 로 실행 가능 여부를 확인한다.
# 다음으로 playwright(브라우저 관리 제공)를 설치하고, --with-deps 로 chromium 과 그 시스템 의존성을 미리 깔아 둔다.
# 이렇게 하면 컨테이너 안의 MCP/CLI 가 최초 기동 시 바로 쓸 수 있고, 브라우저를 네트워크로 내려받지 않는다.
RUN npm install -g @playwright/mcp@latest @playwright/cli@latest playwright@latest \
    && playwright-cli --help \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------------------
# 공격 도구 세트 (ARTEX_MODE=offensive 에서 에이전트가 Bash 로 바로 사용)
# ---------------------------------------------------------------------------
# apt 로 들어오는 것: sqlmap(주입), hydra(자격증명), masscan(대역 스캔),
# whatweb(지문), dirb/wfuzz/gobuster(디렉터리·파라미터 퍼징), perl(nikto 런타임).
RUN apt-get update && apt-get install -y --no-install-recommends \
      sqlmap hydra masscan whatweb dirb wfuzz gobuster perl \
    && rm -rf /var/lib/apt/lists/*
# impacket: SMB/LDAP/Kerberos 공격·자격증명 도구 모음(python3).
RUN pip install --no-cache-dir impacket
# nikto: Debian 패키지가 없어 업스트림 저장소에서 직접 받는다(perl 스크립트).
RUN git clone --depth 1 https://github.com/sullo/nikto.git /opt/nikto \
    && ln -sf /opt/nikto/program/nikto.pl /usr/local/bin/nikto
# 단일 바이너리 도구(Go): 릴리스 아티팩트를 아키텍처에 맞춰 배치한다.
RUN set -eux; \
    case "${TARGETARCH:-amd64}" in \
      amd64) A=amd64 ;; \
      arm64) A=arm64 ;; \
      *) echo "unsupported TARGETARCH ${TARGETARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/ffuf.tgz "https://github.com/ffuf/ffuf/releases/download/v2.3.0/ffuf_2.3.0_linux_${A}.tar.gz"; \
    tar -xzf /tmp/ffuf.tgz -C /usr/local/bin ffuf; \
    curl -fsSL -o /tmp/nuclei.zip "https://github.com/projectdiscovery/nuclei/releases/download/v3.11.1/nuclei_3.11.1_linux_${A}.zip"; \
    unzip -oq /tmp/nuclei.zip -d /usr/local/bin; \
    curl -fsSL -o /tmp/subfinder.zip "https://github.com/projectdiscovery/subfinder/releases/download/v2.17.0/subfinder_2.17.0_linux_${A}.zip"; \
    unzip -oq /tmp/subfinder.zip -d /usr/local/bin; \
    curl -fsSL -o /tmp/chisel.deb "https://github.com/jpillora/chisel/releases/download/v1.12.0/chisel_1.12.0_linux_${A}.deb"; \
    dpkg -i /tmp/chisel.deb; \
    rm -f /tmp/ffuf.tgz /tmp/nuclei.zip /tmp/subfinder.zip /tmp/chisel.deb; \
    ffuf -V | head -1; nuclei -version | head -1; subfinder -version | head -1; chisel --version
# nuclei 템플릿은 빌드 시점에 받아 둔다(런타임 네트워크 의존 제거). 실패해도 빌드는 계속한다.
RUN nuclei -update-templates -silent || true
# 자주 쓰는 워드리스트를 이미지에 고정한다(런타임 다운로드 제거).
RUN mkdir -p /opt/wordlists && cd /opt/wordlists \
    && curl -fsSL -o raft-medium-directories.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/raft-medium-directories.txt \
    && curl -fsSL -o raft-small-words.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/raft-small-words.txt \
    && curl -fsSL -o common.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/common.txt \
    && curl -fsSL -o rockyou-75.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Passwords/Leaked-Databases/rockyou-75.txt \
    && ls -lh /opt/wordlists

WORKDIR /app
# 미리 컴파일한 해당 아키텍처 바이너리(dist/amd64/artex 또는 dist/arm64/artex)
COPY dist/${TARGETARCH}/artex /app/artex
# 감시 기동 스크립트: 프로세스 종료 후 종료 코드에 따라 다시 띄울지 결정하며, 화면의 원클릭 업데이트가 이 스크립트로 교체를 완료한다.
# 이 스크립트는 동시에 SIGTERM 을 artex 로 전달하는 일도 맡는다: docker stop 은 신호를 PID 1 에만 보내므로,
# 전달하지 않으면 artex 가 신호를 받지 못해 우아한 종료를 못 하고 10 초 뒤 SIGKILL 로 강제 종료된다.
COPY start.sh /app/start.sh
RUN chmod +x /app/artex /app/start.sh
COPY skills/ /app/skills/
# data/(SQLite + jwt.key) 영속화 지점
VOLUME ["/app/data"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/start.sh"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
