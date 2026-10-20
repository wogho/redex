package server

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
)

// CVE mapping: join ExploitDB (pack) with NVD (pack) for a given CVE id.

type edbRow struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	Platform    string `json:"platform"`
	Codes       string `json:"codes"`
}

var (
	edbMu     sync.Mutex
	edbMap    map[string][]edbRow
	edbLoaded bool
)

var cveRE = regexp.MustCompile(`(?i)^CVE-\d{4}-\d{4,}$`)

func (s *Server) loadExploitDB() (map[string][]edbRow, error) {
	edbMu.Lock()
	defer edbMu.Unlock()
	if edbLoaded {
		return edbMap, nil
	}
	{
		edbMap = map[string][]edbRow{}
		path, ok := s.PackFilePath("exploitdb", "exploitdb/files_exploits.csv")
		if !ok {
			return nil, os.ErrNotExist
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		if _, err := r.Read(); err != nil { // header
			return nil, err
		}
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil || len(rec) < 8 {
				continue
			}
			row := edbRow{ID: rec[0], File: rec[1], Description: rec[2], Date: rec[3], Type: rec[5], Platform: rec[6], Codes: rec[7]}
			for _, part := range strings.FieldsFunc(row.Codes, func(r rune) bool { return r == ';' || r == ',' || r == ' ' }) {
				if strings.HasPrefix(strings.ToUpper(part), "CVE-") {
					edbMap[strings.ToUpper(part)] = append(edbMap[strings.ToUpper(part)], row)
				}
			}
		}
	}
	edbLoaded = true
	return edbMap, nil
}

type nvdInfo struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Severity    string  `json:"severity"`
}

var (
	nvdMu    sync.Mutex
	nvdCache = map[string]*nvdInfo{}
)

func (s *Server) nvdLookup(cve string) *nvdInfo {
	nvdMu.Lock()
	if v, ok := nvdCache[cve]; ok {
		nvdMu.Unlock()
		return v
	}
	nvdMu.Unlock()
	for _, y := range []string{"2024", "2025", "2026"} {
		path, ok := s.PackFilePath("nvd_cve", "nvdcve-2.0-"+y+".json.gz")
		if !ok {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			continue
		}
		dec := json.NewDecoder(zr)
		info := scanNVD(dec, cve)
		zr.Close()
		f.Close()
		if info != nil {
			nvdMu.Lock()
			nvdCache[cve] = info
			nvdMu.Unlock()
			return info
		}
	}
	nvdMu.Lock()
	nvdCache[cve] = nil
	nvdMu.Unlock()
	return nil
}

// scanNVD streams {"vulnerabilities":[{cve:{...}}]} looking for one id.
func scanNVD(dec *json.Decoder, cve string) *nvdInfo {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil
		}
		if key != "vulnerabilities" {
			var skip json.RawMessage
			if dec.Decode(&skip) != nil {
				return nil
			}
			continue
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return nil
		}
		for dec.More() {
			var item struct {
				CVE struct {
					ID           string `json:"id"`
					Descriptions []struct {
						Lang  string `json:"lang"`
						Value string `json:"value"`
					} `json:"descriptions"`
					Metrics map[string][]struct {
						CVSSData struct {
							BaseScore    float64 `json:"baseScore"`
							BaseSeverity string  `json:"baseSeverity"`
						} `json:"cvssData"`
					} `json:"metrics"`
				} `json:"cve"`
			}
			if dec.Decode(&item) != nil {
				return nil
			}
			if !strings.EqualFold(item.CVE.ID, cve) {
				continue
			}
			info := &nvdInfo{ID: item.CVE.ID}
			for _, d := range item.CVE.Descriptions {
				if d.Lang == "en" {
					info.Description = d.Value
					break
				}
			}
			for _, list := range item.CVE.Metrics {
				for _, m := range list {
					if m.CVSSData.BaseScore > info.Score {
						info.Score = m.CVSSData.BaseScore
						info.Severity = m.CVSSData.BaseSeverity
					}
				}
			}
			return info
		}
	}
	return nil
}

// GET /api/cve/map?cve=CVE-2024-1234
func (s *Server) cveMap(w http.ResponseWriter, r *http.Request) {
	cve := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cve")))
	if !cveRE.MatchString(cve) {
		writeErr(w, 400, "cve must look like CVE-2024-1234")
		return
	}
	rows, _ := s.loadExploitDB()
	exploits := rows[cve]
	if exploits == nil {
		exploits = []edbRow{}
	}
	writeJSON(w, 200, map[string]any{
		"cve":      cve,
		"exploits": exploits,
		"nvd":      s.nvdLookup(cve),
	})
}
