package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/wogho/redex/db"
)

// GET /api/verification/findings?status=&task_id=&min_confidence=&limit=
// Lists findings that carry a verification verdict (see verify package).
func (s *Server) verificationFindings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := db.VerificationFilter{
		Status:        strings.TrimSpace(q.Get("status")),
		MinConfidence: atoiDefault(q.Get("min_confidence"), 0),
		Limit:         atoiDefault(q.Get("limit"), 200),
	}
	if raw := strings.TrimSpace(q.Get("task_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			writeErr(w, 400, "bad task_id")
			return
		}
		f.TaskID = id
	}
	rows, err := s.m.pg.ListVerificationFindings(r.Context(), f)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"findings": rows})
}

// GET /api/verification/summary — counts per verify_status.
func (s *Server) verificationSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.m.pg.VerificationSummary(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"counts": sum})
}
