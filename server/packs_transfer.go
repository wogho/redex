package server

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---- progress jobs (SSE-friendly) ----

type packJob struct {
	ID    string `json:"id"`
	Pack  string `json:"pack"`
	Step  string `json:"step"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
	DoneF bool   `json:"finished"`

	mu sync.Mutex
}

func (j *packJob) set(step string, done, total int64) {
	j.mu.Lock()
	j.Step, j.Done, j.Total = step, done, total
	j.mu.Unlock()
}

func (j *packJob) finish(err error) {
	j.mu.Lock()
	j.DoneF = true
	if err != nil {
		j.Error = err.Error()
	}
	j.mu.Unlock()
}

func (j *packJob) snapshot() packJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	return packJob{ID: j.ID, Pack: j.Pack, Step: j.Step, Done: j.Done, Total: j.Total, Error: j.Error, DoneF: j.DoneF}
}

var packJobStore sync.Map // id -> *packJob

func (s *Server) newPackJob(pack string) *packJob {
	j := &packJob{ID: fmt.Sprintf("job-%d", time.Now().UnixNano()), Pack: pack, Step: "queued"}
	packJobStore.Store(j.ID, j)
	return j
}

// GET /api/packs/jobs/{id} — JSON progress
func (s *Server) packJobGet(w http.ResponseWriter, r *http.Request) {
	v, ok := packJobStore.Load(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, v.(*packJob).snapshot())
}

// GET /api/packs/jobs/{id}/stream — SSE progress
func (s *Server) packJobStream(w http.ResponseWriter, r *http.Request) {
	v, ok := packJobStore.Load(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "job not found")
		return
	}
	j := v.(*packJob)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, _ := w.(http.Flusher)
	for {
		snap := j.snapshot()
		raw, _ := json.Marshal(snap)
		fmt.Fprintf(w, "data: %s\n\n", raw)
		if fl != nil {
			fl.Flush()
		}
		if snap.DoneF {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// ---- helpers ----

func verifyFileSHA(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want)
}

func dirHash(root string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, p := range files {
		rel, _ := filepath.Rel(root, p)
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(raw)
		fmt.Fprintf(h, "%s:%s\n", filepath.ToSlash(rel), hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractPackTar(archive, dest string) error {
	list, err := exec.Command("tar", "--zstd", "-tf", archive).Output()
	if err != nil {
		return fmt.Errorf("invalid archive")
	}
	for _, name := range strings.Split(strings.TrimSpace(string(list)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("unsafe archive path: %s", name)
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return exec.Command("tar", "--zstd", "-xf", archive, "-C", dest).Run()
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return os.Chmod(target, info.Mode().Perm())
	})
}

// mirrorURLs returns canonical + optional mirror URLs.
func mirrorURLs(m packManifestEntry) []string {
	out := []string{m.URL}
	if base := strings.TrimSpace(os.Getenv("REDEX_PACK_MIRROR")); base != "" {
		name := filepath.Base(m.URL)
		out = append(out, strings.TrimRight(base, "/")+"/"+name)
	}
	return out
}

func (s *Server) downloadPack(ctx context.Context, m packManifestEntry, dst string, j *packJob) error {
	limit := m.Size*11/10 + 1<<20
	if limit <= 1<<20 || limit > hardMaxDownload {
		limit = hardMaxDownload
	}
	var lastErr error
	for _, raw := range mirrorURLs(m) {
		if j != nil {
			j.set("download", 0, m.Size)
		}
		u, hdrs, rerr := resolvePackDownload(raw)
		if rerr != nil {
			lastErr = rerr
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		for k, v := range hdrs {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("download status %s", resp.Status)
			continue
		}
		out, err := os.Create(dst)
		if err != nil {
			resp.Body.Close()
			return err
		}
		h := sha256.New()
		var done int64
		buf := make([]byte, 256<<10)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					out.Close()
					resp.Body.Close()
					return werr
				}
				h.Write(buf[:n])
				done += int64(n)
				if j != nil {
					j.set("download", done, m.Size)
				}
				if done > limit {
					out.Close()
					resp.Body.Close()
					lastErr = fmt.Errorf("download exceeds limit")
					goto nextURL
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				lastErr = rerr
				break
			}
		}
		out.Close()
		resp.Body.Close()
		if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), m.SHA256) {
			lastErr = fmt.Errorf("checksum mismatch")
			continue
		}
		return nil
	nextURL:
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download failed")
	}
	return lastErr
}

// POST /api/packs/{id}/install-local — upload a .tar.zst, verify hash, install.
func (s *Server) packInstallLocal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := packByID(id)
	if !ok {
		writeErr(w, 404, "unknown pack")
		return
	}
	reg := s.loadPackRegistry()
	if err := os.MkdirAll(reg.Settings.Dir, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	var src string
	var cleanup func()
	if err := r.ParseMultipartForm(2 << 30); err == nil && r.MultipartForm != nil {
		file, _, ferr := r.FormFile("file")
		if ferr != nil {
			writeErr(w, 400, "file field is required")
			return
		}
		tmp, err := os.CreateTemp(reg.Settings.Dir, "upload-*.tar.zst")
		if err != nil {
			file.Close()
			writeErr(w, 500, err.Error())
			return
		}
		if _, err := io.Copy(tmp, io.LimitReader(file, hardMaxDownload)); err != nil {
			tmp.Close()
			file.Close()
			writeErr(w, 500, err.Error())
			return
		}
		tmp.Close()
		file.Close()
		src = tmp.Name()
		cleanup = func() { _ = os.Remove(src) }
	} else {
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		src = strings.TrimSpace(body.Path)
		cleanup = func() {}
	}
	defer cleanup()
	if src == "" {
		writeErr(w, 400, "file upload or path is required")
		return
	}
	if !verifyFileSHA(src, m.SHA256) {
		writeErr(w, 422, "checksum mismatch")
		return
	}
	dest := filepath.Join(reg.Settings.Dir, id, m.Version)
	_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, id))
	if err := extractPackTar(src, dest); err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	reg.Packs[id] = packState{Version: m.Version, SHA256: m.SHA256, Bytes: packBytes(dest), InstalledAt: time.Now().UTC(), LastUsedAt: time.Now().UTC()}
	s.evictIfNeeded(&reg)
	if err := s.savePackRegistry(reg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id, "version": m.Version, "bytes": reg.Packs[id].Bytes})
}

// GET /api/packs/export?ids=a,b (empty = all installed)
func (s *Server) packsExport(w http.ResponseWriter, r *http.Request) {
	reg := s.loadPackRegistry()
	ids := []string{}
	if q := strings.TrimSpace(r.URL.Query().Get("ids")); q != "" {
		ids = strings.Split(q, ",")
	} else {
		for id := range reg.Packs {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeErr(w, 400, "no packs to export")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="redex-packs.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()

	manifest := map[string]any{}
	sums := map[string]string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		ps, ok := reg.Packs[id]
		if !ok {
			continue
		}
		src := filepath.Join(reg.Settings.Dir, id, ps.Version)
		h, err := dirHash(src)
		if err != nil {
			continue
		}
		sums[id] = h
		manifest[id] = map[string]string{"version": ps.Version, "sha256": ps.SHA256, "dir_sha256": h}
		err = filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(reg.Settings.Dir, p)
			fw, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(fw, f)
			return err
		})
		if err != nil {
			continue
		}
	}
	mw, _ := zw.Create("redex-packs.json")
	raw, _ := json.MarshalIndent(map[string]any{"packs": manifest, "dir_sha256": sums}, "", "  ")
	_, _ = mw.Write(raw)
}

// POST /api/packs/import (multipart zip)
func (s *Server) packsImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 30); err != nil {
		writeErr(w, 400, "invalid multipart form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "file field is required")
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "packs-import-*.zip")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, io.LimitReader(file, hardMaxDownload)); err != nil {
		tmp.Close()
		writeErr(w, 500, err.Error())
		return
	}
	tmp.Close()
	zr, err := zip.OpenReader(tmpName)
	if err != nil {
		writeErr(w, 400, "invalid zip")
		return
	}
	defer zr.Close()
	stage, err := os.MkdirTemp("", "packs-stage-*")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer os.RemoveAll(stage)
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			writeErr(w, 422, "unsafe zip path: "+name)
			return
		}
		dst := filepath.Join(stage, name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(dst, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		rc, err := f.Open()
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			writeErr(w, 500, err.Error())
			return
		}
		_, err = io.Copy(out, io.LimitReader(rc, 1<<30))
		out.Close()
		rc.Close()
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	raw, err := os.ReadFile(filepath.Join(stage, "redex-packs.json"))
	if err != nil {
		writeErr(w, 400, "missing redex-packs.json")
		return
	}
	var meta struct {
		Packs map[string]struct {
			Version   string `json:"version"`
			SHA256    string `json:"sha256"`
			DirSHA256 string `json:"dir_sha256"`
		} `json:"packs"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		writeErr(w, 400, "bad redex-packs.json")
		return
	}
	reg := s.loadPackRegistry()
	installed := []string{}
	for id, m := range meta.Packs {
		src := filepath.Join(stage, id, m.Version)
		h, err := dirHash(src)
		if err != nil || !strings.EqualFold(h, m.DirSHA256) {
			writeErr(w, 422, "integrity check failed for "+id)
			return
		}
		dst := filepath.Join(reg.Settings.Dir, id, m.Version)
		_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, id))
		if err := copyTree(src, dst); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		reg.Packs[id] = packState{Version: m.Version, SHA256: m.SHA256, Bytes: packBytes(dst), InstalledAt: time.Now().UTC(), LastUsedAt: time.Now().UTC()}
		installed = append(installed, id)
	}
	s.evictIfNeeded(&reg)
	if err := s.savePackRegistry(reg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "installed": installed})
}

// migratePacks moves installed packs to a new directory (copy → verify → swap).
func (s *Server) migratePacks(reg *packRegistry, newDir string) error {
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	for id, ps := range reg.Packs {
		src := filepath.Join(reg.Settings.Dir, id, ps.Version)
		dst := filepath.Join(newDir, id, ps.Version)
		if err := copyTree(src, dst); err != nil {
			return err
		}
		hs, err1 := dirHash(src)
		hd, err2 := dirHash(dst)
		if err1 != nil || err2 != nil || hs != hd {
			_ = os.RemoveAll(filepath.Join(newDir, id))
			return fmt.Errorf("migration verify failed for %s", id)
		}
		reg.Packs[id] = ps
	}
	old := reg.Settings.Dir
	reg.Settings.Dir = newDir
	if err := s.savePackRegistry(*reg); err != nil {
		return err
	}
	for id := range reg.Packs {
		_ = os.RemoveAll(filepath.Join(old, id))
	}
	return nil
}
