package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// What Pocket Search, Pocket Day and Pocket Hub ask Math (the capabilities
// value in links.go): one route each, in the shape every Pocket app answers
// in. They read what /search, /day and the start page read, as the request's
// person; only the capture adds something, a new notebook with one sum.

// dots joins the parts of a line that are there: "School · today".
func dots(parts ...string) string {
	out := parts[:0:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// counted is a number with its word: "1 notebook", "3 notebooks".
func counted(n int, one, many string) string {
	return fmt.Sprintf("%d %s", n, plural(n, one, many))
}

// answerLine is a kept answer under its name: the calculation it came from
// when the answer has a name of its own, then the answer.
func answerLine(rn store.Run) string {
	if rn.Title != "" && rn.FormulaName != "" {
		return rn.FormulaName + ": " + rn.Main
	}
	return rn.Main
}

// ---------- search ----------

// hasWords reports whether there is a letter or a digit to look for.
func hasWords(q string) bool {
	return strings.ContainsFunc(q, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
}

// pocketSearch answers Pocket Search with what /search finds, as one list:
// the words worked out when they are a sum, then what has them in its name,
// then what has them inside; each time the person's own things before the
// ready calculations.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	if !hasWords(q) {
		return nil, nil
	}
	ctx := r.Context()
	f, err := s.find(ctx, actorID(ctx), q, limit)
	if err != nil {
		return nil, err
	}
	lq := strings.ToLower(q)
	score := func(name string, own bool) float64 {
		n := 1.0
		if own {
			n++
		}
		if strings.Contains(strings.ToLower(name), lq) {
			n += 2
		}
		return n
	}
	out := []core.SearchItem{}
	if f.sum != nil {
		out = append(out, core.SearchItem{ID: "sum", Title: q + " = " + f.sum.Text, Kind: "sum", URL: "/?q=" + url.QueryEscape(q), Score: 5,
			Snippet: firstNonEmpty(strings.TrimPrefix(f.sum.Alt, "= "), "Worked out by Pocket Math")})
	}
	for _, n := range f.notebooks {
		out = append(out, core.SearchItem{ID: "notebook:" + itoa(n.ID), Title: n.Title, Kind: "notebook", Date: n.UpdatedAt, URL: "/n/" + itoa(n.ID),
			Snippet: firstNonEmpty(n.line, n.Subject), Score: score(n.Title, true)})
	}
	for _, c := range f.formulas {
		out = append(out, core.SearchItem{ID: "formula:" + itoa(c.ID), Title: c.Name, Kind: "formula", Date: c.UpdatedAt, URL: "/f/" + itoa(c.ID),
			Snippet: firstNonEmpty(c.About, "My calculation"), Score: score(c.Name, true)})
	}
	for _, rn := range f.runs {
		out = append(out, core.SearchItem{ID: "answer:" + itoa(rn.ID), Title: firstNonEmpty(rn.Title, rn.FormulaName), Kind: "answer", Date: rn.CreatedAt,
			URL: "/answers#" + itoa(rn.ID), Snippet: answerLine(rn), Score: score(firstNonEmpty(rn.Title, rn.FormulaName), true)})
	}
	for _, b := range f.ready {
		out = append(out, core.SearchItem{ID: "formula:" + b.Key, Title: b.Name, Kind: "formula", URL: "/f/" + b.Key,
			Snippet: firstNonEmpty(b.About, b.Category), Score: score(b.Name, false)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---------- day ----------

// pocketDay answers Pocket Day with the rows of /day: the notebooks started
// or worked on that day, the answers kept and the practice rounds done. The
// day runs from midnight to midnight in the caller's time zone, the box's
// when it names none; practice counts on the date it was saved under.
func (s *Server) pocketDay(r *http.Request, date string, tz *time.Location) (*core.Day, error) {
	ctx := r.Context()
	loc := time.Local
	if tz != nil {
		loc = tz
	}
	start, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, nil
	}
	// no zone letter: a stored "…T10:00:00.5Z" and "…T10:00:00Z" both sort after "…T10:00:00"
	const stored = "2006-01-02T15:04:05"
	nbs, runs, rounds, err := s.dayRows(ctx, actorID(ctx), date, start.UTC().Format(stored), start.AddDate(0, 0, 1).UTC().Format(stored))
	if err != nil {
		return nil, err
	}
	if len(nbs)+len(runs)+len(rounds) == 0 {
		return nil, nil
	}
	clock := func(ts string) string {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return ""
		}
		return t.In(loc).Format("15:04")
	}
	day := &core.Day{Date: date}
	var parts []string
	for _, d := range nbs {
		what := "Worked on"
		if d.started {
			what = "Started"
		}
		day.Items = append(day.Items, core.DayItem{ID: "notebook:" + itoa(d.nb.ID), Time: clock(d.at), Title: d.nb.Title, Sub: dots(what, d.nb.Subject), Kind: "notebook",
			URL: "/n/" + itoa(d.nb.ID)})
	}
	if n := len(nbs); n > 0 {
		parts = append(parts, counted(n, "notebook", "notebooks"))
	}
	for i := len(runs) - 1; i >= 0; i-- {
		rn := runs[i]
		day.Items = append(day.Items, core.DayItem{ID: "answer:" + itoa(rn.ID), Time: clock(rn.CreatedAt), Title: firstNonEmpty(rn.Title, rn.FormulaName), Sub: answerLine(rn),
			Kind: "answer", Note: rn.Note, URL: "/answers#" + itoa(rn.ID)})
	}
	if n := len(runs); n > 0 {
		parts = append(parts, counted(n, "answer kept", "answers kept"))
	}
	right, asked := 0, 0
	for i := len(rounds) - 1; i >= 0; i-- {
		p := rounds[i]
		it := core.DayItem{ID: "practice:" + itoa(p.ID), Time: clock(p.CreatedAt), Title: "Practised " + skillName(p.Skill), Kind: "practice", URL: "/practice",
			Sub: fmt.Sprintf("%d of %d right", p.Correct, p.Questions)}
		if p.Learner != "" {
			it.Title, it.URL = p.Learner+" practised "+skillName(p.Skill), "/practice?learner="+url.QueryEscape(p.Learner)
		}
		day.Items = append(day.Items, it)
		right, asked = right+p.Correct, asked+p.Questions
	}
	if n := len(rounds); n > 0 {
		parts = append(parts, counted(n, "practice round", "practice rounds"))
		day.Metrics = append(day.Metrics, core.Metric{Label: "Practice", Value: fmt.Sprintf("%d of %d right", right, asked)})
	}
	day.Summary = strings.Join(parts, ", ")
	return day, nil
}

// ---------- widget ----------

// widgetRows is how many notebooks a home screen gets (it shows up to eight rows).
const widgetRows = 8

// whenWords says when something was last changed: "today", "yesterday",
// "Mon 23 Nov", "23 Nov 2025".
func whenWords(ts string, now time.Time) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ""
	}
	t = t.In(now.Location())
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	switch today := day(now); {
	case day(t).Equal(today):
		return "today"
	case day(t).Equal(today.AddDate(0, 0, -1)):
		return "yesterday"
	case t.Year() == now.Year():
		return t.Format("Mon 2 Jan")
	}
	return t.Format("2 Jan 2006")
}

// practiceLine says how today's practice is going, for everybody who
// answered a question today: "12 of 20 practice questions today", or with
// children "Practice today: you 12 of 20, Mila 20 of 20". Nobody practised:
// nothing to say.
func practiceLine(done map[string]int, learners []string, goal int) string {
	of := func(n int) string {
		if goal > 0 {
			return fmt.Sprintf("%d of %d", n, goal)
		}
		return fmt.Sprint(n)
	}
	var parts []string
	for _, l := range learners {
		if n := done[l]; n > 0 {
			parts = append(parts, l+" "+of(n))
		}
	}
	me := done[""]
	switch {
	case me > 0 && len(parts) == 0:
		return of(me) + " practice " + plural(max(me, goal), "question", "questions") + " today"
	case me > 0:
		parts = append([]string{"you " + of(me)}, parts...)
	case len(parts) == 0:
		return ""
	}
	return "Practice today: " + strings.Join(parts, ", ")
}

// pocketWidget answers Pocket Hub with the start page's list of notebooks:
// pinned ones first, then the ones worked on last, each with its folder and
// when it was last changed. The line says how today's practice is going,
// when somebody practised. Nothing of either: nothing to show.
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	nbs, err := s.store.Notebooks(ctx, uid, store.NotebookFilter{Limit: widgetRows, Bare: true})
	if err != nil {
		return nil, err
	}
	done, err := s.store.QuestionsOn(ctx, uid, today())
	if err != nil {
		return nil, err
	}
	st := s.settingsOf(ctx, uid)
	w := &core.WidgetData{URL: "/notebooks", Line: practiceLine(done, st.Learners, st.DailyGoal)}
	now := time.Now()
	for _, nb := range nbs {
		w.Items = append(w.Items, core.WidgetItem{ID: "notebook:" + itoa(nb.ID), Title: nb.Title, Detail: dots(nb.Subject, whenWords(nb.UpdatedAt, now)), URL: "/n/" + itoa(nb.ID)})
	}
	if len(w.Items) == 0 {
		if w.Line == "" {
			return nil, nil
		}
		w.URL = "/practice"
	}
	return w, nil
}

