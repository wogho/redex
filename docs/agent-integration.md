# 에이전트 ↔ 구성요소 연결 검증 및 사용법

기준 커밋: `4f38d57` · 2026-10-20

결론부터: **채팅 에이전트는 "도구·스킬·파일"에는 연결되어 있지만, OSINT/자격증명/스케줄/ATT&CK 화면 기능에는 직접 연결되어 있지 않다.** 아래는 실제 코드 배선을 확인한 결과다.

## 1. 연결 상태 매트릭스

| 구성요소 | 에이전트 접근 | 근거 | 비고 |
|---|---|---|---|
| **Bash / 파일 도구** | ✅ | `actool.DefaultTools()` (chat.go) | 명령 실행·파일 읽기/쓰기 |
| **스킬** | ✅ | `wireAgentAugment` + `agent_skill_visibility` | auto/pentest/worker에 기본 노출 |
| **MCP** | ✅ | 동일 augment 경로 | 설정된 서버만 |
| **트래픽 도구** | ✅ | `Manager.HostTools()` (capture 켜짐 시) | traffic_search/get |
| **오케스트레이션 도구** | ✅ | `orchestrationTools()` | 태스크 목록·생성·그래프·발견 조회 등, 바인딩된 에이전트만 |
| **발견 기록(게이트)** | ✅ | `report_finding` → `verify` → `finding_verification` | 증거 40자·중복·confidence |
| **소스 팩(파일)** | ⚠️ 부분 | Bash로 `/app/data/packs/<id>/<ver>/...` 읽기 | **설치/활성은 UI/API** — 에이전트가 켜지 못함 |
| **OSINT 수집기** | ❌ | HTTP API만 존재, 에이전트 도구 없음 | 채팅으로 "OSINT 해줘" 불가 |
| **프로바이더(키)** | ❌ | HTTP API만 | 동일 |
| **자격증명 보관소** | ❌ | HTTP API만 | 보안상 의도된 미노출 |
| **스케줄** | ❌ | 서버 ticker + HTTP API | 에이전트가 등록/조회 불가 |
| **ATT&CK 사용 현황/CVE 매핑** | ❌ | HTTP API + 화면 | 에이전트가 조회 불가 |
| **교전 범위(RoE)** | ⚠️ 부분 | 태스크 scope 도구는 있음 | 별도 RoE CRUD는 UI |

## 2. 지금 "대화로" 되는 것

- "이 태스크 발견 정리해줘" → 오케스트레이션 도구(`list_task_findings` 등)
- "nuclei로 재점검해줘" → Bash + 설치된 팩 파일
- "이 취약점 보고서 써줘" → `report_finding`/보고서 도구
- "이전 태스크랑 비교해줘" → `get_task_graph` 등

## 3. 지금 "대화로" 안 되는 것 (핵심)

- "OSINT 돌려서 후보 찾아줘" — 에이전트에 OSINT 도구 없음
- "자격증명 보관소에서 X 꺼내와" — 도구 없음(의도적)
- "매 6시간마다 재점검 걸어줘" — 스케줄 도구 없음
- "이번 교전 ATT&CK 사용 현황 보여줘" — 도구 없음
- "팩 설치해서 쓰게 해줘" — 설치 API는 있으나 에이전트 도구 아님

## 4. 연결을 닫는 방법 (권장 구현)

| 우선 | 브리지 | 방식 |
|---|---|---|
| P0 | `osint_run` 도구 | 호스트 도구로 등록, RoE 허용 검사 후 수집기 실행 |
| P0 | `packs_list` / `packs_enable` | 읽기·활성만 허용(설치·삭제는 사람) |
| P1 | `attack_usage` | 발견→기법 카운트 조회 |
| P1 | `credential_get` | **명시 요청 + 감사 로그** 조건으로 조회만 |
| P2 | `schedule_create` | 생성은 사람 승인 후 등록 |

호스트 도구는 `orchestrationTools()` 패턴(`roTool`)으로 추가하고, `tools` 테이블 바인딩으로 노출 에이전트를 제한한다.

## 5. 사용법 (현재 기준)

### 팩
1. `/function/osint` → **팩** 탭
2. 설치 → (동의 필요 팩은 팝업 동의) → 활성화
3. 에이전트는 `/app/data/packs/<id>/<version>/` 를 Bash로 사용

### OSINT
1. **교전 범위**에서 해당 교전의 **인물 조사 허용** 켜기
2. OSINT **수집** 탭에서 유형/값 실행 → 후보 확인
3. **프로바이더** 탭에서 키 저장 + 동의 → 실행

### ATT&CK
- **사용 현황**: 발견 기준 자동 집계 (휴리스틱)
- **커버리지**: 실행 가능 기법 + 사용/미사용
- **CVE 매핑**: ExploitDB·NVD 팩 설치 시 동작

### 자격증명
- AES-GCM 저장, 목록엔 힌트만, `👁`로 복사

### 스케줄
- 태스크 + 목표 + 간격(분) 등록 → 서버 ticker가 내부 API로 목표 추가·revive

## 6. 검증 명령

```bash
# 도구 목록·바인딩
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8787/api/tools | jq '.tools[].key' | head

# 팩 상태
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8787/api/packs | jq '.packs[] | {id, installed, enabled}'

# 게이트 결과
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8787/api/verification/summary
```
