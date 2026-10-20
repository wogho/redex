package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// Verification bulk operations and cluster merge.
// A "cluster" is the gate's dedupe_key group: repeated reports of the same issue.

type bulkBody struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

var findingStatuses = map[string]bool{
	"pending": true, "confirmed": true, "false_positive": true,
	"ignored": true, "duplicate": true, "risk_accepted": true, "resolved": true, "fixed": true,
}

// POST /api/verification/bulk {"ids":["1","2"],"status":"confirmed"}
func (s *Server) verificationBulk(w http.ResponseWriter, r *http.Request) {
	var body bulkBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !findingStatuses[body.Status] {
		writeErr(w, 400, "bad status")
		return
	}
	updated := 0
	for _, raw := range body.IDs {
		id, err := parseInt64(strings.TrimSpace(raw))
		if err != nil || id <= 0 {
			continue
		}
		if _, err := s.m.pg.ExecContext(r.Context(), `UPDATE findings SET status=$2 WHERE id=$1`, id, body.Status); err == nil {
			updated++
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "updated": updated, "status": body.Status})
}

// POST /api/verification/merge {"dedupe_key":"...","representative_id":"12"}
// Marks every other finding in the cluster as duplicate.
func (s *Server) verificationMerge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DedupeKey      string `json:"dedupe_key"`
		Representative string `json:"representative_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.DedupeKey) == "" || strings.TrimSpace(body.Representative) == "" {
		writeErr(w, 400, "dedupe_key and representative_id are required")
		return
	}
	rep, err := parseInt64(strings.TrimSpace(body.Representative))
	if err != nil || rep <= 0 {
		writeErr(w, 400, "bad representative_id")
		return
	}
	res, err := s.m.pg.ExecContext(r.Context(), `
UPDATE findings SET status='duplicate'
 WHERE id IN (SELECT f.id FROM findings f JOIN finding_verification v ON v.finding_id=f.id
               WHERE v.dedupe_key=$1 AND f.id<>$2)`, body.DedupeKey, rep)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, 200, map[string]any{"ok": true, "duplicates": n})
}

// GET /api/verification/clusters
func (s *Server) verificationClusters(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.QueryContext(r.Context(), `
SELECT v.dedupe_key, count(*), max(v.confidence),
       (array_agg(f.id ORDER BY v.confidence DESC))[1] AS rep,
       (array_agg(COALESCE(f.name,'')) )[1] AS rep_name
  FROM finding_verification v JOIN findings f ON f.id=v.finding_id
 GROUP BY v.dedupe_key
HAVING count(*) > 1
 ORDER BY count(*) DESC
 LIMIT 200`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type cluster struct {
		DedupeKey   string `json:"dedupe_key"`
		Count       int    `json:"count"`
		MaxConf     int    `json:"max_confidence"`
		RepresentID string `json:"representative_id"`
		RepName     string `json:"representative_name"`
	}
	out := make([]cluster, 0, 32)
	for rows.Next() {
		var c cluster
		var rep int64
		if err := rows.Scan(&c.DedupeKey, &c.Count, &c.MaxConf, &rep, &c.RepName); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		c.RepresentID = fmt.Sprintf("%d", rep)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	writeJSON(w, 200, map[string]any{"clusters": out})
}