// ---------- capture ----------

// quickSums is the name of a notebook made from one quick sum, as the start
// page's "Into a notebook" names it.
const quickSums = "Quick sums"

// captureSum splits a quick line into the sum to keep and the notebook's
// name: the text is the sum and a title names the notebook; a title alone is
// the sum.
func captureSum(in core.CaptureInput) (body, name string) {
	body, name = in.Text, in.Title
	if body == "" {
		body, name = in.Title, ""
	}
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	return body, firstNonEmpty(cleanText(strings.Join(strings.Fields(name), " "), maxName), quickSums)
}

// sumAnswer is a sum with its answer, for the person who typed it: the last
// line that has one ("15% of 240 = 36"; an equation reads "x + 2 = 5: x = 3").
// A line the calculator cannot work out, or no answer at all, is a reason
// the person can act on.
func sumAnswer(body string, res []calc.Line) (string, error) {
	lines := strings.Split(body, "\n")
	answer := ""
	for i, ln := range res {
		if i >= len(lines) {
			break
		}
		line := strings.TrimSpace(lines[i])
		switch {
		case ln.Kind == "error":
			if len(lines) > 1 {
				return "", fmt.Errorf("%w: Pocket Math could not work out line %d: %s", core.ErrCapture, i+1, ln.Error)
			}
			return "", fmt.Errorf("%w: Pocket Math could not work that out: %s", core.ErrCapture, ln.Error)
		case ln.Text == "" || ln.Kind != "value" && ln.Kind != "set":
		case ln.Kind == "set":
			answer = ln.Name + " = " + ln.Text
		case line == ln.Text:
			answer = line
		case strings.Contains(line, "="):
			answer = line + ": " + ln.Text
		default:
			answer = line + " = " + ln.Text
		}
	}
	if answer == "" {
		return "", fmt.Errorf("%w: write a sum to work out, like 15%% of 240 or 3.5 km in miles", core.ErrCapture)
	}
	return clip(answer, 200), nil
}

