package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Source packs v2: three states (installed × enabled), pinning, byte budget with
// LRU eviction, and a configurable storage directory. Downloads come only from
// pinned GitHub Release URLs; every install is SHA256-verified before extract.

type packManifestEntry struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	License     string `json:"license"`
	Description string `json:"description"`
	Upstream    string `json:"upstream"`
	Consent     bool   `json:"consent_required"`
}

type packState struct {
	Version     string    `json:"version"`
	SHA256      string    `json:"sha256"`
	Enabled     bool      `json:"enabled"`
	Pinned      bool      `json:"pinned"`
	Consent     bool      `json:"consent"`
	Bytes       int64     `json:"bytes"`
	InstalledAt time.Time `json:"installed_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

type packSettings struct {
	Dir                 string `json:"dir"`
	MaxBytes            int64  `json:"max_bytes"`
	AutoEvict           bool   `json:"auto_evict"`
	AutoDeleteOnDisable bool   `json:"auto_delete_on_disable"`
}

type packRegistry struct {
	Packs    map[string]packState `json:"packs"`
	Settings packSettings         `json:"settings"`
}

const defaultMaxBytes int64 = 20 << 30 // 20 GiB
const hardMaxDownload int64 = 2 << 30  // GitHub asset limit

var defaultPackManifest = []packManifestEntry{
	{
		ID: "whatsmyname", Version: "2026.10.1",
		URL:     "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/whatsmyname-2026.10.1.tar.zst",
		SHA256:  "9a94eb04d58b0dd50fc91cbd9b20fea85cf1b9450c8bea92299241a2b80673e1",
		Size:    15469,
		License: "CC-BY-SA-4.0", Upstream: "https://github.com/WebBreacher/WhatsMyName",
		Description: "WhatsMyName site list (692 sites)",
	},
	{
		ID: "nuclei_templates", Version: "2026.10.1",
		URL:     "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/nuclei-templates-2026.10.1.tar.zst",
		SHA256:  "f6c4583f3268b21855c024e3d0fd052792d86c5963f44c68e488446b648d0a0e",
		Size:    9582229,
		License: "MIT", Upstream: "https://github.com/projectdiscovery/nuclei-templates",
		Description: "nuclei templates (projectdiscovery)",
	},
	{ID: "kev_epss", Version: "2026.10.1", URL: "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/kev.tar.zst", SHA256: "d26632078538ea3d9bb2df7c8654ca35257da611a0ef844bf9e334fc1663873f", Size: 2923566, License: "Public domain", Description: "CISA KEV + EPSS"},
	{ID: "seclists_curated", Version: "2026.10.1", URL: "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/seclists.tar.zst", SHA256: "684480dfe2b15ca5258a28e26e17e8f32acb85c6242a1897e3bef442f7e9b11a", Size: 383615, License: "MIT", Description: "SecLists curated wordlists"},
	{ID: "payloads_all_the_things", Version: "2026.10.1", URL: "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/payloads.tar.zst", SHA256: "49f007cff964c15ceba756e5ffdef224a21a6b4be82e6d05559d66266d225e03", Size: 7843471, License: "MIT", Description: "PayloadsAllTheThings", Consent: true},
	{ID: "fuzzdb", Version: "2026.10.1", URL: "https://github.com/wogho/redex-packs/releases/download/packs-2026.10/fuzzdb.tar.zst", SHA256: "5c0c5b3ad30a3646f980c1c3e472f18178eba6667830039be7b8e0c2332c59fa", Size: 5004124, License: "Apache/CC", Description: "fuzzdb dictionaries", Consent: true},
}

func (s *Server) packRoot() string {
	reg := s.loadPackRegistry()
	if reg.Settings.Dir != "" {
		return reg.Settings.Dir
	}
	if v := strings.TrimSpace(os.Getenv("REDEX_PACKS_DIR")); v != "" {
		return v
	}
	return filepath.Join(s.m.dir, "packs")
}

func (s *Server) packRegistryPath() string { return filepath.Join(s.m.dir, "packs", "state.json") }

func (s *Server) defaultPackSettings() packSettings {
	return packSettings{Dir: s.packRoot(), MaxBytes: defaultMaxBytes, AutoEvict: true}
}

func (s *Server) loadPackRegistry() packRegistry {
	reg := packRegistry{Packs: map[string]packState{}}
	if raw, err := os.ReadFile(s.packRegistryPath()); err == nil {
		_ = json.Unmarshal(raw, &reg)
	}
	if reg.Packs == nil {
		reg.Packs = map[string]packState{}
	}
	if reg.Settings.MaxBytes == 0 {
		reg.Settings.MaxBytes = defaultMaxBytes
	}
	if reg.Settings.Dir == "" {
		if v := strings.TrimSpace(os.Getenv("REDEX_PACKS_DIR")); v != "" {
			reg.Settings.Dir = v
		} else {
			reg.Settings.Dir = filepath.Join(s.m.dir, "packs")
		}
	}
	return reg
}

func (s *Server) savePackRegistry(reg packRegistry) error {
	path := s.packRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(reg, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// PackEnabled reports installed+enabled.
func (s *Server) PackEnabled(id string) bool {
	ps, ok := s.loadPackRegistry().Packs[id]
	return ok && ps.Enabled
}

// PackFilePath resolves a pack file and touches last_used.
func (s *Server) PackFilePath(id, name string) (string, bool) {
	reg := s.loadPackRegistry()
	ps, ok := reg.Packs[id]
	if !ok || !ps.Enabled {
		return "", false
	}
	p := filepath.Join(reg.Settings.Dir, id, ps.Version, name)
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	if time.Since(ps.LastUsedAt) > time.Hour {
		ps.LastUsedAt = time.Now().UTC()
		reg.Packs[id] = ps
		_ = s.savePackRegistry(reg)
	}
	return p, true
}

func packBytes(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func freeBytes(dir string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

func (s *Server) evictIfNeeded(reg *packRegistry) {
	if !reg.Settings.AutoEvict || reg.Settings.MaxBytes <= 0 {
		return
	}
	var total int64
	for _, ps := range reg.Packs {
		total += ps.Bytes
	}
	if total <= reg.Settings.MaxBytes {
		return
	}
	type cand struct {
		id string
		ps packState
	}
	var list []cand
	for id, ps := range reg.Packs {
		if ps.Pinned || ps.Enabled {
			continue
		}
		list = append(list, cand{id, ps})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ps.LastUsedAt.Before(list[j].ps.LastUsedAt) })
	for _, c := range list {
		if total <= reg.Settings.MaxBytes {
			break
		}
		_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, c.id))
		total -= c.ps.Bytes
		delete(reg.Packs, c.id)
	}
}

func packByID(id string) (packManifestEntry, bool) {
	for _, m := range defaultPackManifest {
		if m.ID == id {
			return m, true
		}
	}
	return packManifestEntry{}, false
}

// GET /api/packs
func (s *Server) packsList(w http.ResponseWriter, _ *http.Request) {
	reg := s.loadPackRegistry()
	out := make([]map[string]any, 0, len(defaultPackManifest))
	var used int64
	for _, m := range defaultPackManifest {
		v := map[string]any{
			"id": m.ID, "version": m.Version, "license": m.License,
			"description": m.Description, "size": m.Size,
			"installed": false, "enabled": false, "pinned": false, "consent_required": m.Consent, "consent": false,
		}
		if ps, ok := reg.Packs[m.ID]; ok {
			v["installed"] = true
			v["enabled"] = ps.Enabled
			v["pinned"] = ps.Pinned
			v["consent"] = ps.Consent
			v["installed_at"] = ps.InstalledAt
			v["last_used_at"] = ps.LastUsedAt
			v["bytes"] = ps.Bytes
			used += ps.Bytes
		}
		out = append(out, v)
	}
	writeJSON(w, 200, map[string]any{
		"packs": out,
		"settings": map[string]any{
			"dir": reg.Settings.Dir, "max_bytes": reg.Settings.MaxBytes,
			"auto_evict":             reg.Settings.AutoEvict,
			"auto_delete_on_disable": reg.Settings.AutoDeleteOnDisable,
		},
		"usage": map[string]any{"bytes": used, "free_bytes": freeBytes(reg.Settings.Dir)},
	})
}

// PUT /api/packs/settings {dir,max_bytes,auto_evict,auto_delete_on_disable}
func (s *Server) packsSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dir                 string `json:"dir"`
		MaxBytes            int64  `json:"max_bytes"`
		AutoEvict           *bool  `json:"auto_evict"`
		AutoDeleteOnDisable *bool  `json:"auto_delete_on_disable"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	reg := s.loadPackRegistry()
	if d := strings.TrimSpace(body.Dir); d != "" && d != reg.Settings.Dir {
		if err := os.MkdirAll(d, 0o755); err != nil {
			writeErr(w, 400, "경로를 사용할 수 없습니다: "+err.Error())
			return
		}
		probe, err := os.CreateTemp(d, ".probe-*")
		if err != nil {
			writeErr(w, 400, "쓰기 권한이 없습니다")
			return
		}
		probe.Close()
		_ = os.Remove(probe.Name())
		if len(reg.Packs) > 0 {
			if err := s.migratePacks(&reg, d); err != nil {
				writeErr(w, 500, "마이그레이션 실패: "+err.Error())
				return
			}
		} else {
			reg.Settings.Dir = d
		}
	}
	if body.MaxBytes > 0 {
		reg.Settings.MaxBytes = body.MaxBytes
	}
	if body.AutoEvict != nil {
		reg.Settings.AutoEvict = *body.AutoEvict
	}
	if body.AutoDeleteOnDisable != nil {
		reg.Settings.AutoDeleteOnDisable = *body.AutoDeleteOnDisable
	}
	if err := s.savePackRegistry(reg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "settings": reg.Settings})
}

