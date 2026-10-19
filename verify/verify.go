// Package verify implements the finding verification gate: structural checks on
// a reported finding before it reaches the graph, deterministic confidence
// scoring, and the dedupe key used to merge restatements of the same issue.
//
// The gate is deliberately model-free: it cannot prove a vulnerability, but it
// can refuse findings that carry no reproducible evidence and it can tell a
// reviewer (human or agent) how much backing a finding has. That is what keeps
// the findings list from turning into a noisy scanner dump.
package verify

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Status values stored per finding.
const (
	// StatusRejected: evidence is too thin to be actionable; the write is refused.
	StatusRejected = "rejected"
	// StatusUnverified: something was recorded, but it lacks commands or raw output.
	StatusUnverified = "unverified"
	// StatusEvidenceBacked: reproducible evidence (command / raw request-response).
	StatusEvidenceBacked = "evidence-backed"
)

// MinEvidenceRunes is the hard floor for a finding to be recorded at all.
const MinEvidenceRunes = 40

// Input is the finding as submitted, before it is written.
type Input struct {
	VulnClass, Name, Severity, Summary, Evidence string
	AssetIDs                                     []int64
	TrafficRefs                                  int
}

// Result is the gate verdict.
type Result struct {
	Confidence int      `json:"confidence"`
	Status     string   `json:"verify_status"`
	DedupeKey  string   `json:"dedupe_key"`
	Reasons    []string `json:"verify_reasons,omitempty"`
}

// commandMarkers are cheap signals that the evidence contains something a
// reviewer can rerun, rather than a prose claim.
var commandMarkers = []string{
	"$ ", "curl ", "wget ", "nmap ", "sqlmap", "ffuf", "nuclei", "hydra", "hashcat",
	"HTTP/1.", "GET /", "POST /", "PUT /", "Request:", "Response:", "payload", "```",
	"id:", "uid=", "root:", "/etc/passwd", "ERROR", "Traceback",
}

// Analyze scores a submitted finding and decides whether it may be recorded.
func Analyze(in Input) Result {
	evidence := strings.TrimSpace(in.Evidence)
	reasons := make([]string, 0, 5)

	res := Result{
		Status:    StatusUnverified,
		DedupeKey: DedupeKey(in.VulnClass, in.Name, in.AssetIDs),
	}

	runes := len([]rune(evidence))
	if runes < MinEvidenceRunes {
		res.Status = StatusRejected
		res.Reasons = []string{fmt.Sprintf("증거가 %d자로 너무 짧습니다(최소 %d자)", runes, MinEvidenceRunes)}
		return res
	}

	score := 20
	reasons = append(reasons, "증거 있음(+20)")

	hasCommand := false
	for _, m := range commandMarkers {
		if strings.Contains(evidence, m) {
			hasCommand = true
			break
		}
	}
	if hasCommand {
		score += 20
		reasons = append(reasons, "재현 가능한 명령/원시 출력 포함(+20)")
	}
	if runes >= 160 {
		score += 10
		reasons = append(reasons, "증거 160자 이상(+10)")
	}
	if runes >= 600 {
		score += 10
		reasons = append(reasons, "증거 600자 이상(+10)")
	}
	if in.TrafficRefs > 0 {
		score += 20
		reasons = append(reasons, fmt.Sprintf("HTTP 트래픽 증거 %d건 결합(+20)", in.TrafficRefs))
	}

	switch strings.ToLower(strings.TrimSpace(in.Severity)) {
	case "critical":
		score += 10
		reasons = append(reasons, "심각도 critical(+10)")
	case "high":
		score += 5
		reasons = append(reasons, "심각도 high(+5)")
	}
	if strings.TrimSpace(in.Name) != "" {
		score += 5
	}
	if strings.TrimSpace(in.Summary) != "" && len([]rune(in.Summary)) >= 20 {
		score += 5
		reasons = append(reasons, "요약 20자 이상(+5)")
	}

	if score > 100 {
		score = 100
	}
	res.Confidence = score
	res.Reasons = reasons

	// Evidence-backed needs both: raw output/commands AND enough of it.
	if hasCommand && runes >= 120 && score >= 60 {
		res.Status = StatusEvidenceBacked
	}
	return res
}

// NormalizeTitle lowercases, strips punctuation and collapses whitespace so
// "SQL Injection in /login" and "sql injection  in /login!" collapse together.
func NormalizeTitle(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// DedupeKey is a stable short key for (class, normalized title, sorted assets).
func DedupeKey(class, name string, assetIDs []int64) string {
	ids := append([]int64(nil), assetIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var sb strings.Builder
	sb.WriteString(NormalizeTitle(class))
	sb.WriteString("|")
	sb.WriteString(NormalizeTitle(name))
	sb.WriteString("|")
	for i, id := range ids {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%d", id)
	}
	sum := sha1.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])[:16]
}