// captured is the answer for a kept sum: the sum with its answer as the
// notebook has it, and where the notebook opens. A request that was kept
// before with another sum is refused.
func (s *Server) captured(ctx context.Context, nb *store.Notebook, body string) (*core.CaptureResult, error) {
	cells, err := s.store.Cells(ctx, nb.UserID, nb.ID)
	if err != nil {
		return nil, err
	}
	// until the cell is there (the same request, at the same moment) the answer is worked out here
	lines, _ := calc.EvalSheet(strings.Split(body, "\n"), s.calcOptions(ctx, nb.UserID, nb))
	for _, c := range cells {
		if c.Kind != "calc" {
			continue
		}
		if c.Body != body {
			return nil, fmt.Errorf("%w: this was already kept with another sum", core.ErrCapture)
		}
		var kept []calc.Line
		if json.Unmarshal(c.Results, &kept) == nil && len(kept) > 0 {
			lines = kept
		}
		break
	}
	res := &core.CaptureResult{ID: "notebook:" + itoa(nb.ID), Title: nb.Title, URL: "/n/" + itoa(nb.ID)}
	if answer, err := sumAnswer(body, lines); err == nil {
		res.Title = answer
	}
	return res, nil
}

// pocketCapture keeps a quick line from Pocket Hub as a sum: a new notebook
// with the line in one calculation cell, worked out with the person's own
// settings — what "Into a notebook" on the start page does. Every sum gets a
// notebook of its own, so a name set in one ("km = 3") never changes the
// answer of another. Like POST /notebooks with a source_ref: the same
// request_id again answers with the notebook made the first time and adds
// nothing.
func (s *Server) pocketCapture(r *http.Request, in core.CaptureInput) (*core.CaptureResult, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	body, name := captureSum(in)
	src, ref := sourceOf(ctx), "capture:"+in.RequestID
	old, err := s.store.NotebookByRef(ctx, uid, src, ref)
	if err == nil {
		return s.captured(ctx, old, body) // an identical retry
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	st := s.settingsOf(ctx, uid)
	nb := &store.Notebook{UserID: uid, Title: name, Angle: st.Angle, Units: st.Units, Source: src, SourceRef: ref}
	// nothing is kept unless the calculator has an answer
	lines, _ := calc.EvalSheet(strings.Split(body, "\n"), s.calcOptions(ctx, uid, nb))
	if _, err := sumAnswer(body, lines); err != nil {
		return nil, err
	}
	if _, err := s.keepNotebook(ctx, uid, nb, []store.Cell{{Kind: "calc", Body: body}}); err != nil {
		if errors.Is(err, store.ErrConflict) { // the same request, at the same moment
			if old, err := s.store.NotebookByRef(ctx, uid, src, ref); err == nil {
				return s.captured(ctx, old, body)
			}
		}
		return nil, err
	}
	return s.captured(ctx, nb, body)
}
