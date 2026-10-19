# Redex ATT&CK 커버리지 (오펜스)

기준: MITRE ATT&CK Enterprise. **실행 가능(coverage)** 기준이며, "도구가 이미지에 있다"와
"에이전트가 그 기법을 실제로 수행할 수 있다"를 구분한다. 탐지 커버리지(`detections/`)와는 별개다.

범례: ✅ 실행 가능 · △ 부분(수동/제한) · ❌ 없음

## 전술 매트릭스

| 전술 | 상태 | 보유 도구/기법 | 대표 Technique |
|---|---|---|---|
| Reconnaissance (TA0043) | ✅ | `nmap` `masscan` `subfinder` `whatweb` `ffuf` `dnsutils` | T1595, T1592, T1590 |
| Resource Development (TA0042) | ❌ | — | T1583, T1608 |
| Initial Access (TA0001) | △ | 웹 취약점 이용(T1190), 유효 계정(T1078) | T1190, T1078 |
| Execution (TA0002) | △ | Bash/스크립트(T1059), WMI(T1047), at(T1053.005) | T1059, T1047 |
| Persistence (TA0003) | △ | 웹쉘 파일, AD Golden Ticket(`impacket-ticketer`) | T1505.003, T1558.001 |
| Privilege Escalation (TA0004) | ✅ | `linpeas` `LES`, `certipy` ADCS ESC, ACL 악용 | T1548, T1068, T1644 |
| Defense Evasion (TA0005) | ❌ | 터널/프록시 정도 | T1027, T1562, T1070 |
| Credential Access (TA0006) | ✅ | `secretsdump` `kerberoast` `AS-REP` `ntlmrelayx` `hashcat` `john` `hydra` `jwt_tool` `pypykatz` `LaZagne` `mitm6` | T1003.001, T1558.003, T1110, T1557.001 |
| Discovery (TA0007) | ✅ | `smbclient` `rpcclient` `ldapsearch` `ldapdomaindump` `bloodhound` `whatweb` `awscli` | T1087, T1018, T1069 |
| Lateral Movement (TA0008) | △ | `psexec` `wmiexec` `smbexec` `atexec`, PtH, SSH | T1021.002, T1021.004, T1550.002 |
| Collection (TA0009) | ❌ | HTTP 증거 캡처만 | T1005, T1113 |
| Command & Control (TA0011) | △ | `sliver`(서버/클라이언트), `chisel` `ligolo-ng` `socat` | T1071, T1095, T1573 |
| Exfiltration (TA0010) | △ | `rclone`, 수동 `curl`/`scp` | T1567.002, T1048 |
| Impact (TA0040) | ❌ (의도적 제외) | 영향 증명용 읽기만 | T1485, T1486, T1490 |

## P1에서 새로 추가된 공격 유형

| 추가 | ATT&CK Technique | 용도 |
|---|---|---|
| **Sliver** (server/client) | T1071, T1095, T1573, T1105, T1570 | C2 채널, 임플란트 운용, 피벗·파일 전송 |
| **rclone** | T1567.002, T1048 | 클라우드/원격 스토리지로 데이터 유출 검증 |
| **LaZagne** (Windows) | T1555, T1552.001 | 브라우저·메일·DB 등 저장 자격증명 수집 |
| **pypykatz** | T1003.001 | LSASS 메모리 덤프 오프라인 파싱 |
| **mitm6** | T1557.001 | DHCPv6/DNS 기반 AiTM, NTLM 릴레이 연계 |
| (기존) impacket | T1021.002, T1550.002, T1003.003 | SMB/WMI 실행, PtH, NTDS |
| (기존) certipy | T1649, T1558 | ADCS ESC, 인증서 악용 |
| (기존) kerbrute | T1110, T1087.002 | 사용자 열거/스프레이 |
| (기존) bloodhound | T1069.002, T1087.002 | 공격 경로 수집 |

## 의도적 제외

| 영역 | 이유 |
|---|---|
| Impact (랜섬/와이퍼/DoS) | 교전 범위를 넘고 서비스 파괴 위험. "영향 증명"은 읽기/명령 실행으로 대체 |
| Defense Evasion 심화 (AMSI/ETW 우회, 루트킷) | EDR 랩 전용. 일반 교전에서 사고 반경만 확대 |
| 대규모 피싱/스팸 인프라 | 별도 서비스(Gophish 등) 영역, 이미지 내장 부적합 |
| NetExec / evil-winrm | arm64 환경에서 wheel/gem 빌드 실패(PyPI·rubygems 배포 부재). impacket 래퍼로 대체 |

## 재현

```bash
# 이미지 내 도구 확인
docker exec redex-redex-1 bash -lc 'sliver-server --version; rclone version; mitm6 --help >/dev/null && echo mitm6; pypykatz --help >/dev/null && echo pypykatz; ls /opt/windows'

# 오펜스 Navigator layer 생성
python3 scripts/gen_offense_layer.py > detections/attack/redex_offense_layer.json
```
