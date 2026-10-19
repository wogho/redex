package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wogho/redex/db"
)

// OSINT module. Collectors are CLI wrappers (maigret/holehe/phoneinfoga/
// instaloader) plus a local InsightFace helper. Leads are stored as candidates,
// not findings: they stay "unverified" until a human/agent confirms them.
//
// Gating: a run is only allowed when the engagement's RoE record has
// person_osint_allowed = true. No engagement → no run.

var (
	ansiRE   = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	handleRE = regexp.MustCompile(`^[A-Za-z0-9._]{1,40}$`)
)

func osintRoot() string { return filepath.Join(osintDataDir(), "osint") }

// osintDataDir is the manager's data directory (see server.New).
func (s *Server) osintDataDir() string { return s.m.dir }

func osintDataDir() string {
	if v := strings.TrimSpace(os.Getenv("REDEX_OSINT_DIR")); v != "" {
		return v
	}
	return "data"
}

func (s *Server) faceScript() string {
	if v := strings.TrimSpace(os.Getenv("REDEX_FACE_SCRIPT")); v != "" {
		return v
	}
	return "/app/osint/face.py"
}

func (s *Server) scopeAllowsPerson(ctx context.Context, engagementID int64) (bool, error) {
	if engagementID <= 0 {
		return false, nil
	}
	return s.m.pg.ScopePersonOsintAllowed(ctx, engagementID)
}

// POST /api/osint/run {engagement_id, kind, value, limit}
func (s *Server) osintRun(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EngagementID int64  `json:"engagement_id"`
		Kind         string `json:"kind"`
		Value        string `json:"value"`
		Limit        int    `json:"limit"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	in.Value = strings.TrimSpace(in.Value)
	if in.Kind != "profile" && in.Value == "" {
		writeErr(w, 400, "value is required")
		return
	}
	allowed, err := s.scopeAllowsPerson(r.Context(), in.EngagementID)
	if err != nil {
		writeErr(w, 400, "engagement not found")
		return
	}
	if !allowed {
		writeErr(w, 403, "이 교전은 인물 조사가 허용되지 않았습니다(교전 범위에서 허용 필요)")
		return
	}
	if in.Limit <= 0 || in.Limit > 40 {
		in.Limit = 10
	}

	var candidates []*db.OsintCandidate
	switch in.Kind {
	case "handle":
		candidates, err = s.collectHandle(r.Context(), in)
	case "email":
		candidates, err = s.collectEmail(r.Context(), in)
	case "contact":
		candidates, err = s.collectContact(r.Context(), in)
	case "profile":
		candidates, err = s.collectProfile(r.Context(), in)
	default:
		writeErr(w, 400, "unknown kind")
		return
	}
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	if extra, xerr := s.collectExtended(r.Context(), in); xerr == nil {
		candidates = append(candidates, extra...)
	}
	writeJSON(w, 200, map[string]any{"candidates": candidates, "count": len(candidates)})
}

func (s *Server) saveCandidates(ctx context.Context, in any, list []db.OsintCandidate) ([]*db.OsintCandidate, error) {
	_ = in
	out := make([]*db.OsintCandidate, 0, len(list))
	for _, c := range list {
		row, err := s.m.pg.InsertOsintCandidate(ctx, c)
		if err != nil {
			return out, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Server) collectHandle(ctx context.Context, in struct {
	EngagementID int64  `json:"engagement_id"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Limit        int    `json:"limit"`
}) ([]*db.OsintCandidate, error) {
	if !handleRE.MatchString(in.Value) {
		return nil, errors.New("invalid handle")
	}
	out, _ := s.runCLI(ctx, 150*time.Second, "maigret", in.Value, "--no-recursion", "--timeout", "20", "--print-found")
	if extra, err := s.runCLI(ctx, 120*time.Second, "sherlock", in.Value, "--print-found", "--timeout", "20"); err == nil {
		out += "\n" + extra
	}
	clean := ansiRE.ReplaceAllString(out, "")
	cands := make([]db.OsintCandidate, 0, 32)
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "http") {
			continue
		}
		title := line
		if i := strings.Index(line, "http"); i > 0 {
			title = strings.TrimRight(line[:i], " :\t-")
		}
		conf := 45
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: "handle", Value: in.Value,
			Source: "maigret", Detail: title, Evidence: truncate(line, 500),
			Confidence: conf, Status: "unverified",
		})
		if len(cands) >= in.Limit*3 {
			break
		}
	}
	if len(cands) == 0 {
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: "handle", Value: in.Value,
			Source: "maigret", Detail: "no public profile matched", Evidence: truncate(clean, 800),
			Confidence: 10, Status: "unverified",
		})
	}
	return s.saveCandidates(ctx, in, cands)
}

