package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

var cellKinds = []string{"calc", "text", "graph", "geometry", "data", "solve"}

const (
	maxCells    = 300
	maxCellBody = 200000
	maxCellData = 2 << 20
)

// ---- working out ----

// numberFormat turns the setting into the engine's format.
func numberFormat(st store.Settings) calc.Format {
	f := calc.Format{Digits: st.Digits, Group: ",", Point: "."}
	switch st.NumberFmt {
	case "1.234,5":
		f.Group, f.Point = ".", ","
	case "1 234,5":
		f.Group, f.Point = " ", ","
	case "1234.5":
		f.Group = ""
	}
	return f
}

func (s *Server) calcOptions(ctx context.Context, uid int64, nb *store.Notebook) calc.Options {
	st := s.settingsOf(ctx, uid)
	o := calc.Options{Units: st.Units, Deg: st.Angle != "rad", Rates: s.ratesFor(ctx, st), Today: time.Now(), Format: numberFormat(st)}
	if nb != nil {
		o.Units, o.Deg = nb.Units, nb.Angle != "rad"
	}
	return o
}

// recompute works out every calculation cell of a notebook in order (a
// variable set in one cell is known in the cells after it) and keeps the
// answers on the cells. It returns the answers by cell id.
func (s *Server) recompute(ctx context.Context, uid int64, nb *store.Notebook) (map[int64][]calc.Line, error) {
	cells, err := s.store.Cells(ctx, uid, nb.ID)
	if err != nil {
		return nil, err
	}
	o := s.calcOptions(ctx, uid, nb)
	env := o.New()
	out := map[int64][]calc.Line{}
	for _, c := range cells {
		if c.Kind != "calc" {
			continue
		}
		env.Break()
		lines := strings.Split(c.Body, "\n")
		res := make([]calc.Line, 0, len(lines))
		var sum []string
		for _, l := range lines {
			ln := env.RunLine(l, o.Format)
			res = append(res, ln)
			if ln.Text != "" && (ln.Kind == "value" || ln.Kind == "set") {
				sum = append(sum, strings.TrimSpace(l)+" = "+ln.Text)
			}
		}
		out[c.ID] = res
		b, _ := json.Marshal(res)
		if string(b) != string(c.Results) || strings.Join(sum, "\n") != c.Summary {
			if err := s.store.SetCellResults(ctx, uid, c.ID, string(b), clip(strings.Join(sum, "\n"), 20000)); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// ---- notebooks ----

func (s *Server) notebookOf(w http.ResponseWriter, r *http.Request, id int64) (*store.Notebook, bool) {
	nb, err := s.store.GetNotebook(r.Context(), actorID(r.Context()), id)
	if err != nil {
		s.fail(w, err, "notebook")
		return nil, false
	}
	return nb, true
}

func (s *Server) notebookURL(nb *store.Notebook) string { return s.link(fmt.Sprintf("/n/%d", nb.ID)) }

func (s *Server) shareURL(nb *store.Notebook) string {
	if !nb.Shared || nb.ShareKey == "" {
		return ""
	}
	return s.link("/s/" + nb.ShareKey)
}

func (s *Server) handleNotebooks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	list, err := s.store.Notebooks(ctx, uid, store.NotebookFilter{Q: q.Get("q"), Subject: q.Get("subject"), Archived: queryBool(r, "archived"), Limit: queryInt(r, "limit")})
	if err != nil {
		s.fail(w, err, "notebooks")
		return
	}
	subjects, _ := s.store.Subjects(ctx, uid)
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": list, "subjects": subjects})
}

type cellIn struct {
	Kind      *string          `json:"kind"`
	Title     *string          `json:"title"`
	Body      *string          `json:"body"`
	Data      *json.RawMessage `json:"data"`
	Summary   *string          `json:"summary"`
	Collapsed *bool            `json:"collapsed"`
	Position  *int             `json:"position"`
	After     *int64           `json:"after"`
}

func (in cellIn) apply(c *store.Cell) error {
	if in.Kind != nil {
		if !oneOf(*in.Kind, cellKinds) {
			return &userErr{"a cell is calc, text, graph, geometry, data or solve"}
		}
		c.Kind = *in.Kind
	}
	if in.Title != nil {
		c.Title = cleanText(*in.Title, maxLine)
	}
	if in.Body != nil {
		if len(*in.Body) > maxCellBody {
			return &userErr{"this cell is too long"}
		}
		c.Body = *in.Body
	}
	if in.Data != nil {
		d := *in.Data
		if len(d) > maxCellData {
			return &userErr{"this cell holds too much"}
		}
		if len(d) == 0 || string(d) == "null" {
			d = json.RawMessage("{}")
		}
		if !json.Valid(d) {
			return &userErr{"the cell's data is not JSON"}
		}
		c.Data = d
	}
	if in.Summary != nil && c.Kind != "calc" {
		c.Summary = clip(strings.TrimSpace(*in.Summary), 20000)
	}
	if in.Collapsed != nil {
		c.Collapsed = *in.Collapsed
	}
	if c.Kind == "text" {
		c.Summary = clip(c.Body, 20000)
	}
	return nil
}

type notebookIn struct {
	Title     *string   `json:"title"`
	Subject   *string   `json:"subject"`
	About     *string   `json:"about"`
	Color     *string   `json:"color"`
	Angle     *string   `json:"angle"`
	Units     *bool     `json:"units"`
	Tags      *[]string `json:"tags"`
	Pinned    *bool     `json:"pinned"`
	Archived  *bool     `json:"archived"`
	Template  string    `json:"template"`
	SourceRef string    `json:"source_ref"`
	Cells     []cellIn  `json:"cells"`
}

func (in notebookIn) apply(nb *store.Notebook) error {
	if in.Title != nil {
		nb.Title = cleanText(*in.Title, maxName)
	}
	if in.Subject != nil {
		nb.Subject = cleanText(*in.Subject, 60)
	}
	if in.About != nil {
		nb.About = cleanText(*in.About, 2000)
	}
	if in.Color != nil {
		c := strings.TrimSpace(*in.Color)
		if c != "" && !reColor.MatchString(c) {
			return &userErr{"a colour is like #3554a5"}
		}
		nb.Color = c
	}
	if in.Angle != nil {
		if *in.Angle != "deg" && *in.Angle != "rad" {
			return &userErr{"angles are deg or rad"}
		}
		nb.Angle = *in.Angle
	}
	if in.Units != nil {
		nb.Units = *in.Units
	}
	if in.Tags != nil {
		tags := []string{}
		seen := map[string]bool{}
		for _, t := range *in.Tags {
			t = cleanText(t, 40)
			if t != "" && !seen[strings.ToLower(t)] && len(tags) < 20 {
				seen[strings.ToLower(t)] = true
				tags = append(tags, t)
			}
		}
		nb.Tags = tags
	}
	if in.Pinned != nil {
		nb.Pinned = *in.Pinned
	}
	if in.Archived != nil {
		nb.Archived = *in.Archived
	}
	return nil
}

func (s *Server) handleCreateNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in notebookIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	src := sourceOf(ctx)
	ref := cleanText(in.SourceRef, 200)
	if ref != "" {
		if old, err := s.store.NotebookByRef(ctx, uid, src, ref); err == nil {
			s.writeNotebook(w, r, old, http.StatusOK)
			return
		}
	}
	st := s.settingsOf(ctx, uid)
	nb := &store.Notebook{UserID: uid, Angle: st.Angle, Units: st.Units, Source: src, SourceRef: ref}
	tmpl, hasTmpl := templateByKey(in.Template)
	if hasTmpl {
		nb.Title, nb.Subject, nb.About = tmpl.Title, tmpl.Subject, tmpl.About
		if tmpl.Angle != "" {
			nb.Angle = tmpl.Angle
		}
	}
	if err := in.apply(nb); err != nil {
		s.userFail(w, err, "notebook")
		return
	}
	if nb.Title == "" {
		nb.Title = "Untitled notebook"
	}
	if len(in.Cells) > maxCells {
		writeErr(w, http.StatusBadRequest, "too many cells")
		return
	}
	cells := []store.Cell{}
	for _, ci := range in.Cells {
		c := store.Cell{Kind: "calc"}
		if err := ci.apply(&c); err != nil {
			s.userFail(w, err, "cell")
			return
		}
		cells = append(cells, c)
	}
	if len(cells) == 0 && hasTmpl {
		cells = tmpl.cells()
	}
	if len(cells) == 0 {
		cells = []store.Cell{{Kind: "calc"}}
	}
	if err := s.store.CreateNotebook(ctx, nb); err != nil {
		s.fail(w, err, "create notebook")
		return
	}
	for i := range cells {
		c := cells[i]
		c.UserID, c.NotebookID, c.Position, c.Source = uid, nb.ID, -1, src
		if err := s.store.CreateCell(ctx, &c); err != nil {
			s.fail(w, err, "create cell")
			return
		}
	}
	if _, err := s.recompute(ctx, uid, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	s.publish(ctx, uid, "math.notebook_created", fmt.Sprint(nb.ID), map[string]any{"notebook_id": nb.ID, "title": nb.Title, "url": s.notebookURL(nb)})
	s.writeNotebook(w, r, nb, http.StatusCreated)
}

func (s *Server) writeNotebook(w http.ResponseWriter, r *http.Request, nb *store.Notebook, status int) {
	cells, err := s.store.Cells(r.Context(), nb.UserID, nb.ID)
	if err != nil {
		s.fail(w, err, "cells")
		return
	}
	writeJSON(w, status, map[string]any{"notebook": nb, "cells": cells, "url": s.notebookURL(nb), "share_url": s.shareURL(nb)})
}

func (s *Server) handleGetNotebook(w http.ResponseWriter, r *http.Request) {
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if linkApp(r.Context()) == "" {
		s.store.OpenedNotebook(r.Context(), nb.UserID, nb.ID)
	}
	// answers are worked out again on open: exchange rates and "today" move
	if _, err := s.recompute(r.Context(), nb.UserID, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	s.writeNotebook(w, r, nb, http.StatusOK)
}

func (s *Server) handleUpdateNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if !mayChange(ctx, nb.Source) {
		writeErr(w, http.StatusForbidden, "this notebook was not made by that app")
		return
	}
	var in notebookIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	reworks := in.Angle != nil && *in.Angle != nb.Angle || in.Units != nil && *in.Units != nb.Units
	if err := in.apply(nb); err != nil {
		s.userFail(w, err, "notebook")
		return
	}
	if nb.Title == "" {
		nb.Title = "Untitled notebook"
	}
	if err := s.store.UpdateNotebook(ctx, nb); err != nil {
		s.fail(w, err, "save notebook")
		return
	}
	if reworks {
		if _, err := s.recompute(ctx, nb.UserID, nb); err != nil {
			s.fail(w, err, "work out")
			return
		}
	}
	s.writeNotebook(w, r, nb, http.StatusOK)
}

func (s *Server) handleDeleteNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if !mayChange(ctx, nb.Source) {
		writeErr(w, http.StatusForbidden, "this notebook was not made by that app")
		return
	}
	if err := s.store.DeleteNotebook(ctx, nb.UserID, nb.ID); err != nil {
		s.fail(w, err, "delete notebook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCopyNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	cells, err := s.store.Cells(ctx, nb.UserID, nb.ID)
	if err != nil {
		s.fail(w, err, "cells")
		return
	}
	cp := *nb
	cp.ID, cp.Title, cp.Shared, cp.ShareKey, cp.Pinned, cp.Archived, cp.Source, cp.SourceRef, cp.CreatedAt = 0, clip(nb.Title+" (copy)", maxName), false, "", false, false, "", "", ""
	if err := s.store.CreateNotebook(ctx, &cp); err != nil {
		s.fail(w, err, "copy")
		return
	}
	for _, c := range cells {
		c.ID, c.NotebookID, c.Position, c.Source = 0, cp.ID, -1, ""
		if err := s.store.CreateCell(ctx, &c); err != nil {
			s.fail(w, err, "copy")
			return
		}
	}
	s.writeNotebook(w, r, &cp, http.StatusCreated)
}

func (s *Server) handleShareNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	var in struct {
		Shared bool `json:"shared"`
		New    bool `json:"new"` // a new link: the old one stops working
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	nb.Shared = in.Shared
	if in.Shared && (nb.ShareKey == "" || in.New) {
		nb.ShareKey = newToken()[:20]
	}
	if err := s.store.UpdateNotebook(ctx, nb); err != nil {
		s.fail(w, err, "share")
		return
	}
	if in.Shared {
		s.publish(ctx, nb.UserID, "math.shared", fmt.Sprint(nb.ID), map[string]any{"notebook_id": nb.ID, "title": nb.Title, "url": s.shareURL(nb)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebook": nb, "share_url": s.shareURL(nb)})
}

// handleShared is a notebook seen through its share link (no account).
func (s *Server) handleShared(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, err := s.store.NotebookByShareKey(ctx, chiParam(r, "key"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this link does not work any more")
		return
	}
	if _, err := s.recompute(ctx, nb.UserID, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	cells, err := s.store.Cells(ctx, nb.UserID, nb.ID)
	if err != nil {
		s.fail(w, err, "cells")
		return
	}
	owner := ""
	if u, err := s.store.GetUser(ctx, nb.UserID); err == nil {
		owner = s.actorName(u)
	}
	st := s.settingsOf(ctx, nb.UserID)
	writeJSON(w, http.StatusOK, map[string]any{
		"notebook": map[string]any{"title": nb.Title, "about": nb.About, "subject": nb.Subject, "angle": nb.Angle, "units": nb.Units, "updated_at": nb.UpdatedAt, "color": nb.Color},
		"cells":    cells, "owner": owner, "number_fmt": st.NumberFmt, "digits": st.Digits, "rates": s.ratesFor(ctx, st),
	})
}

func (s *Server) handleNotebookMarkdown(w http.ResponseWriter, r *http.Request) {
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if _, err := s.recompute(r.Context(), nb.UserID, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	md := s.notebookMarkdown(r.Context(), nb)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileSafe(nb.Title)+`.md"`)
	_, _ = w.Write([]byte(md))
}

// notebookMarkdown writes a notebook as words: calculations with their
// answers, text as it is, the rest described.
func (s *Server) notebookMarkdown(ctx context.Context, nb *store.Notebook) string {
	cells, _ := s.store.Cells(ctx, nb.UserID, nb.ID)
	var b strings.Builder
	b.WriteString("# " + nb.Title + "\n\n")
	if nb.About != "" {
		b.WriteString(nb.About + "\n\n")
	}
	for _, c := range cells {
		if c.Title != "" {
			b.WriteString("## " + c.Title + "\n\n")
		}
		switch c.Kind {
		case "text":
			b.WriteString(strings.TrimSpace(c.Body) + "\n\n")
		case "calc":
			var res []calc.Line
			_ = json.Unmarshal(c.Results, &res)
			lines := strings.Split(c.Body, "\n")
			w := 0
			for _, l := range lines {
				if n := len([]rune(strings.TrimRight(l, " "))); n > w && n < 60 {
					w = n
				}
			}
			b.WriteString("```\n")
			for i, l := range lines {
				l = strings.TrimRight(l, " ")
				if i < len(res) && res[i].Text != "" && (res[i].Kind == "value" || res[i].Kind == "set") {
					pad := w - len([]rune(l))
					if pad < 1 {
						pad = 1
					}
					l += strings.Repeat(" ", pad) + "  = " + res[i].Text
				} else if i < len(res) && res[i].Kind == "error" {
					l += "   ⚠ " + res[i].Error
				}
				b.WriteString(l + "\n")
			}
			b.WriteString("```\n\n")
		default:
			if c.Summary != "" {
				b.WriteString(fmt.Sprintf("*%s:*\n\n```\n%s\n```\n\n", kindName(c.Kind), strings.TrimSpace(c.Summary)))
			} else {
				b.WriteString(fmt.Sprintf("*(%s)*\n\n", kindName(c.Kind)))
			}
		}
	}
	return b.String()
}

func kindName(k string) string {
	switch k {
	case "graph":
		return "Graph"
	case "geometry":
		return "Geometry drawing"
	case "data":
		return "Table of data"
	case "solve":
		return "Equations"
	case "text":
		return "Text"
	}
	return "Calculation"
}

func fileSafe(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		s = "notebook"
	}
	return clip(s, 80)
}

// ---- cells ----

func (s *Server) handleCreateCell(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if !mayChange(ctx, nb.Source) {
		writeErr(w, http.StatusForbidden, "this notebook was not made by that app")
		return
	}
	var in cellIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	cells, _ := s.store.Cells(ctx, nb.UserID, nb.ID)
	if len(cells) >= maxCells {
		writeErr(w, http.StatusBadRequest, "a notebook holds at most 300 cells")
		return
	}
	c := store.Cell{UserID: nb.UserID, NotebookID: nb.ID, Kind: "calc", Position: -1, Source: sourceOf(ctx)}
	if err := in.apply(&c); err != nil {
		s.userFail(w, err, "cell")
		return
	}
	if in.Position != nil {
		c.Position = *in.Position
	}
	if in.After != nil {
		c.Position = 0
		for _, x := range cells {
			if x.ID == *in.After {
				c.Position = x.Position + 1
			}
		}
	}
	if err := s.store.CreateCell(ctx, &c); err != nil {
		s.fail(w, err, "add cell")
		return
	}
	s.afterCellChange(w, r, nb, &c, http.StatusCreated)
}

// afterCellChange works the notebook out again and answers with the cell,
// every calculation's answers and the new revision.
func (s *Server) afterCellChange(w http.ResponseWriter, r *http.Request, nb *store.Notebook, c *store.Cell, status int) {
	ctx := r.Context()
	rev, err := s.store.TouchNotebook(ctx, nb.UserID, nb.ID)
	if err != nil {
		s.fail(w, err, "save")
		return
	}
	res, err := s.recompute(ctx, nb.UserID, nb)
	if err != nil {
		s.fail(w, err, "work out")
		return
	}
	if c != nil {
		if fresh, err := s.store.GetCell(ctx, nb.UserID, c.ID); err == nil {
			c = fresh
		}
	}
	writeJSON(w, status, map[string]any{"cell": c, "results": res, "revision": rev})
}

func (s *Server) cellOf(w http.ResponseWriter, r *http.Request) (*store.Cell, *store.Notebook, bool) {
	ctx := r.Context()
	c, err := s.store.GetCell(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "cell")
		return nil, nil, false
	}
	nb, err := s.store.GetNotebook(ctx, c.UserID, c.NotebookID)
	if err != nil {
		s.fail(w, err, "notebook")
		return nil, nil, false
	}
	if !mayChange(ctx, c.Source) {
		writeErr(w, http.StatusForbidden, "this cell was not made by that app")
		return nil, nil, false
	}
	return c, nb, true
}

func (s *Server) handleUpdateCell(w http.ResponseWriter, r *http.Request) {
	c, nb, ok := s.cellOf(w, r)
	if !ok {
		return
	}
	var in cellIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := in.apply(c); err != nil {
		s.userFail(w, err, "cell")
		return
	}
	if err := s.store.UpdateCell(r.Context(), c); err != nil {
		s.fail(w, err, "save cell")
		return
	}
	s.afterCellChange(w, r, nb, c, http.StatusOK)
}

func (s *Server) handleDeleteCell(w http.ResponseWriter, r *http.Request) {
	c, nb, ok := s.cellOf(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteCell(r.Context(), c.UserID, c.ID); err != nil {
		s.fail(w, err, "delete cell")
		return
	}
	s.afterCellChange(w, r, nb, nil, http.StatusOK)
}

func (s *Server) handleCopyCell(w http.ResponseWriter, r *http.Request) {
	c, nb, ok := s.cellOf(w, r)
	if !ok {
		return
	}
	cp := *c
	cp.ID, cp.Position, cp.Source = 0, c.Position+1, sourceOf(r.Context())
	if err := s.store.CreateCell(r.Context(), &cp); err != nil {
		s.fail(w, err, "copy cell")
		return
	}
	s.afterCellChange(w, r, nb, &cp, http.StatusCreated)
}

func (s *Server) handleReorderCells(w http.ResponseWriter, r *http.Request) {
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decode(r, &in); err != nil || len(in.IDs) > maxCells {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.ReorderCells(r.Context(), nb.UserID, nb.ID, in.IDs); err != nil {
		s.fail(w, err, "order")
		return
	}
	s.afterCellChange(w, r, nb, nil, http.StatusOK)
}

// ---- one calculation on its own ----

// handleCalculate works out an expression (or several lines): for the quick
// calculator, other apps, Shortcuts and Pocket AI.
func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Expr  string `json:"expr"`
		Angle string `json:"angle"`
		Units *bool  `json:"units"`
	}
	if r.Method == http.MethodGet {
		in.Expr, in.Angle = r.URL.Query().Get("q"), r.URL.Query().Get("angle")
	} else if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(in.Expr) > 20000 {
		writeErr(w, http.StatusBadRequest, "that is too long")
		return
	}
	o := s.calcOptions(ctx, actorID(ctx), nil)
	if in.Angle == "deg" || in.Angle == "rad" {
		o.Deg = in.Angle == "deg"
	}
	if in.Units != nil {
		o.Units = *in.Units
	}
	lines, _ := calc.EvalSheet(strings.Split(in.Expr, "\n"), o)
	last := calc.Calculate(in.Expr, o)
	writeJSON(w, http.StatusOK, map[string]any{"result": last, "lines": lines})
}

// handleConvert: value + from unit → to unit.
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	v, from, to := strings.TrimSpace(q.Get("value")), strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	if v == "" {
		v = "1"
	}
	if from == "" || to == "" {
		writeErr(w, http.StatusBadRequest, "say from which unit to which")
		return
	}
	o := s.calcOptions(ctx, actorID(ctx), nil)
	o.Units = true
	expr := v + " " + from + " to " + to
	if strings.ContainsAny(from, "/*^ ") {
		expr = v + " (" + from + ") to " + to
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": calc.Calculate(expr, o), "expr": expr})
}

func (s *Server) handleReference(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	rates := s.ratesFor(ctx, st)
	codes := make([]string, 0, len(rates))
	for c := range rates {
		codes = append(codes, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"kinds": calc.ConverterKinds, "functions": calc.Builtins, "units": calc.UnitNames(), "currencies": sortedStrings(codes), "rates": rates, "rates_info": s.ratesInfo(ctx)})
}
