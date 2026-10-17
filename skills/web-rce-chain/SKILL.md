---
name: web-rce-chain
description: 服务端注入与远程代码执行链（SSTI/反序列化/XXE/文件上传/命令注入）时调用该skill。
---

# 服务端 RCE 链（SSTI / 反序列化 / XXE / 上传 / 命令注入）

目标不是"报一个高危"，而是**从注入点走到可复现的命令执行或文件读取**，并保留原始证据。

## 通用推进顺序

1. **确认 sink**：输入从哪里进入、在哪一层被解释（模板引擎、反序列化器、XML 解析器、shell、文件系统）。
2. **无害探针**：算术/回显/带外（DNS/HTTP）三选一，先证明"可控"，再升级。
3. **构造最小 PoC**：`id`/`whoami`、读 `/etc/hostname`、写一个可控文件。
4. **证明影响**：命令输出、文件内容、带外交互日志三件套。
5. 失败就换层：编码、包装器、协议、参数位置、Content-Type、分块传输。

## SSTI

- 指纹：`${7*7}`、`{{7*7}}`、`#{7*7}`、`<%= 7*7 %>`、`*{7*7}`、`@(7*7)`。
- 引擎判定后按引擎给 payload（Jinja2/Twig/Freemarker/Velocity/ERB/Handlebars）。
- 目标：`config`/`self`/`__class__` 链、模板内建对象，最终 `os.popen` / `Runtime.exec`。
- 盲 SSTI：带外 DNS（`nslookup`/`curl` 到可控域名）+ 时间延迟双确认。

## 反序列化

- Java：`ysoserial` gadget（CommonsCollections、Spring、Groovy、JDK7u21），先解决 classpath 判定；注意 `Content-Type`、`Cookie`、`ViewState`、RMI/JNDI 入口。
- PHP：`phpggc` 生成链；`phar://` 元数据反序列化；`unserialize` 入口。
- Python：`pickle`（`__reduce__`）、`yaml.load`、`jsonpickle`。
- .NET：`ysoserial.net`（Windows 侧执行）。
- 验证：DNS/HTTP 带外优先，避免直接弹 shell 触发防护。

## XXE

- 经典：`<!ENTITY xxe SYSTEM "file:///etc/passwd">`。
- 盲 XXE：外部 DTD + 参数实体 + 带外（`http://attacker/`）。
- 升级：`php://filter`、`expect://`、SSRF 内网探测、`jar:`/`file:` 读取。
- 覆盖点：SOAP、SVG、DOCX/XLSX、SAML、RSS、任何 XML 解析入口。

## 文件上传 → WebShell → RCE

- 校验类型：前端 MIME、后端扩展名黑名单、magic bytes、二次渲染、图片解析器。
- 绕过：双扩展名、`.phtml/.pht/.phar`、大小写、尾随点/空格、`%00`、`.htaccess`、Content-Type 伪造、图片马 + 解析漏洞。
- 落点：上传目录是否可执行、是否有目录穿越（`filename=../../`）、是否可覆盖配置。
- 拿到执行后：先 `id`，再确认可写目录与出网，再决定是否落持久化（默认不落）。

## 命令注入

- 分隔符：`; | || & && \n $() `` ` ``。
- 盲注：时间延迟（`sleep 5`）、带外 DNS、写文件到 web 根。
- 过滤绕过：`${IFS}`、`$@`、通配符、base64 管道、换行、变量拼接、`/bin/ca?`。
- 参数注入：`--option` 注入、`-o` 写文件、模板参数。

## 证据要求

- 完整请求（含 header/body/编码）+ 响应原文。
- 命令与原始输出；带外证据给出 DNS/HTTP 日志时间戳。
- 影响链一句话：入口 → sink → 执行上下文（用户/容器/主机）。

## 工具

`commix` · `curl` · `python3` · `sqlmap --os-shell`（仅确认可用时）· `ysoserial`/`phpggc` · 自带 `/opt/wordlists`。