func (s *Server) collectEmail(ctx context.Context, in struct {
	EngagementID int64  `json:"engagement_id"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Limit        int    `json:"limit"`
}) ([]*db.OsintCandidate, error) {
	out, err := s.runCLI(ctx, 150*time.Second, "holehe", in.Value, "--only-used")
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, err
	}
	clean := ansiRE.ReplaceAllString(out, "")
	cands := make([]db.OsintCandidate, 0, 16)
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "[+]") {
			continue
		}
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: "email", Value: in.Value,
			Source: "holehe", Detail: truncate(line, 300), Evidence: truncate(line, 500),
			Confidence: 50, Status: "unverified",
		})
		if len(cands) >= in.Limit*3 {
			break
		}
	}
	if len(cands) == 0 {
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: "email", Value: in.Value,
			Source: "holehe", Detail: "no registration found", Evidence: truncate(clean, 800),
			Confidence: 10, Status: "unverified",
		})
	}
	return s.saveCandidates(ctx, in, cands)
}

func (s *Server) collectContact(ctx context.Context, in struct {
	EngagementID int64  `json:"engagement_id"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Limit        int    `json:"limit"`
}) ([]*db.OsintCandidate, error) {
	out, err := s.runCLI(ctx, 120*time.Second, "phoneinfoga", "scan", "-n", in.Value)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, err
	}
	clean := ansiRE.ReplaceAllString(out, "")
	cands := []db.OsintCandidate{{
		EngagementID: in.EngagementID, Kind: "contact", Value: in.Value,
		Source: "phoneinfoga", Detail: truncate(firstNonEmptyLine(clean), 300),
		Evidence: truncate(clean, 1200), Confidence: 35, Status: "unverified",
	}}
	return s.saveCandidates(ctx, in, cands)
}

