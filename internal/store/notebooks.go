package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

const notebookCols = `id, user_id, title, subject, about, color, angle, units, tags, pinned, archived, shared, share_key, revision, source, source_ref, opened_at, created_at, updated_at`

func scanNotebook(sc interface{ Scan(...any) error }) (*Notebook, error) {
	var n Notebook
	var tags string
	var units, pinned, archived, shared int
	if err := sc.Scan(&n.ID, &n.UserID, &n.Title, &n.Subject, &n.About, &n.Color, &n.Angle, &units, &tags, &pinned, &archived, &shared, &n.ShareKey, &n.Revision, &n.Source, &n.SourceRef, &n.OpenedAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, err
	}
	n.Units, n.Pinned, n.Archived, n.Shared = units == 1, pinned == 1, archived == 1, shared == 1
	_ = json.Unmarshal([]byte(tags), &n.Tags)
	if n.Tags == nil {
		n.Tags = []string{}
	}
	n.Kinds = []string{}
	return &n, nil
}

// NotebookFilter narrows the list.
type NotebookFilter struct {
	Q        string
	Subject  string
	Archived bool
	Limit    int
}

// Notebooks lists one person's notebooks, pinned first, newest change first.
func (s *Store) Notebooks(ctx context.Context, uid int64, f NotebookFilter) ([]Notebook, error) {
	q := `SELECT ` + notebookCols + ` FROM notebooks WHERE user_id = ? AND archived = ?`
	args := []any{uid, boolInt(f.Archived)}
	if f.Subject != "" {
		q += ` AND subject = ?`
		args = append(args, f.Subject)
	}
	if t := strings.TrimSpace(f.Q); t != "" {
		like := "%" + strings.ToLower(t) + "%"
		q += ` AND (lower(title) LIKE ? OR lower(about) LIKE ? OR lower(subject) LIKE ? OR id IN (SELECT notebook_id FROM cells WHERE user_id = ? AND (lower(body) LIKE ? OR lower(title) LIKE ? OR lower(summary) LIKE ?)))`
		args = append(args, like, like, like, uid, like, like, like)
	}
	q += ` ORDER BY pinned DESC, updated_at DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var out []Notebook
	for rows.Next() {
		n, err := scanNotebook(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// counts, kinds and a first line to show
	for i := range out {
		r2, err := s.DB.QueryContext(ctx, `SELECT kind, body, summary FROM cells WHERE notebook_id = ? AND user_id = ? ORDER BY position, id`, out[i].ID, uid)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for r2.Next() {
			var kind, body, summary string
			if err := r2.Scan(&kind, &body, &summary); err != nil {
				r2.Close()
				return nil, err
			}
			out[i].Cells++
			if !seen[kind] {
				seen[kind] = true
				out[i].Kinds = append(out[i].Kinds, kind)
			}
			if out[i].Preview == "" {
				for _, l := range strings.Split(firstNonEmpty(body, summary), "\n") {
					if l = strings.TrimSpace(strings.TrimLeft(l, "#")); l != "" {
						out[i].Preview = clipText(l, 140)
						break
					}
				}
			}
		}
		r2.Close()
	}
	if out == nil {
		out = []Notebook{}
	}
	return out, nil
}

// Subjects lists the folders in use with how many notebooks each.
func (s *Store) Subjects(ctx context.Context, uid int64) ([]map[string]any, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT subject, COUNT(1) FROM notebooks WHERE user_id = ? AND archived = 0 AND subject <> '' GROUP BY subject ORDER BY subject COLLATE NOCASE`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var n string
		var c int
		if err := rows.Scan(&n, &c); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": n, "count": c})
	}
	return out, rows.Err()
}

// GetNotebook loads one of the person's notebooks.
func (s *Store) GetNotebook(ctx context.Context, uid, id int64) (*Notebook, error) {
	n, err := scanNotebook(s.DB.QueryRowContext(ctx, `SELECT `+notebookCols+` FROM notebooks WHERE id = ? AND user_id = ?`, id, uid))
	return n, notFound(err)
}

