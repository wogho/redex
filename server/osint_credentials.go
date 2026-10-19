package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wogho/redex/db"
)

// OSINT credential handling. Secrets live under <data>/secrets/osint with 0600
// and are never returned by the API — only their configured state is.

func (s *Server) osintSecretDir() string {
	dir := filepath.Join(s.m.dir, "secrets", "osint")
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

func (s *Server) osintSecret(name string) string { return filepath.Join(s.osintSecretDir(), name) }

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// GET /api/osint/credentials
func (s *Server) osintCredentials(w http.ResponseWriter, _ *http.Request) {
	user := ""
	if raw, err := os.ReadFile(s.osintSecret("instaloader.user")); err == nil {
		user = strings.TrimSpace(string(raw))
	}
	writeJSON(w, 200, map[string]any{
		"hibp_configured":        fileExists(s.osintSecret("hibp.key")),
		"ghunt_configured":       fileExists(s.osintSecret("ghunt_cookies.json")),
		"instaloader_configured": fileExists(s.osintSecret("instaloader.session")),
		"instaloader_user":       user,
	})
}

// PUT /api/osint/credentials/hibp {"key":"..."}
func (s *Server) osintSetHIBP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	body.Key = strings.TrimSpace(body.Key)
	if body.Key == "" {
		writeErr(w, 400, "key is required")
		return
	}
	if err := os.WriteFile(s.osintSecret("hibp.key"), []byte(body.Key), 0o600); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "hibp_configured": true})
}

// POST /api/osint/credentials/ghunt (multipart: file)
func (s *Server) osintUploadGhunt(w http.ResponseWriter, r *http.Request) {
	if err := s.saveSecretUpload(r, "file", "ghunt_cookies.json"); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "ghunt_configured": true})
}

// POST /api/osint/credentials/instaloader/session (multipart: file)
func (s *Server) osintUploadInstaloaderSession(w http.ResponseWriter, r *http.Request) {
	if err := s.saveSecretUpload(r, "file", "instaloader.session"); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "instaloader_configured": true})
}

// POST /api/osint/credentials/instaloader/login {"username","password"}
// Runs `instaloader --login` with the password on stdin and stores the resulting
// session file. Never logs or returns the password.
func (s *Server) osintInstaloaderLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !handleRE.MatchString(body.Username) || body.Password == "" {
		writeErr(w, 400, "username/password required")
		return
	}
	session := s.osintSecret("instaloader.session")
	cmd := exec.Command("instaloader", "--login", body.Username, "--sessionfile", session)
	cmd.Stdin = strings.NewReader(body.Password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, 502, "login failed: "+truncate(string(out), 300))
		return
	}
	if err := os.WriteFile(s.osintSecret("instaloader.user"), []byte(body.Username), 0o600); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "instaloader_configured": true, "instaloader_user": body.Username})
}

// POST /api/osint/credentials/instaloader/logout
func (s *Server) osintInstaloaderLogout(w http.ResponseWriter, _ *http.Request) {
	_ = os.Remove(s.osintSecret("instaloader.session"))
	_ = os.Remove(s.osintSecret("instaloader.user"))
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) saveSecretUpload(r *http.Request, field, name string) error {
	file, _, err := r.FormFile(field)
	if err != nil {
		// allow raw body upload too
		raw, rerr := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if rerr != nil || len(raw) == 0 {
			return err
		}
		return os.WriteFile(s.osintSecret(name), raw, 0o600)
	}
	defer file.Close()
	dst, err := os.OpenFile(s.osintSecret(name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, io.LimitReader(file, 8<<20))
	return err
}

// POST /api/osint/hibp {"email":"..."} — breach lookup with the stored key.
func (s *Server) osintHIBP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	body.Email = strings.TrimSpace(body.Email)
	if body.Email == "" {
		writeErr(w, 400, "email is required")
		return
	}
	keyRaw, err := os.ReadFile(s.osintSecret("hibp.key"))
	if err != nil {
		writeErr(w, 400, "HIBP key is not configured")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		"https://haveibeenpwned.com/api/v3/breachedaccount/"+strings.TrimSpace(body.Email)+"?truncateResponse=false", nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	req.Header.Set("hibp-api-key", strings.TrimSpace(string(keyRaw)))
	req.Header.Set("user-agent", "Redex-OSINT")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		writeJSON(w, 200, map[string]any{"breaches": []any{}, "count": 0})
		return
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		writeErr(w, 401, "HIBP key rejected")
		return
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		writeErr(w, 429, "HIBP rate limited")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeErr(w, 502, "HIBP status "+resp.Status)
		return
	}
	var breaches []struct {
		Name        string   `json:"Name"`
		Title       string   `json:"Title"`
		Domain      string   `json:"Domain"`
		BreachDate  string   `json:"BreachDate"`
		PwnCount    int      `json:"PwnCount"`
		DataClasses []string `json:"DataClasses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&breaches); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	out := make([]*db.OsintCandidate, 0, len(breaches))
	for _, b := range breaches {
		conf := 60
		row, err := s.m.pg.InsertOsintCandidate(r.Context(), db.OsintCandidate{
			Kind: "email", Value: body.Email, Source: "hibp",
			Detail:     b.Title + " (" + b.Domain + ", " + b.BreachDate + ", " + itoa(b.PwnCount) + " records)",
			Evidence:   strings.Join(b.DataClasses, ", "),
			Confidence: conf, Status: "unverified",
		})
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, row)
	}
	writeJSON(w, 200, map[string]any{"breaches": breaches, "candidates": out, "count": len(out)})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
