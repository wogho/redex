---
name: web-auth-session
description: 认证与会话攻击（JWT/OAuth/会话固定/密码重置/2FA）时调用该skill。
---

# Web 认证与会话攻击（Auth & Session）

在**已授权**目标上，把"登录前后"的所有身份边界当作攻击面：令牌、Cookie、OAuth 流程、密码重置、MFA。目标是拿到**另一个身份或更高权限**的可用凭据，而不是停在"发现可疑"。

## 判定顺序（Step 0：先建模，再动手）

1. 收集：登录/注册/注销、刷新令牌、第三方登录（OAuth/OIDC）、密码重置、2FA 设置与校验、会话 Cookie、记住我、邀请链接。
2. 标记每个信任决策点：`谁签发`（签名算法/密钥）、`谁校验`（服务端是否真的验签）、`谁能重放`（Cookie/Session/Token 有效期与吊销）。
3. 只对判定为"服务端信任边界"的点做攻击；纯客户端跳转（JS 路由守卫）只作为信息泄露面。

## 攻击清单

### JWT
- `alg: none` / `alg` 大小写 / `HS256↔RS256` 算法混淆：用 `jwt_tool <token> -M at` / `jwt_tool -X a` 批量试。
- `kid` 注入：`kid=../../dev/null`、SQL/命令注入、路径穿越到可控文件。
- JWK/JKU 注入：`jku` 指向自建 JWKS，`x5u` 类似。
- 弱密钥：`jwt_tool -C -d /opt/wordlists/rockyou.txt` 或 `hashcat -m 16500`。
- 声明滥用：`sub`/`role`/`tenant`/`scope` 篡改后是否被服务端二次校验。
- 过期与吊销：登出/改密后旧 token 是否仍可用；`exp` 未校验。

### 会话
- Cookie 属性：`HttpOnly`/`Secure`/`SameSite`；会话固定（登录前 Set-Cookie 是否复用）。
- 会话令牌可预测：连续采样看时间/随机源；越权替换他人会话 ID。
- 并发会话：改密后其他设备会话是否失效。

### OAuth / OIDC
- `redirect_uri` 校验：开放重定向、路径拼接、`@`、通配、子域。
- `state` 缺失/不校验 → CSRF 绑定攻击。
- PKCE 降级、`response_mode=form_post`、隐式流 token 泄露。
- 账号绑定：同邮箱不同 IdP 是否互相接管。

### 密码重置 / 邮箱验证
- 令牌强度与复用（改密后旧链接是否失效）、Host 头注入重置链接、`X-Forwarded-Host`。
- 用户枚举：响应差异/时序/邮件发送。
- 重置后登录：旧会话是否失效。

### MFA
- 校验接口能否直接跳过（前端校验/接口未强制）。
- 备份码、OTP 重放、OTP 爆破（限速/锁定）、短信轰炸。
- 2FA 绑定流程能否预先绑定攻击者设备（账号接管）。

## 证据要求（report_finding 必填）

- 攻击前后两个身份的**原始请求/响应**（含 Cookie/Authorization）。
- 复现命令（curl 完整参数），以及"为什么服务端信了"的一句话判定依据。
- 影响：能读到什么、能改什么、能维持多久。

## 常见误报

- 仅前端隐藏按钮（无接口校验）。
- 不带凭据的 200（公共接口）。
- 令牌过期时间短但没有越权（只是配置差异）。
- 库默认行为（如未设置 SameSite 但站点无跨站敏感操作）。

## 工具

`jwt_tool` · `hashcat -m 16500` · `ffuf`（重置/OTP 接口枚举）· `curl`/`python requests`（脚本化时序与重放）· Playwright（真实浏览器流程）。
