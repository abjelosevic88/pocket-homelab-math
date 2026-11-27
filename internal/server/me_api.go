package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// handleHome is the start page: recent notebooks, pinned calculations,
// kept answers and today's practice.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	recent, err := s.store.Notebooks(ctx, uid, store.NotebookFilter{Limit: 8})
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	mine, _ := s.store.Formulas(ctx, uid, "")
	pinned := []store.Formula{}
	for _, f := range mine {
		if f.Pinned {
			pinned = append(pinned, f)
		}
	}
	runs, _ := s.store.Runs(ctx, uid, store.RunFilter{Limit: 6})
	// the built-ins used most
	used := map[string]int{}
	if all, err := s.store.Runs(ctx, uid, store.RunFilter{Limit: 1000}); err == nil {
		for _, r := range all {
			if r.FormulaKey != "" {
				used[r.FormulaKey]++
			}
		}
	}
	favs := []store.Formula{}
	for _, b := range library {
		if used[b.Key] > 0 {
			f := b.asFormula()
			f.Runs = used[b.Key]
			favs = append(favs, f)
		}
	}
	sort.SliceStable(favs, func(i, j int) bool { return favs[i].Runs > favs[j].Runs })
	if len(favs) > 6 {
		favs = favs[:6]
	}
	st := s.settingsOf(ctx, uid)
	practice := []learnerStats{}
	for _, l := range append([]string{""}, st.Learners...) {
		ls := s.learnerStats(ctx, uid, l, st.DailyGoal)
		ls.Days = nil
		ls.Skills = nil
		practice = append(practice, ls)
	}
	counts := s.store.CountsFor(ctx, uid)
	templates := []map[string]any{}
	for _, t := range notebookTemplates {
		templates = append(templates, map[string]any{"key": t.Key, "name": t.Name, "about": t.About, "icon": t.Icon})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": recent, "pinned": pinned, "favourites": favs, "runs": runs, "practice": practice, "counts": counts, "templates": templates})
}

// handleTemplates lists the starts for a new notebook.
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, t := range notebookTemplates {
		out = append(out, map[string]any{"key": t.Key, "name": t.Name, "about": t.About, "icon": t.Icon})
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": out})
}

// handleSummary is a short overview for Pocket Day, AI and dashboards.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	counts := s.store.CountsFor(ctx, uid)
	recent, _ := s.store.Notebooks(ctx, uid, store.NotebookFilter{Limit: 1})
	var last map[string]any
	if len(recent) > 0 {
		last = map[string]any{"id": recent[0].ID, "title": recent[0].Title, "updated_at": recent[0].UpdatedAt, "url": s.notebookURL(&recent[0])}
	}
	st := s.settingsOf(ctx, uid)
	done, _ := s.store.QuestionsOn(ctx, uid, today())
	practice := []map[string]any{}
	for _, l := range append([]string{""}, st.Learners...) {
		practice = append(practice, map[string]any{"learner": firstNonEmpty(l, "you"), "today": done[l], "goal": st.DailyGoal})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": counts["notebooks"], "formulas": counts["formulas"], "answers": counts["runs"], "practice_rounds": counts["practice"],
		"last_notebook": last, "practice_today": practice, "url": s.link("/")})
}

// handleDay is what was worked out on a day: notebooks started or changed,
// answers kept, practice done. Shape for Pocket Day: items with a time.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := chiParam(r, "date")
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	from, to := day.UTC().Format(time.RFC3339Nano), day.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)
	items := []map[string]any{}
	at := func(ts string) string {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return ""
		}
		return t.Local().Format("15:04")
	}
	nbs, runs, rounds, _ := s.dayRows(ctx, uid, date, from, to)
	for _, d := range nbs {
		nb := d.nb
		what := "Worked on"
		if d.started {
			what = "Started"
		}
		items = append(items, map[string]any{"time": at(d.at), "kind": "notebook", "emoji": "🧮", "text": what + " “" + nb.Title + "”", "notebook_id": nb.ID, "url": s.notebookURL(nb)})
	}
	for i := len(runs) - 1; i >= 0; i-- {
		rn := runs[i]
		text := rn.FormulaName + ": " + rn.Main
		if rn.Title != "" {
			text = rn.Title + " — " + text
		}
		items = append(items, map[string]any{"time": at(rn.CreatedAt), "kind": "answer", "emoji": "✅", "text": clip(text, 200), "run_id": rn.ID, "url": s.link(fmt.Sprintf("/answers#%d", rn.ID))})
	}
	for i := len(rounds) - 1; i >= 0; i-- {
		p := rounds[i]
		who := ""
		if p.Learner != "" {
			who = p.Learner + ": "
		}
		items = append(items, map[string]any{"time": at(p.CreatedAt), "kind": "practice", "emoji": "✏️", "text": fmt.Sprintf("%sPractised %s — %d of %d right", who, skillName(p.Skill), p.Correct, p.Questions), "url": s.link("/practice")})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i]["time"].(string) < items[j]["time"].(string) })
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "items": items})
}