// POST /api/packs/{id}/install
func (s *Server) packInstall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := packByID(id)
	if !ok {
		writeErr(w, 404, "unknown pack")
		return
	}
	reg := s.loadPackRegistry()
	dir := reg.Settings.Dir
	if free := freeBytes(dir); free >= 0 && free < m.Size*11/10+1<<20 {
		writeErr(w, 507, "디스크 여유 공간이 부족합니다")
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tmp, err := os.CreateTemp(dir, "pack-*.tar.zst")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)
	if err := s.downloadPack(r.Context(), m, tmpName, nil); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	dest := filepath.Join(dir, id, m.Version)
	_ = os.RemoveAll(filepath.Join(dir, id))
	if err := extractPackTar(tmpName, dest); err != nil {
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

// POST /api/packs/{id}/enable|disable|pin
func (s *Server) packToggle(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		reg := s.loadPackRegistry()
		ps, ok := reg.Packs[id]
		if !ok {
			writeErr(w, 400, "pack is not installed")
			return
		}
		if enabled {
			for _, m := range defaultPackManifest {
				if m.ID == id && m.Consent && !ps.Consent {
					writeErr(w, 403, "이 팩은 동의가 필요합니다. /api/packs/{id}/consent 로 동의 후 활성화하세요.")
					return
				}
			}
		}
		ps.Enabled = enabled
		ps.LastUsedAt = time.Now().UTC()
		reg.Packs[id] = ps
		if !enabled && reg.Settings.AutoDeleteOnDisable {
			_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, id))
			delete(reg.Packs, id)
		}
		s.evictIfNeeded(&reg)
		if err := s.savePackRegistry(reg); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "id": id, "enabled": enabled})
	}
}

