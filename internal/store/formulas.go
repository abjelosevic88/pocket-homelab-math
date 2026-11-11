package store

import (
	"context"
	"encoding/json"
	"strings"
)

const formulaCols = `f.id, f.user_id, f.name, f.about, f.category, f.icon, f.code, f.inputs, f.outputs, f.from_key, f.pinned, f.source, f.source_ref, f.created_at, f.updated_at,
	(SELECT COUNT(1) FROM runs r WHERE r.formula_id = f.id), (SELECT MAX(created_at) FROM runs r WHERE r.formula_id = f.id)`

func scanFormula(sc interface{ Scan(...any) error }) (*Formula, error) {
	var f Formula
	var in, out string
	var pinned int
	if err := sc.Scan(&f.ID, &f.UserID, &f.Name, &f.About, &f.Category, &f.Icon, &f.Code, &in, &out, &f.FromKey, &pinned, &f.Source, &f.SourceRef, &f.CreatedAt, &f.UpdatedAt, &f.Runs, &f.LastRun); err != nil {
		return nil, err
	}
	f.Pinned = pinned == 1
	f.Inputs, f.Outputs = rawOr(in, "[]"), rawOr(out, "[]")
	return &f, nil
}

// Formulas lists one person's saved calculations.
func (s *Store) Formulas(ctx context.Context, uid int64, q string) ([]Formula, error) {
	query := `SELECT ` + formulaCols + ` FROM formulas f WHERE f.user_id = ?`
	args := []any{uid}
	if q = strings.TrimSpace(q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		query += ` AND (lower(f.name) LIKE ? OR lower(f.about) LIKE ? OR lower(f.category) LIKE ? OR lower(f.code) LIKE ?)`
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY f.pinned DESC, f.name COLLATE NOCASE`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Formula{}
	for rows.Next() {
		f, err := scanFormula(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// GetFormula loads one.
func (s *Store) GetFormula(ctx context.Context, uid, id int64) (*Formula, error) {
	f, err := scanFormula(s.DB.QueryRowContext(ctx, `SELECT `+formulaCols+` FROM formulas f WHERE f.id = ? AND f.user_id = ?`, id, uid))
	return f, notFound(err)
}

// FormulaByRef finds what an app saved before.
func (s *Store) FormulaByRef(ctx context.Context, uid int64, source, ref string) (*Formula, error) {
	f, err := scanFormula(s.DB.QueryRowContext(ctx, `SELECT `+formulaCols+` FROM formulas f WHERE f.user_id = ? AND f.source = ? AND f.source_ref = ?`, uid, source, ref))
	return f, notFound(err)
}

// CreateFormula saves a new one.
func (s *Store) CreateFormula(ctx context.Context, f *Formula) error {
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO formulas (user_id, name, about, category, icon, code, inputs, outputs, from_key, pinned, source, source_ref, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.UserID, f.Name, f.About, f.Category, f.Icon, f.Code, string(rawOr(string(f.Inputs), "[]")), string(rawOr(string(f.Outputs), "[]")), f.FromKey, boolInt(f.Pinned), f.Source, f.SourceRef, firstNonEmpty(f.CreatedAt, now), now)
	if err != nil {
		return conflict(err)
	}
	f.ID, _ = res.LastInsertId()
	f.CreatedAt, f.UpdatedAt = firstNonEmpty(f.CreatedAt, now), now
	return nil
}

// UpdateFormula saves changes.
func (s *Store) UpdateFormula(ctx context.Context, f *Formula) error {
	now := Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE formulas SET name = ?, about = ?, category = ?, icon = ?, code = ?, inputs = ?, outputs = ?, pinned = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		f.Name, f.About, f.Category, f.Icon, f.Code, string(rawOr(string(f.Inputs), "[]")), string(rawOr(string(f.Outputs), "[]")), boolInt(f.Pinned), now, f.ID, f.UserID)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	f.UpdatedAt = now
	return nil
}

// DeleteFormula removes one (its kept answers stay, with the name).
func (s *Store) DeleteFormula(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM formulas WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- kept answers ----

const runCols = `id, user_id, formula_id, formula_key, formula_name, title, inputs, outputs, main, note, source, created_at`

func scanRun(sc interface{ Scan(...any) error }) (*Run, error) {
	var r Run
	var in, out string
	if err := sc.Scan(&r.ID, &r.UserID, &r.FormulaID, &r.FormulaKey, &r.FormulaName, &r.Title, &in, &out, &r.Main, &r.Note, &r.Source, &r.CreatedAt); err != nil {
		return nil, err
	}
	r.Inputs, r.Outputs = rawOr(in, "{}"), rawOr(out, "[]")
	return &r, nil
}

// RunFilter narrows kept answers.
type RunFilter struct {
	FormulaID  int64
	FormulaKey string
	From, To   string // created_at range (UTC ISO)
	Limit      int
}

// Runs lists kept answers, newest first.
func (s *Store) Runs(ctx context.Context, uid int64, f RunFilter) ([]Run, error) {
	q := `SELECT ` + runCols + ` FROM runs WHERE user_id = ?`
	args := []any{uid}
	if f.FormulaID > 0 {
		q += ` AND formula_id = ?`
		args = append(args, f.FormulaID)
	}
	if f.FormulaKey != "" {
		q += ` AND formula_key = ?`
		args = append(args, f.FormulaKey)
	}
	if f.From != "" {
		q += ` AND created_at >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND created_at < ?`
		args = append(args, f.To)
	}
	q += ` ORDER BY created_at DESC, id DESC`
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 200
	}
	q += ` LIMIT ?`
	args = append(args, f.Limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetRun loads one kept answer.
func (s *Store) GetRun(ctx context.Context, uid, id int64) (*Run, error) {
	r, err := scanRun(s.DB.QueryRowContext(ctx, `SELECT `+runCols+` FROM runs WHERE id = ? AND user_id = ?`, id, uid))
	return r, notFound(err)
}

// CreateRun keeps an answer.
func (s *Store) CreateRun(ctx context.Context, r *Run) error {
	now := firstNonEmpty(r.CreatedAt, Now())
	var fid any
	if r.FormulaID != nil && *r.FormulaID > 0 {
		fid = *r.FormulaID
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO runs (user_id, formula_id, formula_key, formula_name, title, inputs, outputs, main, note, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.UserID, fid, r.FormulaKey, r.FormulaName, r.Title, string(rawOr(string(r.Inputs), "{}")), string(rawOr(string(r.Outputs), "[]")), r.Main, r.Note, r.Source, now)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	r.CreatedAt = now
	return nil
}

// UpdateRun changes what a kept answer was for.
func (s *Store) UpdateRun(ctx context.Context, r *Run) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE runs SET title = ?, note = ? WHERE id = ? AND user_id = ?`, r.Title, r.Note, r.ID, r.UserID)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRun forgets a kept answer.
func (s *Store) DeleteRun(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM runs WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- practice ----

const practiceCols = `id, user_id, learner, skill, level, questions, correct, seconds, details, day, created_at`

func scanPractice(sc interface{ Scan(...any) error }) (*Practice, error) {
	var p Practice
	var d string
	if err := sc.Scan(&p.ID, &p.UserID, &p.Learner, &p.Skill, &p.Level, &p.Questions, &p.Correct, &p.Seconds, &d, &p.Day, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.Details = rawOr(d, "[]")
	return &p, nil
}

// PracticeFilter narrows rounds.
type PracticeFilter struct {
	Learner  *string
	From, To string // days, inclusive
	Details  bool
	Limit    int
}

// Practices lists rounds, newest first.
func (s *Store) Practices(ctx context.Context, uid int64, f PracticeFilter) ([]Practice, error) {
	q := `SELECT ` + practiceCols + ` FROM practice WHERE user_id = ?`
	args := []any{uid}
	if f.Learner != nil {
		q += ` AND learner = ?`
		args = append(args, *f.Learner)
	}
	if f.From != "" {
		q += ` AND day >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND day <= ?`
		args = append(args, f.To)
	}
	q += ` ORDER BY created_at DESC, id DESC`
	if f.Limit <= 0 || f.Limit > 5000 {
		f.Limit = 500
	}
	q += ` LIMIT ?`
	args = append(args, f.Limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Practice{}
	for rows.Next() {
		p, err := scanPractice(rows)
		if err != nil {
			return nil, err
		}
		if !f.Details {
			p.Details = json.RawMessage("[]")
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetPractice loads a round.
func (s *Store) GetPractice(ctx context.Context, uid, id int64) (*Practice, error) {
	p, err := scanPractice(s.DB.QueryRowContext(ctx, `SELECT `+practiceCols+` FROM practice WHERE id = ? AND user_id = ?`, id, uid))
	return p, notFound(err)
}

// CreatePractice keeps a finished round.
func (s *Store) CreatePractice(ctx context.Context, p *Practice) error {
	now := firstNonEmpty(p.CreatedAt, Now())
	res, err := s.DB.ExecContext(ctx, `INSERT INTO practice (user_id, learner, skill, level, questions, correct, seconds, details, day, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Learner, p.Skill, p.Level, p.Questions, p.Correct, p.Seconds, string(rawOr(string(p.Details), "[]")), p.Day, now)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	p.CreatedAt = now
	return nil
}

// DeletePractice removes a round.
func (s *Store) DeletePractice(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM practice WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return ErrNotFound
	}
	return nil
}

// QuestionsOn counts questions answered on a day, per learner.
func (s *Store) QuestionsOn(ctx context.Context, uid int64, day string) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT learner, SUM(questions) FROM practice WHERE user_id = ? AND day = ? GROUP BY learner`, uid, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var l string
		var n int
		if err := rows.Scan(&l, &n); err != nil {
			return nil, err
		}
		out[l] = n
	}
	return out, rows.Err()
}
