package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wogho/redex/db"
)

// Provider registry: declarative credential fields + optional runner.
// Credentials are operator-supplied; Redex never pays for a provider.

type providerDef struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Fields   []string `json:"fields"`
	Paid     bool     `json:"paid"`
	Docs     string   `json:"docs"`
	Note     string   `json:"note,omitempty"`
	Consent  bool     `json:"consent_required"`
	run      func(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error)
}

var providers = []providerDef{
	{ID: "shodan", Consent: true, Name: "Shodan", Category: "infra", Fields: []string{"key"}, Paid: true, Docs: "https://developer.shodan.io", run: runShodan},
	{ID: "censys", Consent: true, Name: "Censys", Category: "infra", Fields: []string{"id", "secret"}, Paid: true, Docs: "https://search.censys.io/api", run: runCensys},
	{ID: "intelx", Consent: true, Name: "Intelligence X", Category: "identity", Fields: []string{"key"}, Paid: true, Docs: "https://intelx.io", run: runIntelX},
	{ID: "epieos", Consent: true, Name: "Epieos", Category: "identity", Fields: []string{"key"}, Paid: true, Docs: "https://epieos.com", run: runEpieos},
	{ID: "numverify", Consent: true, Name: "Numverify", Category: "identity", Fields: []string{"key"}, Paid: true, Docs: "https://numverify.com", run: runNumverify},
	{ID: "abstract_phone", Consent: true, Name: "Abstract Phone", Category: "identity", Fields: []string{"key"}, Paid: true, Docs: "https://www.abstractapi.com", Note: "전화 검증 API; 커넥터 준비 중"},
	{ID: "naver", Name: "Naver Search", Category: "kr", Fields: []string{"client_id", "client_secret"}, Paid: false, Docs: "https://developers.naver.com", run: runNaver},
	{ID: "kipris", Name: "KIPRIS", Category: "kr", Fields: []string{"key"}, Paid: false, Docs: "http://plus.kipris.or.kr", run: runKIPRIS},
	{ID: "saucenao", Name: "SauceNAO", Category: "image", Fields: []string{"key"}, Paid: false, Docs: "https://saucenao.com/user.php?page=search-api", run: runSauceNAO},
	{ID: "x", Consent: true, Name: "X (Twitter) API", Category: "social", Fields: []string{"bearer"}, Paid: true, Docs: "https://developer.x.com", run: runX},
	{ID: "bing_visual", Consent: true, Name: "Bing Visual Search", Category: "image", Fields: []string{"key"}, Paid: true, Docs: "https://www.microsoft.com/bing/apis/visual-search", Note: "이미지 업로드 API; 커넥터 준비 중"},
	{ID: "google_vision", Consent: true, Name: "Google Vision", Category: "image", Fields: []string{"key"}, Paid: true, Docs: "https://cloud.google.com/vision", Note: "Web Detection; 커넥터 준비 중"},
	{ID: "tineye", Consent: true, Name: "TinEye", Category: "image", Fields: []string{"key"}, Paid: true, Docs: "https://tineye.com/api", Note: "이미지 업로드 API; 커넥터 준비 중"},
	{ID: "yandex", Name: "Yandex Images", Category: "image", Fields: []string{}, Paid: false, Docs: "https://yandex.com/images", Note: "공식 API 없음; 수동 검색 전용"},
	{ID: "linkedin", Consent: true, Name: "LinkedIn", Category: "social", Fields: []string{"cookie"}, Paid: false, Docs: "https://www.linkedin.com", Note: "공개 프로필만; 계정 세션 슬롯"},
	{ID: "facebook", Consent: true, Name: "Facebook", Category: "social", Fields: []string{"cookie"}, Paid: false, Docs: "https://www.facebook.com", Note: "공개 범위 제한; 계정 세션 슬롯"},
}

func providerByID(id string) (providerDef, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return providerDef{}, false
}

func (s *Server) providerCredPath() string { return s.osintSecret("providers.json") }

func (s *Server) providerCreds() map[string]map[string]string {
	out := map[string]map[string]string{}
	if raw, err := os.ReadFile(s.providerCredPath()); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func (s *Server) saveProviderCreds(m map[string]map[string]string) error {
	dir := filepath.Dir(s.providerCredPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	tmp := s.providerCredPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.providerCredPath())
}

// GET /api/osint/providers
func (s *Server) providersList(w http.ResponseWriter, _ *http.Request) {
	creds := s.providerCreds()
	out := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		cfg := creds[p.ID]
		configured := len(p.Fields) > 0
		for _, f := range p.Fields {
			if strings.TrimSpace(cfg[f]) == "" {
				configured = false
				break
			}
		}
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "category": p.Category, "fields": p.Fields,
			"paid": p.Paid, "docs": p.Docs, "note": p.Note,
			"configured": configured, "implemented": p.run != nil,
			"consent_required": p.Consent, "consent": s.providerConsents()[p.ID],
		})
	}
	writeJSON(w, 200, map[string]any{"providers": out})
}

