#
# 실행 전용 이미지(이미지 안에서 컴파일하지 않음): 상용 도구만 설치하고, **미리 컴파일한 Linux 단일 바이너리**를 넣는다.
# 바이너리는 CI 의 binaries job 이 크로스 컴파일하며(순수 Go, QEMU 없음), 목표 아키텍처별로
# 빌드 컨텍스트의 dist/<TARGETARCH>/redex 에 둔다. 이렇게 하면 다중 아키텍처 빌드에서 arm64 는 apt 계층만
# 에뮬레이션하면 되고, Next/Go 컴파일은 더 이상 에뮬레이션하지 않아 훨씬 빠르다.
#
# 로컬에서 이미지를 수동으로 빌드할 때는 먼저 바이너리를 직접 준비한다:
#   cd web && npm run build:static && cd ..
#   mkdir -p server/webui && cp -r web/out server/webui/dist
#   CGO_ENABLED=0 GOARCH=amd64 go build -tags embedui -o dist/amd64/redex ./cmd/redex
#   docker build -t redex:local .

# kerbrute 는 upstream 이 arm64 바이너리를 배포하지 않아 빌드 스테이지에서 직접 만든다.
FROM golang:1.26.3-bookworm AS kerbrute-build
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go install github.com/ropnop/kerbrute@v1.0.3

