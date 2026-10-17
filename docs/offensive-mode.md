# 공격형 모드 (`REDEX_MODE=offensive`)

Redex 를 "정찰·검증 중심"에서 **"익스플로잇 우선"** 으로 바꾸는 런타임 모드.
코드는 모드 분기만 하고, 기본값(미설정)은 상류 동작 그대로다.

## 켜는 법

```yaml
# compose / docker run
environment:
  REDEX_MODE: offensive
```

또는 `docker run -e REDEX_MODE=offensive ...`. 재기동하면 적용된다.

## 바뀌는 것

| 계층 | 기본 모드 | 공격형 모드 |
|---|---|---|
| **승인 게이트** (`guard.applyIntercept`) | DB intercept 규칙 → LLM 판정 → 수동 승인 대기 | **전부 우회, 즉시 허용** (감사 로그는 그대로 기록) |
| **planner / worker / mainagent / goals 프롬프트** | 정찰·검증·증거 수집 중심 | 익스플로잇 우선 지시문(`agent/offensive.go`)을 본문 뒤에 **코드로 고정 주입** |
| **이미지 도구** | nmap, curl, playwright 등 | + sqlmap, hydra, masscan, whatweb, dirb, wfuzz, gobuster, nikto, ffuf, nuclei(+templates), subfinder, impacket, chisel, `/opt/wordlists` |
| **감사 추적** | 차단/승인 모두 기록 | 호출마다 `allow` 기록(게이트만 없음) |

지시문은 DB 에서 편집한 프롬프트 본문 뒤에 붙으므로, UI 에서 프롬프트를 고쳐도 사라지지 않는다
(`langDirective`·`artifactSpec` 과 같은 방식).

## 안 바뀌는 것

- **에이전트의 추론 본문**(중국어 벤치마크 프롬프트 段[A])은 그대로 둔다. 성향만 바꾼다.
- **출력 언어** 규약, 중간 산출물 경로 규약, 그래프 도구·이벤트 루프.
- **범위 규율**: 지시문 6항에 "교전 선언 범위 밖第三方 인프라 금지"를 명시한다. 범위 자체는
  태스크 생성 시 입력하는 target/scope 가 결정한다.

## 되돌리기

`REDEX_MODE` 를 지우거나 `default` 로 바꾸고 재기동. DB·볼륨·프롬프트 수정은 건드리지 않는다.

## 확인

```bash
docker logs redex-redex-1 | grep '\[mode\]'
# [mode] offensive — intercept 승인 게이트 비활성화 · 공격(익스플로잇) 우선 프롬프트 주입

docker exec redex-redex-1 bash -lc 'sqlmap --version; ffuf -V | head -1; nuclei -version | head -1'
```

## 주의

승인 게이트가 없으므로 에이전트가 실행하는 명령은 **사람 확인 없이 즉시 실행**된다.
대상 범위를 좁게 잡고, 파괴적 명령이 섞이지 않도록 태스크 지시문에 제약을 함께 적는 것을 권한다.