// dayNotebook is a notebook with a cell changed on a day: when the last
// change was, and whether the notebook was started that day.
type dayNotebook struct {
	nb      *store.Notebook
	at      string
	started bool
}

// dayRows is what the person worked out on a day: the notebooks with a cell
// changed between from and to (UTC, as stored), in the order they were first
// touched; the answers kept then and the practice rounds of that date, both
// newest first. /day and Pocket Day (capabilities.go) read the same rows.
// What could be read comes back even when one of the lists failed.
func (s *Server) dayRows(ctx context.Context, uid int64, date, from, to string) ([]dayNotebook, []store.Run, []store.Practice, error) {
	cells, err := s.store.CellsChangedOn(ctx, uid, from, to)
	byNb := map[int64]string{}
	order := []int64{}
	for _, c := range cells {
		if _, ok := byNb[c.NotebookID]; !ok {
			order = append(order, c.NotebookID)
		}
		byNb[c.NotebookID] = c.UpdatedAt
	}
	nbs := []dayNotebook{}
	for _, id := range order {
		nb, err := s.store.GetNotebook(ctx, uid, id)
		if err != nil {
			continue
		}
		nbs = append(nbs, dayNotebook{nb: nb, at: byNb[id], started: nb.CreatedAt >= from && nb.CreatedAt < to})
	}
	runs, e := s.store.Runs(ctx, uid, store.RunFilter{From: from, To: to, Limit: 100})
	if err == nil {
		err = e
	}
	rounds, e := s.store.Practices(ctx, uid, store.PracticeFilter{From: date, To: date, Limit: 100})
	if err == nil {
		err = e
	}
	return nbs, runs, rounds, err
}

func skillName(k string) string {
	switch k {
	case "add":
		return "adding"
	case "subtract":
		return "taking away"
	case "times":
		return "times"
	case "divide":
		return "dividing"
	case "tables":
		return "times tables"
	case "fractions":
		return "fractions"
	case "decimals":
		return "decimals"
	case "percent":
		return "percentages"
	case "equations":
		return "equations"
	case "powers":
		return "squares and roots"
	case "rounding":
		return "rounding"
	}
	return "a mix"
}