# mitm6/pypykatz 의 네이티브 의존성(netifaces 등)은 빌드 스테이지에서 컴파일하고
# 런타임 이미지에는 산출물만 복사한다(빌드 도구를 최종 이미지에 남기지 않음).
FROM python:3.12-slim-bookworm AS pypi-build
RUN apt-get update && apt-get install -y --no-install-recommends build-essential libpcap-dev \
    && pip install --no-cache-dir --prefix=/install mitm6 pypykatz

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
# 공격 도구 세트 (REDEX_MODE=offensive 에서 에이전트가 Bash 로 바로 사용)
# ---------------------------------------------------------------------------
# Tier 1-A: AD/내부 열거 (SMB/LDAP) + Tier 1-B: 세션·자격증명 + Tier 1-D: 영향 증명용 DB 클라이언트
RUN apt-get update && apt-get install -y --no-install-recommends \
      sqlmap hydra masscan whatweb dirb wfuzz gobuster perl \
      libxml-writer-perl libnet-ssleay-perl \
      samba-common-bin smbclient ldap-utils \
      hashcat john \
      postgresql-client default-mysql-client redis-tools \
    && rm -rf /var/lib/apt/lists/*
# impacket(AD 공격), ldapdomaindump·bloodhound(공격 경로), certipy-ad(ADCS ESC),
# arjun(숨은 파라미터), awscli(클라우드 영향 증명), rockyou 크래킹 보조.
RUN pip install --no-cache-dir impacket ldapdomaindump bloodhound certipy-ad arjun awscli
# nikto: Debian 패키지가 없어 업스트림 저장소에서 직접 받는다(perl 스크립트).
RUN git clone --depth 1 https://github.com/sullo/nikto.git /opt/nikto \
    && ln -sf /opt/nikto/program/nikto.pl /usr/local/bin/nikto \
    && nikto -Version >/dev/null
# jwt_tool: JWT alg confusion / none / kid injection / 시크릿 크랙.
RUN git clone --depth 1 https://github.com/ticarpi/jwt_tool.git /opt/jwt_tool \
    && ln -sf /opt/jwt_tool/jwt_tool.py /usr/local/bin/jwt_tool \
    && python3 -m pip install --no-cache-dir -r /opt/jwt_tool/requirements.txt
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
# kerbrute: 위 빌드 스테이지에서 만든 arm64 바이너리.
COPY --from=kerbrute-build /go/bin/kerbrute /usr/local/bin/kerbrute
# nuclei 템플릿은 빌드 시점에 받아 둔다(런타임 네트워크 의존 제거). 실패해도 빌드는 계속한다.
RUN nuclei -update-templates -silent || true
# Tier 1-E: 포스트 익스플로잇 스크립트(PEASS linpeas/winpeas, LSE, mimikatz).
RUN mkdir -p /opt/peass /opt/linux-exploit-suggester /opt/mimikatz \
    && curl -fsSL -o /opt/peass/linpeas.sh https://github.com/peass-ng/PEASS-ng/releases/latest/download/linpeas.sh \
    && curl -fsSL -o /opt/peass/winPEASany.exe https://github.com/peass-ng/PEASS-ng/releases/latest/download/winPEASany.exe \
    && chmod +x /opt/peass/linpeas.sh \
    && git clone --depth 1 https://github.com/mzet-/linux-exploit-suggester.git /opt/linux-exploit-suggester \
    && curl -fsSL -o /tmp/mimikatz.zip https://github.com/gentilkiwi/mimikatz/releases/download/2.2.0-20220919/mimikatz_trunk.zip \
    && unzip -oq /tmp/mimikatz.zip -d /opt/mimikatz && rm -f /tmp/mimikatz.zip \
    && ln -sf /opt/linux-exploit-suggester/linux-exploit-suggester.sh /usr/local/bin/les
# 자주 쓰는 워드리스트를 이미지에 고정한다(런타임 다운로드 제거). rockyou 전체 포함.
RUN mkdir -p /opt/wordlists && cd /opt/wordlists \
    && curl -fsSL -o raft-medium-directories.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/raft-medium-directories.txt \
    && curl -fsSL -o raft-small-words.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/raft-small-words.txt \
    && curl -fsSL -o common.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/common.txt \
    && curl -fsSL -o rockyou-75.txt https://raw.githubusercontent.com/danielmiessler/SecLists/master/Passwords/Leaked-Databases/rockyou-75.txt \
    && curl -fsSL -o /tmp/rockyou.tar.gz https://raw.githubusercontent.com/danielmiessler/SecLists/master/Passwords/Leaked-Databases/rockyou.txt.tar.gz \
    && tar -xzf /tmp/rockyou.tar.gz -C /opt/wordlists && rm -f /tmp/rockyou.tar.gz \
    && ls -lh /opt/wordlists

# impacket 은 pip 설치 시 콘솔 스크립트를 만들지 않으므로, 자주 쓰는 예제를
# `impacket-<name>` 래퍼로 노출한다(에이전트가 짧은 이름으로 호출할 수 있게).
RUN set -eux; \
    for t in secretsdump psexec wmiexec smbexec atexec GetNPUsers GetUserSPNs ntlmrelayx smbclient lookupsid; do \
      printf '#!/bin/sh\nexec python3 -m impacket.examples.%s "$@"\n' "$t" > /usr/local/bin/impacket-$t; \
      chmod +x /usr/local/bin/impacket-$t; \
    done; \
    impacket-secretsdump --help >/dev/null 2>&1 || true

# ---------------------------------------------------------------------------
# P1: C2 / 데이터 유출 / Windows 자격증명 수집
# ---------------------------------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends rclone \
    && rm -rf /var/lib/apt/lists/*
COPY --from=pypi-build /install/lib/python3.12/site-packages/ /usr/local/lib/python3.12/site-packages/
COPY --from=pypi-build /install/bin/ /usr/local/bin/
RUN set -eux; \
    case "${TARGETARCH:-amd64}" in \
      amd64) A=amd64 ;; \
      arm64) A=arm64 ;; \
      *) echo "unsupported TARGETARCH ${TARGETARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL -o /usr/local/bin/sliver-server "https://github.com/BishopFox/sliver/releases/download/v1.7.8/sliver-server_linux-${A}"; \
    curl -fsSL -o /usr/local/bin/sliver "https://github.com/BishopFox/sliver/releases/download/v1.7.8/sliver-client_linux-${A}"; \
    chmod +x /usr/local/bin/sliver-server /usr/local/bin/sliver; \
    mkdir -p /opt/windows; \
    curl -fsSL -o /opt/windows/LaZagne.exe "https://github.com/AlessandroZ/LaZagne/releases/download/v2.4.7/LaZagne.exe"; \
    ls -lh /opt/windows

# ---------------------------------------------------------------------------
# OSINT: 공개 정보 수집(maigret/holehe/ghunt/phoneinfoga/instaloader) + 로컬 얼굴 임베딩(InsightFace)
# ---------------------------------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends libgl1 libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir maigret holehe ghunt sherlock-project yt-dlp onnxruntime opencv-python-headless insightface instaloader
RUN set -eux; \
    case "${TARGETARCH:-amd64}" in amd64) A=amd64 ;; arm64) A=arm64 ;; *) exit 1 ;; esac; \
    curl -fsSL -o /tmp/phoneinfoga.tgz "https://github.com/sundowndev/phoneinfoga/releases/download/v2.11.0/phoneinfoga_Linux_${A}.tar.gz"; \
    tar -xzf /tmp/phoneinfoga.tgz -C /usr/local/bin phoneinfoga; \
    rm -f /tmp/phoneinfoga.tgz; \
    phoneinfoga version
COPY osint/ /app/osint/
# buffalo_l 모델을 빌드 시점에 받아 런타임 네트워크 의존을 없앤다.
RUN python3 -c "from insightface.app import FaceAnalysis; FaceAnalysis(name='buffalo_l', providers=['CPUExecutionProvider']).prepare(ctx_id=-1, det_size=(640,640))"

# 빌드 시 도구 존재 확인(하나라도 없으면 이미지 빌드가 실패한다).
RUN set -eux; \
    for b in smbclient rpcclient ldapsearch hashcat john psql mysql redis-cli aws \
             arjun certipy ldapdomaindump bloodhound-python jwt_tool kerbrute \
             rclone mitm6 pypykatz sliver-server sliver maigret holehe ghunt sherlock yt-dlp phoneinfoga instaloader \
             ffuf nuclei subfinder chisel nikto sqlmap hydra masscan whatweb gobuster impacket-secretsdump; do \
      command -v "$b"; \
    done

WORKDIR /app
# 미리 컴파일한 해당 아키텍처 바이너리(dist/amd64/redex 또는 dist/arm64/redex)
COPY dist/${TARGETARCH}/redex /app/redex
# 감시 기동 스크립트: 프로세스 종료 후 종료 코드에 따라 다시 띄울지 결정하며, 화면의 원클릭 업데이트가 이 스크립트로 교체를 완료한다.
# 이 스크립트는 동시에 SIGTERM 을 redex 로 전달하는 일도 맡는다: docker stop 은 신호를 PID 1 에만 보내므로,
# 전달하지 않으면 redex 가 신호를 받지 못해 우아한 종료를 못 하고 10 초 뒤 SIGKILL 로 강제 종료된다.
COPY start.sh /app/start.sh
RUN chmod +x /app/redex /app/start.sh
COPY skills/ /app/skills/
COPY detections/attack/redex_offense_layer.json /app/attack/redex_offense_layer.json
# data/(SQLite + jwt.key) 영속화 지점
VOLUME ["/app/data"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/start.sh"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
