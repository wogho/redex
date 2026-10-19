# 검증 게이트와 운영 정책 (P0)

Redex의 P0 품질 장치 두 가지. 둘 다 **모델과 무관하게 코드로 강제**된다.

## 1) 검증 게이트 — `verify` 패키지

`report_finding` 이 호출되면 findings 에 쓰기 **전에** `verify.Analyze` 가 판정한다.

| 판정 | 조건 | 동작 |
|---|---|---|
| `rejected` | 증거가 40자 미만 | **등록 거부**. 에이전트에게 명령·원시 출력을 요구하는 에러 반환 |
| `unverified` | 기록됨, 그러나 명령/원시 출력 신호 없음 | 등록 + confidence 낮음 |
| `evidence-backed` | 재현 가능한 명령/HTTP 원시 출력 + 120자 이상 + confidence ≥ 60 | 등록 + 신뢰 표시 |

confidence(0–100)는 증거 길이, 재현 신호(`curl`, `HTTP/1.`, `uid=`, 코드블록 등),
바인딩된 HTTP 트래픽 수, 심각도, 요약 길이로 계산한다. 산식은 `verify.Analyze` 참고.

### 중복 제거

`DedupeKey = sha1(정규화(vulnclass) | 정규화(name) | 정렬된 asset_ids)[:16]` 를
`finding_verification` 에 저장한다. 같은 태스크에서 같은 키가 이미 있으면
새 행을 만들지 않고 **기존 finding_id 를 반환**하며 `duplicate: true` 를 표시한다.
에이전트는 중복 등록을 멈추고 기존 항목에 증거를 보강하면 된다.

### 저장 위치

`finding_verification(finding_id, confidence, status, dedupe_key, reasons, created_at)`.
findings 테이블의 SELECT/scan 을 건드리지 않기 위한 1:1 보조 테이블이다.

`report_finding` 응답에 `confidence`, `verify_status`, `verify_reasons`, `duplicate` 가
포함되므로 에이전트가 스스로 품질을 판단할 수 있다.

## 2) 운영 정책 — `guard.Policy`

`REDEX_POLICY_FILE` 로 JSON 을 지정한다. **공격형 모드에서도 항상 먼저 적용**된다.
게이트 우회는 "승인 절차"를 없애는 것이지, 운영자가 선언한 금지 명령을 허용하는 게 아니다.

```json
{
  "deny": ["rm\\s+-rf\\s+/(\\s|$)", "mkfs\\."],
  "allow": [],
  "deny_tools": ["shell_open"]
}
```

- `deny`: `<tool>\n<command>` 에 매칭되는 RE2 정규식 → 차단
- `allow`: 비어 있지 않으면 **허용 목록 모드**(매칭 안 되면 차단)
- `deny_tools`: 도구 이름 정확히 일치 → 차단

예시: [`config/policy.example.json`](../config/policy.example.json)

### 사람 승인 강제

```bash
REDEX_REQUIRE_HUMAN=1
```

공격형 모드에서도 intercept 승인 게이트를 다시 켠다. 고객사 교전처럼
"사람이 매 단계 확인"이 필요한 경우 사용한다.

## 적용 확인

```bash
docker logs redex-redex-1 | grep -E "\[mode\]|\[policy\]"
```

- `[mode] offensive …` : 게이트 우회 + 공격 프롬프트
- `[policy] …` : 정책 파일 로드 실패 시 경고(빈 정책으로 계속)

## 테스트

```bash
go test ./verify ./guard ./config
```