// PUT /api/osint/providers/{id} {"creds":{"key":"..."}}
func (s *Server) providerSave(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	def, ok := providerByID(id)
	if !ok {
		writeErr(w, 404, "unknown provider")
		return
	}
	var body struct {
		Creds map[string]string `json:"creds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	creds := s.providerCreds()
	if creds[id] == nil {
		creds[id] = map[string]string{}
	}
	for _, f := range def.Fields {
		if v, ok := body.Creds[f]; ok {
			creds[id][f] = strings.TrimSpace(v)
		}
	}
	if err := s.saveProviderCreds(creds); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// POST /api/osint/providers/{id}/run {"input":"...","engagement_id":1}
func (s *Server) providerRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	def, ok := providerByID(id)
	if !ok {
		writeErr(w, 404, "unknown provider")
		return
	}
	if def.run == nil {
		writeErr(w, 501, def.Name+" 커넥터는 준비 중입니다 (키는 저장됨)")
		return
	}
	var body struct {
		Input        string `json:"input"`
		EngagementID int64  `json:"engagement_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	body.Input = strings.TrimSpace(body.Input)
	if body.Input == "" {
		writeErr(w, 400, "input is required")
		return
	}
	if def.Consent && !s.providerConsents()[id] {
		writeErr(w, 403, "이 프로바이더는 동의가 필요합니다. /api/osint/providers/{id}/consent 로 동의하세요.")
		return
	}
	creds := s.providerCreds()[id]
	if creds == nil {
		writeErr(w, 400, "provider is not configured")
		return
	}
	cands, err := def.run(r.Context(), s, creds, body.Input)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	saved, err := s.saveCandidates(r.Context(), nil, cands)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": saved, "count": len(saved)})
}

func providerHTTP(ctx context.Context, method, rawurl string, headers map[string]string, body string) (int, []byte) {
	c, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c, method, rawurl, rd)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("user-agent", "Redex-OSINT/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, raw
}

func runShodan(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://api.shodan.io/shodan/host/search?key=" + url.QueryEscape(creds["key"]) + "&query=" + url.QueryEscape(input)
	code, body := providerHTTP(ctx, http.MethodGet, u, nil, "")
	if code != 200 {
		return nil, fmt.Errorf("shodan status %d", code)
	}
	var res struct {
		Matches []struct {
			IP    string   `json:"ip_str"`
			Port  int      `json:"port"`
			Org   string   `json:"org"`
			Hosts []string `json:"hostnames"`
		} `json:"matches"`
	}
	if json.Unmarshal(body, &res) != nil {
		return nil, fmt.Errorf("shodan parse error")
	}
	out := make([]db.OsintCandidate, 0, len(res.Matches))
	for _, m := range res.Matches {
		out = append(out, db.OsintCandidate{
			Kind: "infra", Value: input, Source: "shodan",
			Detail:   fmt.Sprintf("%s:%d %s %s", m.IP, m.Port, m.Org, strings.Join(m.Hosts, ",")),
			Evidence: "shodan result", Confidence: 70, Status: "evidence-backed",
		})
	}
	return out, nil
}

func runCensys(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://search.censys.io/api/v2/hosts/search?q=" + url.QueryEscape(input) + "&per_page=25"
	auth := base64.StdEncoding.EncodeToString([]byte(creds["id"] + ":" + creds["secret"]))
	code, body := providerHTTP(ctx, http.MethodGet, u, map[string]string{"Authorization": "Basic " + auth}, "")
	if code != 200 {
		return nil, fmt.Errorf("censys status %d", code)
	}
	var res struct {
		Result struct {
			Hits []struct {
				IP   string `json:"ip"`
				Name string `json:"name"`
			} `json:"hits"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &res) != nil {
		return nil, fmt.Errorf("censys parse error")
	}
	out := make([]db.OsintCandidate, 0, len(res.Result.Hits))
	for _, h := range res.Result.Hits {
		out = append(out, db.OsintCandidate{
			Kind: "infra", Value: input, Source: "censys",
			Detail: h.IP + " " + h.Name, Evidence: "censys result", Confidence: 70, Status: "evidence-backed",
		})
	}
	return out, nil
}

func runIntelX(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	body := fmt.Sprintf(`{"term":%q,"maxresults":20,"media":0,"sort":2,"terminate":[]}`, input)
	code, raw := providerHTTP(ctx, http.MethodPost, "https://2.intelx.io/intelligent/search", map[string]string{
		"x-key": creds["key"], "Content-Type": "application/json",
	}, body)
	if code != 200 {
		return nil, fmt.Errorf("intelx status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "identity", Value: input, Source: "intelx", Detail: "intelx search submitted",
		Evidence: truncate(string(raw), 800), Confidence: 55, Status: "unverified",
	}}, nil
}

func runEpieos(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://api.epieos.com/v1/search?type=email&value=" + url.QueryEscape(input) + "&key=" + url.QueryEscape(creds["key"])
	code, body := providerHTTP(ctx, http.MethodGet, u, nil, "")
	if code != 200 {
		return nil, fmt.Errorf("epieos status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "email", Value: input, Source: "epieos", Detail: "epieos result",
		Evidence: truncate(string(body), 800), Confidence: 60, Status: "unverified",
	}}, nil
}

func runNumverify(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://apilayer.net/api/validate?access_key=" + url.QueryEscape(creds["key"]) + "&number=" + url.QueryEscape(input)
	code, body := providerHTTP(ctx, http.MethodGet, u, nil, "")
	if code != 200 {
		return nil, fmt.Errorf("numverify status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "contact", Value: input, Source: "numverify", Detail: "carrier/line lookup",
		Evidence: truncate(string(body), 800), Confidence: 55, Status: "unverified",
	}}, nil
}

func runNaver(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://openapi.naver.com/v1/search/webkr.json?display=10&query=" + url.QueryEscape(input)
	code, body := providerHTTP(ctx, http.MethodGet, u, map[string]string{
		"X-Naver-Client-Id": creds["client_id"], "X-Naver-Client-Secret": creds["client_secret"],
	}, "")
	if code != 200 {
		return nil, fmt.Errorf("naver status %d", code)
	}
	var res struct {
		Items []struct {
			Title       string `json:"title"`
			Link        string `json:"link"`
			Description string `json:"description"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &res) != nil {
		return nil, fmt.Errorf("naver parse error")
	}
	out := make([]db.OsintCandidate, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, db.OsintCandidate{
			Kind: "handle", Value: input, Source: "naver",
			Detail: stripTags(it.Title) + " " + it.Link, Evidence: truncate(stripTags(it.Description), 400),
			Confidence: 45, Status: "unverified",
		})
	}
	return out, nil
}

func runKIPRIS(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "http://plus.kipris.or.kr/openapi/rest/patUtiModInfoSearchSevice/applicantNameSearchInfo?applicantName=" +
		url.QueryEscape(input) + "&accessKey=" + url.QueryEscape(creds["key"])
	code, body := providerHTTP(ctx, http.MethodGet, u, nil, "")
	if code != 200 {
		return nil, fmt.Errorf("kipris status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "handle", Value: input, Source: "kipris", Detail: "patent applicant search",
		Evidence: truncate(string(body), 800), Confidence: 40, Status: "unverified",
	}}, nil
}

func runSauceNAO(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://saucenao.com/search.php?output_type=2&api_key=" + url.QueryEscape(creds["key"]) + "&url=" + url.QueryEscape(input)
	code, body := providerHTTP(ctx, http.MethodGet, u, nil, "")
	if code != 200 {
		return nil, fmt.Errorf("saucenao status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "profile", Value: input, Source: "saucenao", Detail: "image similarity result",
		Evidence: truncate(string(body), 800), Confidence: 45, Status: "unverified",
	}}, nil
}

func runX(ctx context.Context, s *Server, creds map[string]string, input string) ([]db.OsintCandidate, error) {
	u := "https://api.twitter.com/2/users/by/username/" + url.PathEscape(input)
	code, body := providerHTTP(ctx, http.MethodGet, u, map[string]string{"Authorization": "Bearer " + creds["bearer"]}, "")
	if code != 200 {
		return nil, fmt.Errorf("x status %d", code)
	}
	return []db.OsintCandidate{{
		Kind: "handle", Value: input, Source: "x", Detail: "x account lookup",
		Evidence: truncate(string(body), 800), Confidence: 50, Status: "unverified",
	}}, nil
}

// GET /api/osint/graphml — Maltego-friendly candidate export.
func (s *Server) osintGraphML(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.ListOsintCandidates(r.Context(), 0, "")
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/graphml+xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="redex-osint.graphml"`)
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<graphml xmlns="http://graphml.graphdrawing.org/xmlns">
<key id="label" for="node" attr.name="label" attr.type="string"/>
<graph id="G" edgedefault="directed">
`)
	for i, c := range rows {
		fmt.Fprintf(w, `<node id="n%d"><data key="label">%s | %s</data></node>`+"\n", i, xmlEscape(c.Source), xmlEscape(c.Detail))
	}
	fmt.Fprint(w, "</graph></graphml>\n")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch r {
		case '<':
			in = true
		case '>':
			in = false
		default:
			if !in {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func (s *Server) providerConsentPath() string { return s.osintSecret("provider_consent.json") }

func (s *Server) providerConsents() map[string]bool {
	out := map[string]bool{}
	if raw, err := os.ReadFile(s.providerConsentPath()); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// POST /api/osint/providers/{id}/consent {"agree":true}
func (s *Server) providerConsent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := providerByID(id); !ok {
		writeErr(w, 404, "unknown provider")
		return
	}
	var body struct {
		Agree bool `json:"agree"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	m := s.providerConsents()
	m[id] = body.Agree
	raw, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(s.providerConsentPath(), raw, 0o600); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "consent": body.Agree})
}
