package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ATT&CK usage: turn recorded findings into technique counts, so the coverage
// page shows what this engagement actually exercised rather than a static list.

type attackTechnique struct {
	ID       string   `json:"id"`
	Tactic   string   `json:"tactic"`
	Count    int      `json:"count"`
	Examples []string `json:"examples"`
}

var attackRules = []struct {
	Keywords  []string
	Technique string
	Tactic    string
}{
	{[]string{"sqli", "sql injection", "sql 삽입", "injection"}, "T1190", "initial-access"},
	{[]string{"xss", "cross-site scripting", "스크립트"}, "T1059.007", "execution"},
	{[]string{"ssrf", "server-side request"}, "T1190", "initial-access"},
	{[]string{"rce", "command injection", "명령 실행", "code execution"}, "T1059", "execution"},
	{[]string{"lfi", "path traversal", "directory traversal", "파일 읽기"}, "T1005", "collection"},
	{[]string{"auth bypass", "authentication bypass", "인증 우회"}, "T1078", "initial-access"},
	{[]string{"default credential", "weak password", "기본 비밀번호", "약한 비밀번호"}, "T1078.001", "initial-access"},
	{[]string{"brute", "spray", "무차별", "스프레이"}, "T1110", "credential-access"},
	{[]string{"idor", "bola", "object reference", "권한 우회"}, "T1078", "initial-access"},
	{[]string{"kerberoast"}, "T1558.003", "credential-access"},
	{[]string{"as-rep", "asrep"}, "T1558.004", "credential-access"},
	{[]string{"lsass", "mimikatz", "pypykatz"}, "T1003.001", "credential-access"},
	{[]string{"ntds", "secretsdump", "dcsync"}, "T1003.003", "credential-access"},
	{[]string{"smb", "psexec", "wmiexec"}, "T1021.002", "lateral-movement"},
	{[]string{"ssh", "sshd"}, "T1021.004", "lateral-movement"},
	{[]string{" phishing", "피싱"}, "T1566", "initial-access"},
	{[]string{"open redirect", "리다이렉트"}, "T1204", "execution"},
	{[]string{"subdomain", "dns", "서브도메인"}, "T1590", "reconnaissance"},
	{[]string{"port", "scan", "포트", "스캔"}, "T1595", "reconnaissance"},
	{[]string{"版本", "fingerprint", "technology", "기술 스택"}, "T1592", "reconnaissance"},
	{[]string{"s3", "bucket", "cloud", "클라우드"}, "T1530", "collection"},
	{[]string{"deserial", "역직렬화"}, "T1059", "execution"},
	{[]string{"xxe", "xml external"}, "T1190", "initial-access"},
	{[]string{"upload", "webshell", "웹쉘", "업로드"}, "T1505.003", "persistence"},
	{[]string{"race", "레이스"}, "T1190", "initial-access"},
}

func techniqueFor(text string) (id, tactic string, ok bool) {
	t := strings.ToLower(text)
	for _, r := range attackRules {
		for _, k := range r.Keywords {
			if strings.Contains(t, k) {
				return r.Technique, r.Tactic, true
			}
		}
	}
	return "", "", false
}

// GET /api/attack/usage?task_id=
func (s *Server) attackUsage(w http.ResponseWriter, r *http.Request) {
	var taskID int64
	if v := strings.TrimSpace(r.URL.Query().Get("task_id")); v != "" {
		if n, err := parseInt64(v); err == nil {
			taskID = n
		}
	}
	rows, err := s.m.pg.QueryContext(r.Context(), `
SELECT id, COALESCE(name,''), COALESCE(vulnclass,''), COALESCE(severity,'')
  FROM findings
 WHERE ($1 = 0 OR task_id = $1)
 ORDER BY created_at DESC
 LIMIT 2000`, taskID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	byTech := map[string]*attackTechnique{}
	total := 0
	for rows.Next() {
		var id int64
		var name, class, sev string
		if err := rows.Scan(&id, &name, &class, &sev); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		total++
		tid, tactic, ok := techniqueFor(name + " " + class)
		if !ok {
			continue
		}
		t := byTech[tid]
		if t == nil {
			t = &attackTechnique{ID: tid, Tactic: tactic}
			byTech[tid] = t
		}
		t.Count++
		if len(t.Examples) < 3 {
			t.Examples = append(t.Examples, name)
		}
	}
	out := make([]*attackTechnique, 0, len(byTech))
	for _, t := range byTech {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	writeJSON(w, 200, map[string]any{"techniques": out, "findings": total})
}

func parseInt64(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
