package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wogho/redex/config"
	"github.com/wogho/redex/guard"
)

type policyView struct {
	Path       string `json:"path"`
	Configured bool   `json:"configured"`
	Exists     bool   `json:"exists"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

func policyPath() string { return strings.TrimSpace(os.Getenv(config.PolicyFileEnv)) }

// GET /api/policy — current policy file (path, raw JSON, parse error).
func (s *Server) policyGet(w http.ResponseWriter, _ *http.Request) {
	v := policyView{Path: policyPath(), Configured: policyPath() != ""}
	if !v.Configured {
		writeJSON(w, 200, v)
		return
	}
	raw, err := os.ReadFile(v.Path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, 200, v)
			return
		}
		v.Error = err.Error()
		writeJSON(w, 200, v)
		return
	}
	v.Exists = true
	v.Content = string(raw)
	if err := guard.ValidatePolicy(raw); err != nil {
		v.Error = err.Error()
	}
	writeJSON(w, 200, v)
}

// PUT /api/policy {"content": "..."} — validate, write atomically, apply live.
func (s *Server) policyPut(w http.ResponseWriter, r *http.Request) {
	path := policyPath()
	if path == "" {
		writeErr(w, 400, "REDEX_POLICY_FILE is not set; configure it and restart")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := guard.ValidatePolicy([]byte(body.Content)); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tmp, err := os.CreateTemp(dir, ".policy-*.tmp")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(body.Content); err != nil {
		tmp.Close()
		writeErr(w, 500, err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := os.Rename(tmpName, path); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := guard.LoadPolicy(path); err != nil {
		// File is written, but the in-process policy failed to apply: report it.
		writeErr(w, 500, "saved but failed to apply: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "path": path})
}

// POST /api/policy/test {"tool":"Bash","command":"rm -rf /"} — dry-run check.
func (s *Server) policyTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tool    string `json:"tool"`
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.Tool) == "" {
		body.Tool = "Bash"
	}
	blocked, reason := guard.Check(body.Tool, body.Command)
	writeJSON(w, 200, map[string]any{"blocked": blocked, "reason": reason})
}
