package server

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wogho/redex/db"
)

// Extended free-source collectors. Key-gated/paid sources (Epieos, Numverify,
// Bing Visual, TinEye, Google Vision, SauceNAO, Naver/Daum/KIPRIS) are surfaced
// in the UI with their key slots; only sources that work without a paid key run
// automatically here.

var md5RE = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Server) evidenceDir() string {
	dir := filepath.Join(s.m.dir, "osint", "evidence")
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

// saveEvidence stores a snapshot and returns its sha256 + path.
func (s *Server) saveEvidence(body []byte) (string, string) {
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	path := filepath.Join(s.evidenceDir(), sha+".txt")
	if _, err := os.Stat(path); err != nil {
		_ = os.WriteFile(path, body, 0o600)
	}
	return sha, path
}

func (s *Server) httpGet(ctx context.Context, url string, timeout time.Duration) (int, []byte) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("user-agent", "Redex-OSINT/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, body
}

func (s *Server) collectExtended(ctx context.Context, in struct {
	EngagementID int64  `json:"engagement_id"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Limit        int    `json:"limit"`
}) ([]*db.OsintCandidate, error) {
	var cands []db.OsintCandidate
	add := func(kind, source, detail string, conf int, evidence []byte) {
		sha, _ := s.saveEvidence(evidence)
		cands = append(cands, db.OsintCandidate{
			EngagementID: in.EngagementID, Kind: kind, Value: in.Value,
			Source: source, Detail: truncate(detail, 300),
			Evidence: "sha256:" + sha, Confidence: conf, Status: "unverified",
		})
	}

	switch in.Kind {
	case "email":
		// Gravatar
		sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(in.Value))))
		hash := hex.EncodeToString(sum[:])
		if code, _ := s.httpGet(ctx, "https://www.gravatar.com/avatar/"+hash+"?d=404", 10*time.Second); code == 200 {
			add("email", "gravatar", "gravatar profile exists", 55, []byte(in.Value))
		}
		// Hudson Rock (infostealer logs, public endpoint)
		if code, body := s.httpGet(ctx, "https://cavalier.hudsonrock.com/api/json/v2/osint-tools/search-by-email?email="+in.Value, 20*time.Second); code == 200 && len(body) > 0 {
			var hr struct {
				Message string `json:"message"`
				Data    struct {
					Total int `json:"total"`
				} `json:"data"`
			}
			if json.Unmarshal(body, &hr) == nil && hr.Data.Total > 0 {
				add("email", "hudsonrock", fmt.Sprintf("%d infostealer records", hr.Data.Total), 65, body)
			}
		}
		// HIBP pastes (key required)
		if key, err := os.ReadFile(s.osintSecret("hibp.key")); err == nil && len(strings.TrimSpace(string(key))) > 0 {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://haveibeenpwned.com/api/v3/pasteaccount/"+in.Value, nil)
			req.Header.Set("hibp-api-key", strings.TrimSpace(string(key)))
			req.Header.Set("user-agent", "Redex-OSINT")
			if resp, err := http.DefaultClient.Do(req); err == nil {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				resp.Body.Close()
				if resp.StatusCode == 200 && len(body) > 2 {
					add("email", "hibp-pastes", "paste exposure found", 60, body)
				}
			}
		}

	case "handle":
		h := in.Value
		// Keybase
		if code, body := s.httpGet(ctx, "https://keybase.io/_/api/1.0/user/lookup.json?username="+h, 15*time.Second); code == 200 && strings.Contains(string(body), `"status":{"code":0`) {
			add("handle", "keybase", "keybase profile exists", 50, body)
		}
		// GitHub
		if code, body := s.httpGet(ctx, "https://api.github.com/users/"+h, 15*time.Second); code == 200 {
			add("handle", "github", "github account exists", 55, body)
		}
		// GitLab
		if code, body := s.httpGet(ctx, "https://gitlab.com/api/v4/users?username="+h, 15*time.Second); code == 200 && strings.HasPrefix(strings.TrimSpace(string(body)), "[{") {
			add("handle", "gitlab", "gitlab account exists", 50, body)
		}
		// Reddit (PullPush archive)
		if code, body := s.httpGet(ctx, "https://api.pullpush.io/reddit/search/submission/?author="+h+"&size=5", 20*time.Second); code == 200 && strings.Contains(string(body), `"author"`) {
			add("handle", "reddit", "reddit posts found", 45, body)
		}
		// Telegram public channel page
		if code, body := s.httpGet(ctx, "https://t.me/s/"+h, 15*time.Second); code == 200 && strings.Contains(string(body), "tgme_channel_info") {
			add("handle", "telegram", "public channel exists", 50, body)
		}
		// TikTok metadata via yt-dlp (title/uploader only)
		if out, err := exec.CommandContext(ctx, "yt-dlp", "--dump-single-json", "--no-warnings", "--skip-download", "https://www.tiktok.com/@"+h).Output(); err == nil && len(out) > 0 {
			add("handle", "tiktok", "tiktok profile metadata", 45, out)
		}
		// WhatsMyName pack (only if installed+enabled)
		if path, ok := s.PackFilePath("whatsmyname", "whatsmyname.json"); ok {
			hits := s.whatsMyNameHits(ctx, path, h)
			for _, hit := range hits {
				add("handle", "whatsmyname", hit, 50, []byte(hit))
			}
		}
	}

	if len(cands) == 0 {
		return nil, nil
	}
	if len(cands) > in.Limit*4 {
		cands = cands[:in.Limit*4]
	}
	return s.saveCandidates(ctx, in, cands)
}

// whatsMyNameHits checks a subset of the pack's sites concurrently.
func (s *Server) whatsMyNameHits(ctx context.Context, path, handle string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var data struct {
		Sites []struct {
			Name     string `json:"name"`
			URICheck string `json:"uri_check"`
			Valid    string `json:"valid"`
		} `json:"sites"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return nil
	}
	const maxSites = 120
	if len(data.Sites) > maxSites {
		data.Sites = data.Sites[:maxSites]
	}
	var (
		mu   sync.Mutex
		hits []string
		wg   sync.WaitGroup
		sem  = make(chan struct{}, 8)
	)
	client := &http.Client{Timeout: 8 * time.Second}
	for _, site := range data.Sites {
		if site.URICheck == "" {
			continue
		}
		site := site
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			u := strings.ReplaceAll(site.URICheck, "{account}", handle)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				return
			}
			req.Header.Set("user-agent", "Mozilla/5.0 (Redex OSINT)")
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
			resp.Body.Close()
			if resp.StatusCode != 200 {
				return
			}
			if site.Valid != "" && strings.Contains(string(body), site.Valid) {
				mu.Lock()
				hits = append(hits, site.Name+": "+u)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return hits
}
