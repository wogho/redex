package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wogho/redex/db"
)

// Engagement scope (RoE) CRUD. Kept intentionally small: the record documents
// what was authorized, for whom, and when. Enforcement still comes from operator
// policy + task scope; this is the auditable authorization record.

func (s *Server) scopeList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.ListEngagementScopes(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"scopes": rows})
}

func (s *Server) scopeCreate(w http.ResponseWriter, r *http.Request) {
	var in db.EngagementScope
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	row, err := s.m.pg.CreateEngagementScope(r.Context(), in)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, row)
}

func (s *Server) scopeUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	var in db.EngagementScope
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	in.ID = id
	row, err := s.m.pg.UpdateEngagementScope(r.Context(), in)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, row)
}

func (s *Server) scopeDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	if err := s.m.pg.DeleteEngagementScope(r.Context(), id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// POST /api/scope/{id}/person-osint {"allowed":bool}
func (s *Server) scopePersonOsint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	var body struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.m.pg.SetScopePersonOsint(r.Context(), id, body.Allowed); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "allowed": body.Allowed})
}