// handleSearch finds notebooks, calculations and kept answers by their
// words (for the command palette, Pocket Search and Pocket AI). Row shape:
// kind, id, title, subtitle, line, url, path.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []map[string]any{}
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": out})
		return
	}
	limit := clampInt(firstPos(queryInt(r, "limit"), 20), 1, 50)
	fd, err := s.find(ctx, uid, q, limit)
	if err != nil {
		s.fail(w, err, "search")
		return
	}
	if ln := fd.sum; ln != nil {
		path := "/?q=" + url.QueryEscape(q)
		out = append(out, map[string]any{"kind": "sum", "id": "sum", "title": q + " = " + ln.Text, "subtitle": "Worked out by Pocket Math", "line": strings.TrimPrefix(ln.Alt, "= "), "url": s.link(path), "path": path, "text": ln.Text, "number": ln.Number, "unit": ln.Unit})
	}
	for _, n := range fd.notebooks {
		nb := n.Notebook
		path := fmt.Sprintf("/n/%d", nb.ID)
		out = append(out, map[string]any{"kind": "notebook", "id": nb.ID, "title": nb.Title, "subtitle": firstNonEmpty(nb.Subject, "Notebook"), "line": clip(n.line, 200), "url": s.link(path), "path": path, "updated_at": nb.UpdatedAt})
	}
	for _, f := range fd.formulas {
		path := fmt.Sprintf("/f/%d", f.ID)
		out = append(out, map[string]any{"kind": "formula", "id": f.ID, "title": f.Name, "subtitle": "My calculation", "line": clip(f.About, 200), "url": s.link(path), "path": path})
	}
	for _, b := range fd.ready {
		path := "/f/" + b.Key
		out = append(out, map[string]any{"kind": "formula", "id": b.Key, "title": b.Name, "subtitle": b.Category, "line": b.About, "url": s.link(path), "path": path})
	}
	for _, rn := range fd.runs {
		path := fmt.Sprintf("/answers#%d", rn.ID)
		out = append(out, map[string]any{"kind": "answer", "id": rn.ID, "title": firstNonEmpty(rn.Title, rn.FormulaName), "subtitle": "Kept answer · " + rn.FormulaName, "line": rn.Main, "url": s.link(path), "path": path, "updated_at": rn.CreatedAt})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// found is what the words of a search find, each list in the order its page
// shows it.
type found struct {
	sum       *calc.Line       // the words worked out, when they are a sum
	notebooks []foundNotebook  // at most as many as asked for
	formulas  []store.Formula  // the person's own calculations
	ready     []builtinFormula // the ready ones
	runs      []store.Run      // kept answers
}

// foundNotebook is a notebook with the line the words were found in (its
// first line when they are in its name).
type foundNotebook struct {
	store.Notebook
	line string
}

// find looks the words up in the person's notebooks, calculations and kept
// answers and in the ready calculations, and works them out when they are a
// sum ("100 EUR in BAM"). /search and Pocket Search (capabilities.go) show
// the same finds.
func (s *Server) find(ctx context.Context, uid int64, q string, limit int) (*found, error) {
	f := &found{}
	if looksLikeSum(q) {
		if ln := calc.Calculate(q, s.calcOptions(ctx, uid, nil)); ln.Kind == "value" && ln.Text != "" {
			f.sum = &ln
		}
	}
	nbs, err := s.store.Notebooks(ctx, uid, store.NotebookFilter{Q: q, Limit: limit})
	if err != nil {
		return nil, err
	}
	lq := strings.ToLower(q)
	for _, nb := range nbs {
		line := ""
		if strings.Contains(strings.ToLower(nb.Title), lq) {
			line = nb.Preview
		} else if cells, err := s.store.Cells(ctx, uid, nb.ID); err == nil {
			for _, c := range cells {
				if l := firstNonEmpty(matchLine(c.Summary, q), matchLine(c.Body, q), matchLine(c.Title, q)); l != "" {
					line = l
					break
				}
			}
		}
		f.notebooks = append(f.notebooks, foundNotebook{Notebook: nb, line: line})
	}
	f.formulas, _ = s.store.Formulas(ctx, uid, q)
	for _, b := range library {
		if strings.Contains(strings.ToLower(b.Name+" "+b.About), lq) {
			f.ready = append(f.ready, b)
		}
	}
	if runs, err := s.store.Runs(ctx, uid, store.RunFilter{Limit: 500}); err == nil {
		for _, rn := range runs {
			if strings.Contains(strings.ToLower(rn.Title+" "+rn.Note+" "+rn.FormulaName), lq) {
				f.runs = append(f.runs, rn)
			}
		}
	}
	return f, nil
}

// looksLikeSum: digits with an operator, a conversion or a percentage.
func looksLikeSum(q string) bool {
	if !strings.ContainsAny(q, "0123456789") || len(q) > 200 {
		return false
	}
	l := " " + strings.ToLower(q) + " "
	return strings.ContainsAny(q, "+-*/^%×÷=√") || strings.Contains(l, " to ") || strings.Contains(l, " in ") || strings.Contains(l, " of ")
}

func matchLine(text, q string) string {
	lq := strings.ToLower(q)
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(strings.ToLower(l), lq) {
			return strings.TrimSpace(strings.TrimLeft(l, "#-* "))
		}
	}
	return ""
}