func (s *Server) packPin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Pinned bool `json:"pinned"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	reg := s.loadPackRegistry()
	ps, ok := reg.Packs[id]
	if !ok {
		writeErr(w, 400, "pack is not installed")
		return
	}
	ps.Pinned = body.Pinned
	reg.Packs[id] = ps
	if err := s.savePackRegistry(reg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id, "pinned": body.Pinned})
}

// DELETE /api/packs/{id}
func (s *Server) packDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reg := s.loadPackRegistry()
	if ps, ok := reg.Packs[id]; ok {
		if ps.Enabled {
			writeErr(w, 409, "활성 상태에서는 삭제할 수 없습니다. 먼저 끄세요.")
			return
		}
		_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, id))
		delete(reg.Packs, id)
		if err := s.savePackRegistry(reg); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

var errPackMissing = errors.New("pack not enabled")

// POST /api/packs/{id}/consent {"agree":true}
func (s *Server) packConsent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Agree bool `json:"agree"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	reg := s.loadPackRegistry()
	ps, ok := reg.Packs[id]
	if !ok {
		writeErr(w, 400, "pack is not installed")
		return
	}
	ps.Consent = body.Agree
	reg.Packs[id] = ps
	if err := s.savePackRegistry(reg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "consent": body.Agree})
}
