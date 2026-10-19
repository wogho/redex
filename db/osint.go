package db

import (
	"context"
	"encoding/json"
	"time"
)

// OsintCandidate is one lead produced by an OSINT collector. It is deliberately
// separate from findings: a lead is not a vulnerability, and must be verified
// before it is promoted.
type OsintCandidate struct {
	ID           int64     `json:"id,string"`
	EngagementID int64     `json:"engagement_id,omitempty"`
	Kind         string    `json:"kind"`   // handle | email | contact | profile | face
	Value        string    `json:"value"`  // the input that produced it
	Source       string    `json:"source"` // maigret/holehe/phoneinfoga/instaloader+insightface
	Detail       string    `json:"detail"`
	Evidence     string    `json:"evidence,omitempty"`
	Confidence   int       `json:"confidence"`
	Status       string    `json:"status"` // unverified | evidence-backed
	CreatedAt    time.Time `json:"created_at"`
}

const osintCandidateCols = `id, COALESCE(engagement_id,0), kind, value, source, detail, evidence, confidence, status, created_at`

func scanOsintCandidate(row interface{ Scan(...any) error }, v *OsintCandidate) error {
	return row.Scan(&v.ID, &v.EngagementID, &v.Kind, &v.Value, &v.Source, &v.Detail, &v.Evidence, &v.Confidence, &v.Status, &v.CreatedAt)
}

func (d *DB) InsertOsintCandidate(ctx context.Context, c OsintCandidate) (*OsintCandidate, error) {
	v := &OsintCandidate{}
	err := scanOsintCandidate(d.QueryRowContext(ctx, `
INSERT INTO osint_candidate(engagement_id,kind,value,source,detail,evidence,confidence,status)
VALUES(NULLIF($1,0),$2,$3,$4,$5,$6,$7,$8) RETURNING `+osintCandidateCols,
		c.EngagementID, c.Kind, c.Value, c.Source, c.Detail, c.Evidence, c.Confidence, c.Status), v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *DB) ListOsintCandidates(ctx context.Context, engagementID int64, kind string) ([]*OsintCandidate, error) {
	rows, err := d.QueryContext(ctx, `
SELECT `+osintCandidateCols+`
  FROM osint_candidate
 WHERE ($1 = 0 OR engagement_id = $1)
   AND ($2 = '' OR kind = $2)
 ORDER BY created_at DESC
 LIMIT 500`, engagementID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*OsintCandidate, 0, 32)
	for rows.Next() {
		v := &OsintCandidate{}
		if err := scanOsintCandidate(rows, v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// FaceRecord is one enrolled face embedding.
type FaceRecord struct {
	ID        int64     `json:"id,string"`
	Label     string    `json:"label"`
	Source    string    `json:"source"`
	Embedding []float32 `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (d *DB) InsertFace(ctx context.Context, label, source string, embedding []float32) (int64, error) {
	raw, _ := json.Marshal(embedding)
	var id int64
	err := d.QueryRowContext(ctx, `INSERT INTO osint_face(label,source,embedding) VALUES($1,$2,$3) RETURNING id`,
		label, source, string(raw)).Scan(&id)
	return id, err
}

func (d *DB) ListFaces(ctx context.Context) ([]*FaceRecord, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,label,source,embedding,created_at FROM osint_face ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*FaceRecord, 0, 16)
	for rows.Next() {
		v := &FaceRecord{}
		var emb []byte
		if err := rows.Scan(&v.ID, &v.Label, &v.Source, &emb, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(emb, &v.Embedding)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) DeleteFace(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM osint_face WHERE id=$1`, id)
	return err
}

// SetScopePersonOsint flips the per-engagement "person investigation allowed" flag.
func (d *DB) SetScopePersonOsint(ctx context.Context, id int64, allowed bool) error {
	_, err := d.ExecContext(ctx, `UPDATE engagement_scope SET person_osint_allowed=$2 WHERE id=$1`, id, allowed)
	return err
}

// ScopePersonOsintAllowed reports whether the engagement permits person OSINT.
func (d *DB) ScopePersonOsintAllowed(ctx context.Context, id int64) (bool, error) {
	var ok bool
	err := d.QueryRowContext(ctx, `SELECT person_osint_allowed FROM engagement_scope WHERE id=$1`, id).Scan(&ok)
	return ok, err
}