// NotebookByShareKey loads a shared notebook for its share link.
func (s *Store) NotebookByShareKey(ctx context.Context, key string) (*Notebook, error) {
	if key == "" {
		return nil, ErrNotFound
	}
	n, err := scanNotebook(s.DB.QueryRowContext(ctx, `SELECT `+notebookCols+` FROM notebooks WHERE share_key = ? AND shared = 1`, key))
	return n, notFound(err)
}

// NotebookByRef finds what an app made before (its own reference).
func (s *Store) NotebookByRef(ctx context.Context, uid int64, source, ref string) (*Notebook, error) {
	n, err := scanNotebook(s.DB.QueryRowContext(ctx, `SELECT `+notebookCols+` FROM notebooks WHERE user_id = ? AND source = ? AND source_ref = ?`, uid, source, ref))
	return n, notFound(err)
}

// CreateNotebook adds a notebook.
func (s *Store) CreateNotebook(ctx context.Context, n *Notebook) error {
	if n.Tags == nil {
		n.Tags = []string{}
	}
	if n.Angle == "" {
		n.Angle = "deg"
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO notebooks (user_id, title, subject, about, color, angle, units, tags, pinned, archived, shared, share_key, source, source_ref, opened_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.UserID, n.Title, n.Subject, n.About, n.Color, n.Angle, boolInt(n.Units), toJSON(n.Tags), boolInt(n.Pinned), boolInt(n.Archived), boolInt(n.Shared), n.ShareKey, n.Source, n.SourceRef, now, firstNonEmpty(n.CreatedAt, now), now)
	if err != nil {
		return conflict(err)
	}
	n.ID, _ = res.LastInsertId()
	n.CreatedAt, n.UpdatedAt, n.Revision = firstNonEmpty(n.CreatedAt, now), now, 1
	return nil
}

// UpdateNotebook saves the notebook's own fields and bumps its revision.
func (s *Store) UpdateNotebook(ctx context.Context, n *Notebook) error {
	if n.Tags == nil {
		n.Tags = []string{}
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE notebooks SET title = ?, subject = ?, about = ?, color = ?, angle = ?, units = ?, tags = ?, pinned = ?, archived = ?, shared = ?, share_key = ?, revision = revision + 1, updated_at = ? WHERE id = ? AND user_id = ?`,
		n.Title, n.Subject, n.About, n.Color, n.Angle, boolInt(n.Units), toJSON(n.Tags), boolInt(n.Pinned), boolInt(n.Archived), boolInt(n.Shared), n.ShareKey, now, n.ID, n.UserID)
	if err != nil {
		return conflict(err)
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	n.UpdatedAt = now
	n.Revision++
	return nil
}

// TouchNotebook marks a change inside (a cell) and gives the new revision.
func (s *Store) TouchNotebook(ctx context.Context, uid, id int64) (int64, error) {
	var rev int64
	err := s.DB.QueryRowContext(ctx, `UPDATE notebooks SET revision = revision + 1, updated_at = ? WHERE id = ? AND user_id = ? RETURNING revision`, Now(), id, uid).Scan(&rev)
	return rev, notFound(err)
}

// OpenedNotebook notes when a notebook was last opened.
func (s *Store) OpenedNotebook(ctx context.Context, uid, id int64) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE notebooks SET opened_at = ? WHERE id = ? AND user_id = ?`, Now(), id, uid)
}

// DeleteNotebook removes a notebook with its cells.
func (s *Store) DeleteNotebook(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM notebooks WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- cells ----

const cellCols = `id, user_id, notebook_id, position, kind, title, body, data, results, summary, collapsed, source, created_at, updated_at`

func scanCell(sc interface{ Scan(...any) error }) (*Cell, error) {
	var c Cell
	var data, results string
	var collapsed int
	if err := sc.Scan(&c.ID, &c.UserID, &c.NotebookID, &c.Position, &c.Kind, &c.Title, &c.Body, &data, &results, &c.Summary, &collapsed, &c.Source, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Collapsed = collapsed == 1
	c.Data, c.Results = rawOr(data, "{}"), rawOr(results, "[]")
	return &c, nil
}

func rawOr(s, def string) json.RawMessage {
	if s == "" || !json.Valid([]byte(s)) {
		return json.RawMessage(def)
	}
	return json.RawMessage(s)
}

// Cells lists a notebook's cells in order.
func (s *Store) Cells(ctx context.Context, uid, notebookID int64) ([]Cell, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+cellCols+` FROM cells WHERE notebook_id = ? AND user_id = ? ORDER BY position, id`, notebookID, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cell{}
	for rows.Next() {
		c, err := scanCell(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// GetCell loads one cell of the person's.
func (s *Store) GetCell(ctx context.Context, uid, id int64) (*Cell, error) {
	c, err := scanCell(s.DB.QueryRowContext(ctx, `SELECT `+cellCols+` FROM cells WHERE id = ? AND user_id = ?`, id, uid))
	return c, notFound(err)
}

// CreateCell adds a cell at a position (the others move down), or at the end
// when position < 0.
func (s *Store) CreateCell(ctx context.Context, c *Cell) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var maxPos sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(position) FROM cells WHERE notebook_id = ? AND user_id = ?`, c.NotebookID, c.UserID).Scan(&maxPos); err != nil {
		return err
	}
	end := 0
	if maxPos.Valid {
		end = int(maxPos.Int64) + 1
	}
	if c.Position < 0 || c.Position > end {
		c.Position = end
	} else if _, err := tx.ExecContext(ctx, `UPDATE cells SET position = position + 1 WHERE notebook_id = ? AND user_id = ? AND position >= ?`, c.NotebookID, c.UserID, c.Position); err != nil {
		return err
	}
	if len(c.Data) == 0 {
		c.Data = json.RawMessage("{}")
	}
	if len(c.Results) == 0 {
		c.Results = json.RawMessage("[]")
	}
	now := Now()
	res, err := tx.ExecContext(ctx, `INSERT INTO cells (user_id, notebook_id, position, kind, title, body, data, results, summary, collapsed, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.UserID, c.NotebookID, c.Position, c.Kind, c.Title, c.Body, string(c.Data), string(c.Results), c.Summary, boolInt(c.Collapsed), c.Source, now, now)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	c.CreatedAt, c.UpdatedAt = now, now
	return tx.Commit()
}

// UpdateCell saves a cell.
func (s *Store) UpdateCell(ctx context.Context, c *Cell) error {
	now := Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE cells SET kind = ?, title = ?, body = ?, data = ?, results = ?, summary = ?, collapsed = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		c.Kind, c.Title, c.Body, string(c.Data), string(c.Results), c.Summary, boolInt(c.Collapsed), now, c.ID, c.UserID)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	c.UpdatedAt = now
	return nil
}

// SetCellResults stores worked-out answers (no change of updated_at).
func (s *Store) SetCellResults(ctx context.Context, uid, id int64, results, summary string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE cells SET results = ?, summary = ? WHERE id = ? AND user_id = ?`, results, summary, id, uid)
	return err
}

// DeleteCell removes a cell and closes the gap.
func (s *Store) DeleteCell(ctx context.Context, uid, id int64) error {
	c, err := s.GetCell(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM cells WHERE id = ? AND user_id = ?`, id, uid); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE cells SET position = position - 1 WHERE notebook_id = ? AND user_id = ? AND position > ?`, c.NotebookID, uid, c.Position)
	return err
}

// ReorderCells puts the cells in this order (ids not listed keep theirs at the end).
func (s *Store) ReorderCells(ctx context.Context, uid, notebookID int64, ids []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE cells SET position = ? WHERE id = ? AND notebook_id = ? AND user_id = ?`, i, id, notebookID, uid); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cells SET position = position + ? WHERE notebook_id = ? AND user_id = ? AND id NOT IN (SELECT value FROM json_each(?))`, len(ids), notebookID, uid, toJSON(ids)); err != nil {
		return err
	}
	return tx.Commit()
}

// CellsChangedOn lists cells changed on a local day (for the journal).
func (s *Store) CellsChangedOn(ctx context.Context, uid int64, from, to string) ([]Cell, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+cellCols+` FROM cells WHERE user_id = ? AND updated_at >= ? AND updated_at < ? ORDER BY updated_at`, uid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cell{}
	for rows.Next() {
		c, err := scanCell(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func clipText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
