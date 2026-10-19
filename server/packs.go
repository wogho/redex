package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Source packs: data fetched on demand from GitHub Releases (CDN), verified by
// SHA256, then enabled per installation. No self-hosted pack server.

type packManifestEntry struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	License     string `json:"license"`
	Description string `json:"description"`
	Upstream    string `json:"upstream"`
}

type packState struct {
	Version     string    `json:"version"`
	SHA256      string    `json:"sha256"`
	InstalledAt time.Time `json:"installed_at"`
	Enabled     bool      `json:"enabled"`
}

type packView struct {
	packManifestEntry
	Installed bool `json:"installed"`
	Enabled   bool `json:"enabled"`
}

// defaultPackManifest is pinned in code: a compromised repo branch cannot
// redirect downloads. The SHA256 is the trust anchor.
var defaultPackManifest = []packManifestEntry{
	{
		ID: "whatsmyname", Version: "2026.10.1",
		URL:     "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/whatsmyname-2026.10.1.tar.zst",
		SHA256:  "9a94eb04d58b0dd50fc91cbd9b20fea85cf1b9450c8bea92299241a2b80673e1",
		Size:    15469,
		License: "CC-BY-SA-4.0", Upstream: "https://github.com/WebBreacher/WhatsMyName",
		Description: "WhatsMyName site list (692 sites)",
	},
}

func (s *Server) packDir() string { return filepath.Join(s.m.dir, "packs") }

func (s *Server) packState() map[string]packState {
	out := map[string]packState{}
	raw, err := os.ReadFile(filepath.Join(s.packDir(), "state.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func (s *Server) savePackState(st map[string]packState) error {
	if err := os.MkdirAll(s.packDir(), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(filepath.Join(s.packDir(), "state.json"), raw, 0o600)
}

// PackEnabled reports whether a pack is installed and enabled.
func (s *Server) PackEnabled(id string) bool {
	st, ok := s.packState()[id]
	return ok && st.Enabled
}

// PackFilePath returns <packs>/<id>/<version>/<name> if the pack is enabled.
func (s *Server) PackFilePath(id, name string) (string, bool) {
	st, ok := s.packState()[id]
	if !ok || !st.Enabled {
		return "", false
	}
	p := filepath.Join(s.packDir(), id, st.Version, name)
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// GET /api/packs
func (s *Server) packsList(w http.ResponseWriter, _ *http.Request) {
	st := s.packState()
	out := make([]packView, 0, len(defaultPackManifest))
	for _, m := range defaultPackManifest {
		v := packView{packManifestEntry: m}
		if ps, ok := st[m.ID]; ok {
			v.Installed = true
			v.Enabled = ps.Enabled
		}
		out = append(out, v)
	}
	writeJSON(w, 200, map[string]any{"packs": out})
}

func (s *Server) packByID(id string) (packManifestEntry, bool) {
	for _, m := range defaultPackManifest {
		if m.ID == id {
			return m, true
		}
	}
	return packManifestEntry{}, false
}

// POST /api/packs/{id}/install — download, verify sha256, extract.
func (s *Server) packInstall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := s.packByID(id)
	if !ok {
		writeErr(w, 404, "unknown pack")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, m.URL, nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, 502, fmt.Sprintf("download status %s", resp.Status))
		return
	}
	tmp, err := os.CreateTemp(s.packDir(), "pack-*.tar.zst")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, 64<<20)); err != nil {
		tmp.Close()
		writeErr(w, 502, err.Error())
		return
	}
	tmp.Close()
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, m.SHA256) {
		writeErr(w, 422, "checksum mismatch")
		return
	}
	if err := os.MkdirAll(s.packDir(), 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	dest := filepath.Join(s.packDir(), id, m.Version)
	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// List first: reject absolute paths / traversal before extracting.
	list, err := exec.Command("tar", "--zstd", "-tf", tmp.Name()).Output()
	if err != nil {
		writeErr(w, 500, "invalid archive")
		return
	}
	for _, name := range strings.Split(strings.TrimSpace(string(list)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			writeErr(w, 422, "unsafe archive path: "+name)
			return
		}
	}
	if err := exec.Command("tar", "--zstd", "-xf", tmp.Name(), "-C", dest).Run(); err != nil {
		writeErr(w, 500, "extract failed: "+err.Error())
		return
	}
	st := s.packState()
	st[id] = packState{Version: m.Version, SHA256: m.SHA256, InstalledAt: time.Now().UTC(), Enabled: false}
	if err := s.savePackState(st); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id, "version": m.Version, "sha256": got})
}

// POST /api/packs/{id}/enable and /disable
func (s *Server) packToggle(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		st := s.packState()
		ps, ok := st[id]
		if !ok {
			writeErr(w, 400, "pack is not installed")
			return
		}
		ps.Enabled = enabled
		st[id] = ps
		if err := s.savePackState(st); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "id": id, "enabled": enabled})
	}
}

// DELETE /api/packs/{id}
func (s *Server) packDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st := s.packState()
	ps, ok := st[id]
	if !ok {
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	_ = os.RemoveAll(filepath.Join(s.packDir(), id))
	delete(st, id)
	if err := s.savePackState(st); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	_ = ps
	writeJSON(w, 200, map[string]any{"ok": true})
}

var errPackMissing = errors.New("pack not enabled")