func (s *Server) collectProfile(ctx context.Context, in struct {
	EngagementID int64  `json:"engagement_id"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Limit        int    `json:"limit"`
}) ([]*db.OsintCandidate, error) {
	if !handleRE.MatchString(in.Value) {
		return nil, errors.New("invalid handle")
	}
	dir := filepath.Join(s.osintDataDir(), "osint", in.Value)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	args := []string{"--no-videos", "--no-metadata-json", "--dirname-pattern=.", "--count", strconv.Itoa(in.Limit)}
	if session := s.osintSecret("instaloader.session"); fileExists(session) {
		args = append(args, "--sessionfile", session)
	}
	args = append(args, in.Value)
	_, err := s.runCLIInDir(ctx, 240*time.Second, dir, "instaloader", args...)
	if err != nil {
		return nil, fmt.Errorf("profile collection failed: %w", err)
	}
	files := listImages(dir)
	cands := []db.OsintCandidate{{
		EngagementID: in.EngagementID, Kind: "profile", Value: in.Value,
		Source: "instaloader", Detail: fmt.Sprintf("%d images collected", len(files)),
		Evidence: dir, Confidence: 30, Status: "unverified",
	}}
	// Match collected images against enrolled embeddings.
	matches := 0
	for _, f := range files {
		faces, ferr := s.faceEmbeddings(ctx, f)
		if ferr != nil || len(faces) == 0 {
			continue
		}
		best, score, label := s.bestFaceMatch(ctx, faces)
		if best == 0 || score < 0.45 {
			continue
		}
		matches++
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: "face", Value: in.Value,
			Source: "instaloader+insightface", Detail: fmt.Sprintf("match %q score %.3f", label, score),
			Evidence: f, Confidence: int(math.Round(score * 100)), Status: "evidence-backed",
		})
		if matches >= in.Limit {
			break
		}
	}
	return s.saveCandidates(ctx, in, cands)
}

// --- face index/search ---

type faceHit struct {
	BBox      []float64 `json:"bbox"`
	DetScore  float64   `json:"det_score"`
	Embedding []float32 `json:"embedding"`
}

func (s *Server) faceEmbeddings(ctx context.Context, path string) ([]faceHit, error) {
	c, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, "python3", s.faceScript(), path)
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var hits []faceHit
	if err := json.Unmarshal(raw, &hits); err != nil {
		return nil, err
	}
	return hits, nil
}

func (s *Server) bestFaceMatch(ctx context.Context, hits []faceHit) (int64, float64, string) {
	faces, err := s.m.pg.ListFaces(ctx)
	if err != nil || len(faces) == 0 {
		return 0, 0, ""
	}
	bestID, bestScore, bestLabel := int64(0), -1.0, ""
	for _, h := range hits {
		for _, f := range faces {
			score := cosine(h.Embedding, f.Embedding)
			if score > bestScore {
				bestID, bestScore, bestLabel = f.ID, score, f.Label
			}
		}
		if bestScore >= 0.65 {
			break
		}
	}
	return bestID, bestScore, bestLabel
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// POST /api/osint/face/index (multipart: image, label, source)
func (s *Server) osintFaceIndex(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, 400, "invalid multipart form")
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		writeErr(w, 400, "label is required")
		return
	}
	path, cleanup, err := saveOsintUpload(r, "image", s.osintDataDir())
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	defer cleanup()
	hits, err := s.faceEmbeddings(r.Context(), path)
	if err != nil {
		writeErr(w, 502, "face engine error: "+err.Error())
		return
	}
	if len(hits) == 0 {
		writeErr(w, 422, "no face found in image")
		return
	}
	best := hits[0]
	for _, h := range hits[1:] {
		if h.DetScore > best.DetScore {
			best = h
		}
	}
	id, err := s.m.pg.InsertFace(r.Context(), label, r.FormValue("source"), best.Embedding)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "label": label, "faces": len(hits), "det_score": best.DetScore})
}

// POST /api/osint/face/search (multipart: image)
func (s *Server) osintFaceSearch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, 400, "invalid multipart form")
		return
	}
	path, cleanup, err := saveOsintUpload(r, "image", s.osintDataDir())
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	defer cleanup()
	hits, err := s.faceEmbeddings(r.Context(), path)
	if err != nil {
		writeErr(w, 502, "face engine error: "+err.Error())
		return
	}
	if len(hits) == 0 {
		writeJSON(w, 200, map[string]any{"matches": []any{}})
		return
	}
	faces, err := s.m.pg.ListFaces(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	type match struct {
		ID    int64   `json:"id,string"`
		Label string  `json:"label"`
		Score float64 `json:"score"`
	}
	var matches []match
	for _, h := range hits {
		for _, f := range faces {
			sc := cosine(h.Embedding, f.Embedding)
			if sc >= 0.4 {
				matches = append(matches, match{ID: f.ID, Label: f.Label, Score: sc})
			}
		}
	}
	// sort desc, keep top 10
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	if len(matches) > 10 {
		matches = matches[:10]
	}
	writeJSON(w, 200, map[string]any{"matches": matches})
}

// GET /api/osint/faces
func (s *Server) osintFacesList(w http.ResponseWriter, r *http.Request) {
	faces, err := s.m.pg.ListFaces(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"faces": faces})
}

// DELETE /api/osint/faces/{id}
func (s *Server) osintFaceDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	if err := s.m.pg.DeleteFace(r.Context(), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// GET /api/osint/candidates?engagement_id=&kind=
func (s *Server) osintCandidates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var engagementID int64
	if v := strings.TrimSpace(q.Get("engagement_id")); v != "" {
		engagementID, _ = strconv.ParseInt(v, 10, 64)
	}
	rows, err := s.m.pg.ListOsintCandidates(r.Context(), engagementID, strings.TrimSpace(q.Get("kind")))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": rows})
}

// --- helpers ---

func (s *Server) runCLI(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	return s.runCLIInDir(ctx, timeout, "", name, args...)
}

func (s *Server) runCLIInDir(ctx context.Context, timeout time.Duration, dir, name string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(c, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func saveOsintUpload(r *http.Request, field, base string) (string, func(), error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return "", func() {}, errors.New("image field is required")
	}
	dir := filepath.Join(base, "osint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		file.Close()
		return "", func() {}, err
	}
	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		file.Close()
		return "", func() {}, err
	}
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		file.Close()
		os.Remove(tmp.Name())
		return "", func() {}, err
	}
	tmp.Close()
	file.Close()
	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func listImages(dir string) []string {
	var out []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".webp":
			out = append(out, path)
		}
		return nil
	})
	return out
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// PATCH /api/osint/candidates/{id} {"status":"confirmed|rejected|unverified"}
func (s *Server) osintCandidateUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	switch body.Status {
	case "confirmed", "rejected", "unverified", "evidence-backed":
	default:
		writeErr(w, 400, "bad status")
		return
	}
	if err := s.m.pg.UpdateOsintCandidateStatus(r.Context(), id, body.Status); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "status": body.Status})
}

// GET /api/osint/evidence/{sha} — download a stored evidence snapshot.
func (s *Server) osintEvidence(w http.ResponseWriter, r *http.Request) {
	sha := strings.ToLower(r.PathValue("sha"))
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(sha) {
		writeErr(w, 400, "bad sha")
		return
	}
	path := filepath.Join(s.evidenceDir(), sha+".txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, 404, "evidence not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, sha[:12]))
	_, _ = w.Write(raw)
}
