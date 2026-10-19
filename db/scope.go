package db

import (
	"context"
	"time"
)

// EngagementScope is the auditable authorization record (RoE) for an engagement.
type EngagementScope struct {
	ID         int64     `json:"id,string"`
	Name       string    `json:"name"`
	Targets    string    `json:"targets"`
	Exclusions string    `json:"exclusions"`
	Contact    string    `json:"contact"`
	StartsAt   string    `json:"starts_at"`
	EndsAt     string    `json:"ends_at"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

const engagementScopeCols = `id, name, targets, exclusions, contact, starts_at, ends_at, notes, created_at`

func scanEngagementScope(row interface{ Scan(...any) error }, v *EngagementScope) error {
	return row.Scan(&v.ID, &v.Name, &v.Targets, &v.Exclusions, &v.Contact, &v.StartsAt, &v.EndsAt, &v.Notes, &v.CreatedAt)
}

func (d *DB) ListEngagementScopes(ctx context.Context) ([]*EngagementScope, error) {
	rows, err := d.QueryContext(ctx, `SELECT `+engagementScopeCols+` FROM engagement_scope ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*EngagementScope, 0, 8)
	for rows.Next() {
		v := &EngagementScope{}
		if err := scanEngagementScope(rows, v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) CreateEngagementScope(ctx context.Context, in EngagementScope) (*EngagementScope, error) {
	v := &EngagementScope{}
	err := scanEngagementScope(d.QueryRowContext(ctx, `
INSERT INTO engagement_scope(name,targets,exclusions,contact,starts_at,ends_at,notes)
VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+engagementScopeCols,
		in.Name, in.Targets, in.Exclusions, in.Contact, in.StartsAt, in.EndsAt, in.Notes), v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *DB) UpdateEngagementScope(ctx context.Context, in EngagementScope) (*EngagementScope, error) {
	v := &EngagementScope{}
	err := scanEngagementScope(d.QueryRowContext(ctx, `
UPDATE engagement_scope
   SET name=$2, targets=$3, exclusions=$4, contact=$5, starts_at=$6, ends_at=$7, notes=$8
 WHERE id=$1 RETURNING `+engagementScopeCols,
		in.ID, in.Name, in.Targets, in.Exclusions, in.Contact, in.StartsAt, in.EndsAt, in.Notes), v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *DB) DeleteEngagementScope(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM engagement_scope WHERE id=$1`, id)
	return err
}
