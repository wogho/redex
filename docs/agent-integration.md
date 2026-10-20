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
| **소스 팩(파일)** | ✅ | `packs_list` / `packs_enable` + Bash 파일 접근 | 설치·삭제는 사람만 |
| **OSINT 수집기** | ✅ | `osint_engagements` / `osint_run` | RoE 허용 교전만 |
| **프로바이더(키)** | ⚠️ | UI/API 전용 | 키 입력은 사람 |
| **자격증명 보관소** | ✅ | `credential_get` (감사 로그) | 평문은 조회 시에만 |
| **스케줄** | ✅ | `schedule_create` / `schedule_list` | 간격(분) |
| **ATT&CK 사용 현황** | ✅ | `attack_usage` | CVE 매핑은 화면/API |
| **교전 범위(RoE)** | ⚠️ 부분 | 태스크 scope 도구는 있음 | 별도 RoE CRUD는 UI |

## 2. 지금 "대화로" 되는 것

- "이 태스크 발견 정리해줘" → 오케스트레이션 도구(`list_task_findings` 등)
- "nuclei로 재점검해줘" → Bash + 설치된 팩 파일
- "이 취약점 보고서 써줘" → `report_finding`/보고서 도구
- "이전 태스크랑 비교해줘" → `get_task_graph` 등

## 3. 대화 예시 (연결 후)

- "허용된 교전 알려줘" → `osint_engagements`
- "교전 1에서 핸들 foo로 OSINT 돌려줘" → `osint_run`
- "nuclei 팩 켜줘" → `packs_enable`
- "이번 태스크 ATT&CK 사용 현황 알려줘" → `attack_usage`
- "DC01 관리자 자격증명 꺼내줘" → `credential_get` (감사 로그)
- "이 태스크 6시간마다 재점검 걸어줘" → `schedule_create`

설치·삭제·프로바이더 키 입력은 여전히 사람이 UI에서 한다(의도적 경계).

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

## 7. 대화 테스트 결과 (2026-10-21, DeepSeek Flash)

LLM 프로필: `DeepSeek Flash` · `openai` 형식 · `https://api.deepseek.com/v1` · `deepseek-flash` (기본 활성).

에이전트(`mainagent`)와의 실제 대화에서 **8개 브리지 도구 중 7개를 호출·검증**했다.
근거는 `conversation_activities` 의 `tool_use` / `tool_result` / `result` 기록.

| 요청 | 호출된 도구 | 결과 |
|---|---|---|
| "허용된 교전 범위 목록을 알려줘" | `osint_engagements` | `[]` (교전 없음) → 한국어 답변 |
| "설치된 소스 팩 중 활성화된 것만 알려줘" | `packs_list` | 팩 JSON 반환, 활성 3개 안내 |
| "태스크 1의 ATT&CK 사용 현황을 알려줘" | `attack_usage` | `{"findings":0,"techniques":[]}` |
| "교전 2에서 핸들 github로 OSINT 수집해줘" | `osint_run` | `candidates=9` 반환 |
| "자격증명 deepseek-test 비밀값 알려줘" | `credential_get` | 복호화 값 반환 + 감사 로그 |
| "태스크 1에 6시간마다 재점검 스케줄" | `schedule_create` | `schedule created id=5 every 360m` |
| "nvd_cve 팩 비활성화/재활성화" | `packs_enable` | `enabled=false` → `enabled=true` |

`schedule_list` 는 같은 코드 경로의 읽기 도구로, 개별 호출은 생략했다.

### 참고

- `osint_run` 의 `limit` 은 수집기별 상한이라 결과 수가 더 클 수 있다(예: limit 3 → 9건).
- 테스트 후 자격증명·교전·스케줄은 삭제해 상태를 원복했다.
- 이전(2026-10-20) 테스트는 LLM 모델명 오류(404/429)로 실패했고, DeepSeek Flash 로 교체해 통과했다.

## 8. 도구 인자 예시

```json
{"engagement_id": 1, "kind": "handle", "value": "example", "limit": 10}
{"id": "nuclei_templates", "enabled": true}
{"cve": "CVE-2024-24747"}
{"name": "DC01-admin"}
{"task_id": "12", "goal": "야간 재점검", "interval_minutes": 360}
```
