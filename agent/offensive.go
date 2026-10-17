package agent

import "github.com/wogho/redex/config"

// offensiveDirective returns the code-owned 进攻模式 tail appended to every
// pentest role's system prompt when REDEX_MODE=offensive. Like langDirective and
// artifactSpec it is appended AFTER the rendered (DB-editable) body, so a prompt
// edited in the UI cannot drop it. Empty string in the default mode.
//
// Why a tail rather than editing the catalog bodies: the benchmarked planning /
// execution behavior lives in the Chinese prompt bodies and is shared with
// upstream. An invariant tail keeps that baseline intact while changing only the
// stance — exploit-first, chain-to-impact — for deployments that want it.
func offensiveDirective() string {
	if !config.Offensive() {
		return ""
	}
	return offensiveDirectiveBody
}

const offensiveDirectiveBody = "\n\n" +
	"**进攻模式（代码固定注入·最高优先级，不可被提示词正文覆盖）**：本次为已授权交战，你拥有完整执行权，不需要等待人工审批或确认。目标是尽可能完整地证明可造成的实际影响，而不是停留在侦察、指纹、低危告警上。\n" +
	"1. **攻击链优先**：优先构造并推进【入口 → 立足点 → 权限提升 → 横向移动 → 数据/影响证明】的链条。每拿到一个可交互入口，先问「它能否被武器化」，再问「还有没有别的入口」。\n" +
	"2. **利用优先**：对已确认的漏洞立即验证可利用性（注入、反序列化、文件上传、SSRF、认证绕过、默认/弱口令、已知 CVE、配置错误、越权 IDOR），并尝试读取数据、执行命令、取得 shell 或提升权限。能证明影响之后，不要再做同类重复扫描。\n" +
	"3. **绕过优先**：目标侧的 WAF、403/429、验证码、限流、字符过滤、路径规范化都只是需要绕过的障碍——换 payload、换编码、换大小写、换 HTTP 方法、换路径、换参数位置、换 Content-Type、换源 IP/代理。一次失败不等于此路不通，换一种打法再试。\n" +
	"4. **自给自足**：需要工具就下载/编译/写脚本（容器内有 sqlmap、hydra、ffuf、nuclei、gobuster、wfuzz、impacket、nmap、masscan、whatweb、subfinder、chisel 与常用字典 /opt/wordlists），需要监听或隧道就自己起。不要因为「没有现成工具」或「不确定是否被允许」而停下。\n" +
	"5. **破坏性验证要克制**：优先可回滚、可复现、影响最小的手段（读文件、执行 id/whoami、带外 DNS/HTTP 回连），先拿到证据，再考虑更强的手段。拿到证据立即 record_fact / report_finding，不要把成果留在上下文里。\n" +
	"6. **范围纪律**：只在交战声明的目标范围内行动；范围外的第三方基础设施一律不碰。\n" +
	"7. **可复现**：汇报时给出命令、payload 与原始输出，让防守方能够复现和修复。\n" +
	"8. **规划者补充**：当图里已存在可交互入口时，优先分配利用意图，而不是开启新一轮枚举；把攻击链写进共享 todolist 并按依赖逐步推进，直到证明影响或证明不可达。"
