# NOTICE — Redex

## What this is

Redex is a fork of the open-source project **ARTEX** (upstream: [Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX), via the global edition [jiwoochris/artex-global](https://github.com/jiwoochris/artex-global) and the Korean edition [wogho/artex-ko](https://github.com/wogho/artex-ko)), distributed under **AGPL-3.0**. The original copyright notices and license remain in `LICENSE`; upstream documentation, changelogs and detection material are kept verbatim and still refer to ARTEX.

## Dangerous tool — read before running

Redex is an **autonomous offensive security framework**. Once started against a target it can, without further human confirmation:

- scan, enumerate and fingerprint systems;
- exploit vulnerabilities it finds;
- execute commands, read data and move laterally;
- in `REDEX_MODE=offensive`, run every tool call **without an approval gate**.

It is powerful enough to disrupt or destroy production systems, expose personal data, and violate computer-crime and privacy law. Misuse is a criminal offence in most jurisdictions.

## Authorized use only

Run Redex **only** against systems you own, or for which you hold **explicit prior written authorization** that defines the scope, time window and permitted techniques. If you are unsure whether you are authorized, you are not authorized.

## No warranty — liability rests with the operator

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.

TO THE MAXIMUM EXTENT PERMITTED BY LAW, THE AUTHORS, MAINTAINERS AND CONTRIBUTORS ACCEPT **NO LIABILITY** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL OR EXEMPLARY DAMAGES — including data loss, service outage, regulatory fines, criminal or civil liability — ARISING FROM THE USE OR MISUSE OF THIS SOFTWARE. **ALL RESPONSIBILITY AND ALL LEGAL CONSEQUENCES REST SOLELY WITH THE PERSON OR ORGANIZATION THAT DOWNLOADS, INSTALLS OR RUNS IT.**

Downloading, installing, building or running Redex constitutes acceptance of these terms and of the AGPL-3.0 license.

## No secrets in this repository

This repository contains **no credentials, API keys, private keys, database dumps or customer data**. Deployment secrets (`.env`, `.secrets/`, `jwt.key`, database volumes, backups) are git-ignored and must never be committed. If you deploy Redex, keep your own secrets out of version control as well.

## Korean / 한국어 요지

- Redex는 공격형 자동화 도구입니다. **소유하거나 서면으로 명시적 허가를 받은 시스템에만** 사용하십시오.
- 허가 없는 스캔·침투·익스플로잇은 형사처벌 대상입니다(정보통신망법, 개인정보보호법 등).
- 본 소프트웨어는 어떠한 보증도 없이 "있는 그대로" 제공되며, 사용으로 인해 발생하는 모든 직·간접적 손해와 법적 책임은 **전적으로 사용자에게 있습니다**. 제작자·기여자는 책임지지 않습니다.
- 이 저장소에는 비밀번호·API 키·개인키·DB 덤프 등 민감정보가 포함되어 있지 않습니다. 배포용 비밀값은 반드시 저장소 밖에서 관리하십시오.

## Terms

Full terms of use and liability allocation: [TERMS.ko.md](TERMS.ko.md) · [TERMS.en.md](TERMS.en.md).
