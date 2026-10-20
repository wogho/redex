package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Task schedules: repeat a task by adding a goal on an interval. The due action
// calls the same internal API a human would (loopback + signed token), so task
// lifecycle rules stay in one place.

type taskSchedule struct {
	ID          int64     `json:"id,string"`
	TaskID      string    `json:"task_id"`
	Name        string    `json:"name"`
	Goal        string    `json:"goal"`
	IntervalMin int       `json:"interval_minutes"`
	Enabled     bool      `json:"enabled"`
	LastRunAt   time.Time `json:"last_run_at"`
	NextRunAt   time.Time `json:"next_run_at"`
}

func (s *Server) selfURL() string {
	if v := strings.TrimSpace(os.Getenv("REDEX_SELF_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:8787"
}

// GET /api/schedules
func (s *Server) schedulesList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.QueryContext(r.Context(), `
SELECT id, task_id, name, goal, interval_min, enabled, COALESCE(last_run_at,'epoch'), next_run_at
  FROM task_schedule ORDER BY created_at DESC`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := make([]taskSchedule, 0, 16)
	for rows.Next() {
		var v taskSchedule
		if err := rows.Scan(&v.ID, &v.TaskID, &v.Name, &v.Goal, &v.IntervalMin, &v.Enabled, &v.LastRunAt, &v.NextRunAt); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, map[string]any{"schedules": out})
}

// POST /api/schedules {task_id,name,goal,interval_minutes,enabled}
func (s *Server) schedulesCreate(w http.ResponseWriter, r *http.Request) {
	var body taskSchedule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	body.TaskID = strings.TrimSpace(body.TaskID)
	body.Goal = strings.TrimSpace(body.Goal)
	if body.TaskID == "" || body.Goal == "" {
		writeErr(w, 400, "task_id and goal are required")
		return
	}
	if body.IntervalMin < 1 {
		body.IntervalMin = 60
	}
	next := time.Now().UTC().Add(time.Duration(body.IntervalMin) * time.Minute)
	var id int64
	if err := s.m.pg.QueryRowContext(r.Context(), `
INSERT INTO task_schedule(task_id,name,goal,interval_min,enabled,next_run_at)
VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		body.TaskID, body.Name, body.Goal, body.IntervalMin, body.Enabled, next).Scan(&id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}

// POST /api/schedules/{id}/toggle {"enabled":bool}
func (s *Server) schedulesToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if _, err := s.m.pg.ExecContext(r.Context(), `UPDATE task_schedule SET enabled=$2 WHERE id=$1`, id, body.Enabled); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/schedules/{id}
func (s *Server) schedulesDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	_, _ = s.m.pg.ExecContext(r.Context(), `DELETE FROM task_schedule WHERE id=$1`, id)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// POST /api/schedules/{id}/run — run now
func (s *Server) schedulesRunNow(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad id")
		return
	}
	if err := s.runSchedule(r.Context(), id); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) runSchedule(ctx context.Context, id int64) error {
	var v taskSchedule
	err := s.m.pg.QueryRowContext(ctx, `
SELECT id, task_id, name, goal, interval_min, enabled, COALESCE(last_run_at,'epoch'), next_run_at
  FROM task_schedule WHERE id=$1`, id).
		Scan(&v.ID, &v.TaskID, &v.Name, &v.Goal, &v.IntervalMin, &v.Enabled, &v.LastRunAt, &v.NextRunAt)
	if err != nil {
		return fmt.Errorf("schedule not found")
	}
	hash, err := s.readPassHash()
	if err != nil {
		return err
	}
	token, err := signJWT(s.signingKey(hash))
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"text": v.Goal})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.selfURL()+"/api/tasks/"+v.TaskID+"/goals", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("goal trigger status %d", resp.StatusCode)
	}
	next := time.Now().UTC().Add(time.Duration(v.IntervalMin) * time.Minute)
	_, _ = s.m.pg.ExecContext(ctx, `UPDATE task_schedule SET last_run_at=now(), next_run_at=$2 WHERE id=$1`, id, next)
	return nil
}

// scheduleLoop runs due schedules once a minute.
func (s *Server) scheduleLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			rows, err := s.m.pg.QueryContext(s.ctx, `
SELECT id FROM task_schedule WHERE enabled AND next_run_at <= now() ORDER BY next_run_at LIMIT 10`)
			if err != nil {
				continue
			}
			var ids []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				_ = s.runSchedule(s.ctx, id)
			}
		}
	}
}
