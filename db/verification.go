package db

import (
	"context"
	"encoding/json"
	"time"
)

// VerificationFinding is one row of the verification queue: a finding plus the
// gate verdict stored in finding_verification.
type VerificationFinding struct {
	FindingID       int64     `json:"finding_id,string"`
	TaskID          int64     `json:"task_id,omitempty"`
	TaskDescription string    `json:"task_description,omitempty"`
	Name            string    `json:"name"`
	VulnClass       string    `json:"vulnclass"`
	Severity        string    `json:"severity"`
	FindingStatus   string    `json:"status"`
	Confidence      int       `json:"confidence"`
	VerifyStatus    string    `json:"verify_status"`
	DedupeKey       string    `json:"dedupe_key,omitempty"`
	Reasons         []string  `json:"reasons,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// VerificationFilter narrows the queue.
type VerificationFilter struct {
	Status        string // exact verify_status; "" = all
	TaskID        int64
	MinConfidence int
	Limit         int
}

// ListVerificationFindings returns findings that passed through the gate,
// newest first. Only gated findings exist in finding_verification, so legacy
// rows without a verdict are intentionally absent.
func (d *DB) ListVerificationFindings(ctx context.Context, f VerificationFilter) ([]*VerificationFinding, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	rows, err := d.QueryContext(ctx, `
SELECT f.id, COALESCE(f.task_id,0), COALESCE(t.description,''),
       COALESCE(f.name,''), COALESCE(f.vulnclass,''), COALESCE(f.severity,''),
       COALESCE(f.status,'pending'), v.confidence, COALESCE(v.status,''),
       COALESCE(v.dedupe_key,''), COALESCE(v.reasons,'[]'::jsonb), f.created_at
  FROM findings f
  JOIN finding_verification v ON v.finding_id = f.id
  LEFT JOIN tasks t ON t.id = f.task_id
 WHERE ($1 = '' OR v.status = $1)
   AND ($2 = 0 OR f.task_id = $2)
   AND v.confidence >= $3
 ORDER BY f.created_at DESC
 LIMIT $4`, f.Status, f.TaskID, f.MinConfidence, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*VerificationFinding, 0, 32)
	for rows.Next() {
		v := &VerificationFinding{}
		var reasons []byte
		if err := rows.Scan(&v.FindingID, &v.TaskID, &v.TaskDescription, &v.Name, &v.VulnClass,
			&v.Severity, &v.FindingStatus, &v.Confidence, &v.VerifyStatus, &v.DedupeKey,
			&reasons, &v.CreatedAt); err != nil {
			return nil, err
		}
		if len(reasons) > 0 {
			_ = json.Unmarshal(reasons, &v.Reasons)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// VerificationSummary counts gated findings per verify_status.
func (d *DB) VerificationSummary(ctx context.Context) (map[string]int, error) {
	rows, err := d.QueryContext(ctx, `
SELECT COALESCE(status,'') AS status, count(*)
  FROM finding_verification
 GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}
