# 팩 컴플라이언스 — 추가·검증·철회 런북

이 문서는 Redex 소스 팩을 **합법적으로 배포·유지**하기 위한 최소 절차를 정한다.
목적은 "몰랐다"가 아니라 **"확인하고 기록했다"** 를 남기는 것이다.

## 1. 원칙

1. 재배포가 **명시적으로 허용된** 데이터만 팩으로 만든다.
2. 팩마다 **출처·라이선스 원문·해시·검토자·검토일·재배포 근거**를 기록한다.
3. 의심스러우면 **배포하지 않는다**. 회색지대는 동의 게이트 + 고지로 처리하되,
   리스크가 크면 기본 미배포로 둔다.
4. 통지(라이선스/권리/악성)를 받으면 **24시간 내 비활성**, **72시간 내 회신**.
5. 이 문서는 법률 자문이 아니다. 법인 배포·고위험 팩은 전문가 검토를 받는다.

## 2. 팩 추가 체크리스트

| # | 확인 | 통과 기준 |
|---|---|---|
| 1 | 출처 | 공식 업스트림 또는 배포판(Kali/Debian 등) |
| 2 | 고정 | 커밋/패키지 버전 고정 (`latest` 금지) |
| 3 | 라이선스 | SPDX 식별자 확인, `LICENSE`/`COPYRIGHT` **팩에 포함** |
| 4 | 재배포 근거 | `redistribution_basis` 에 문장으로 기록 |
| 5 | 개인정보 | 개인정보·유출 DB·계약 데이터 **없음** |
| 6 | 악성코드 | 릴리스 전 스캔, 실행 파일 최소화 |
| 7 | 시크릿 | 키·토큰·비밀번호 포함 없음 |
| 8 | 크기/해시 | `sha256`·`size` 기록, 재현 가능한 빌드 |
| 9 | 고지 | `DISCLAIMER.md` 연결, 제한 팩은 동의 필요 표시 |
| 10 | 검토 | `reviewed_by`, `reviewed_at` 기록 |

## 3. manifest 필드

```json
{
  "id": "exploitdb",
  "version": "20261002",
  "license": "GPL-2.0+",
  "license_source": "kali:/usr/share/doc/exploitdb/copyright",
  "upstream": "https://www.exploit-db.com",
  "commit": "kali-exploitdb_20261002-0kali1",
  "url": "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/exploitdb-20261002.tar.zst",
  "sha256": "588a785a...",
  "size": 46131589,
  "consent_required": true,
  "reviewed_by": "operator",
  "reviewed_at": "2026-10-20",
  "redistribution_basis": "Kali package declares GPL-2.0+; COPYRIGHT/LICENSE included verbatim"
}
```

## 4. 릴리스 전 검증

```bash
# 저장소 루트에서
bash scripts/pack-license-check.sh /path/to/redex-packs/manifest.json
```

검사 항목: 필수 필드, 해시 형식, LICENSE/COPYRIGHT 존재(설치된 팩), 알려진 금지 데이터 패턴.

실패 시 릴리스하지 않는다.

## 5. 철회 런북 (통지 접수)

1. **접수 기록**: 원문·시각·경로(이슈/메일) 보존
2. **24h 내 비활성**
   - `gh release delete-asset <tag> <asset>` 로 자산 제거
   - `manifest.json` 에서 항목 제거 후 커밋
   - 필요 시 `defaultPackManifest` 에서도 제거(다음 릴리스)
3. **72h 내 회신**: 접수·조치·재발방지 요약
4. **원인 분석**: 라이선스/개인정보/악성/계약 위반 중 무엇인지
5. **재발 방지**: 체크리스트 항목 추가, 필요 시 다른 팩 재검토
6. **공지**: 릴리스 노트/CHANGELOG에 철회 사유 기록

### 회신 템플릿

```
접수 확인: <일시>
대상: <pack id / version / asset>
조치: <asset 삭제 / manifest 제거 / 설치 차단>
원인: <요약>
재발 방지: <체크리스트 보강 내용>
문의: <연락처>
```

## 6. 고위험 팩 취급

| 팩 | 위험 | 정책 |
|---|---|---|
| ExploitDB | 서드파티 PoC 라이선스 혼재 | Kali 원본 그대로, COPYRIGHT 포함, 동의 필수. 리스크 커지면 기본 미배포 |
| PayloadsAllTheThings / fuzzdb | 공격 페이로드 | 동의 필수, "공격 목적 아님" 고지 |
| NVD / KEV / EPSS | 공공 데이터 | 그대로 |
| GeoLite2 / 유출 DB | 재배포 금지 | **배포 금지** (사용자 반입만) |

## 7. 기록 보존

- manifest 커밋 이력 = 검토 이력
- 릴리스 자산 해시 = 배포물 동일성
- 통지·조치 로그 = 방치 아님의 증거
- 보존 기간: 최소 3년 권장

## 8. 관련 문서

- [`NOTICE.md`](../NOTICE.md) — 무보증·책임 배제
- [`TERMS.ko.md`](../TERMS.ko.md) §12–§15 — 배포·프로바이더·동의·면책
- [`docs/verification-and-policy.md`](verification-and-policy.md) — 동의 게이트
## 9. 예외 — public domain

미국 정부 공공저작물(NVD, CISA KEV 등)은 라이선스 파일 첨부 의무가 없다. 이런 팩은
manifest에 `"license_file_required": false` 를 두고, `license_source`·`redistribution_basis`
로 근거를 남긴다. 그 외 팩은 LICENSE/COPYRIGHT 포함이 필수다.
