package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-math/internal/config"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// fakeCapCore is a Pocket Core that vouches for link tokens
// pcx_<alice|bob>_<search|day|widget|capture|read|write> from Pocket Hub
// and remembers the manifest Math registers with.
type fakeCapCore struct {
	*httptest.Server
	mu       sync.Mutex
	manifest map[string]any
}

func newCapApp(t *testing.T) (*app, *fakeCapCore) {
	t.Helper()
	fc := &fakeCapCore{}
	scopes := map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture,
		"read": "math:read", "write": "math:write"}
	fc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps/register":
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			fc.mu.Lock()
			fc.manifest = m
			fc.mu.Unlock()
			io.WriteString(w, `{}`)
		case "/api/v1/auth/introspect":
			var in struct{ Token string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.Split(in.Token, "_")
			if len(parts) != 3 || parts[0] != "pcx" || scopes[parts[2]] == "" {
				io.WriteString(w, `{"active":false}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "kind": "delegated", "audience": "math", "scope": scopes[parts[2]],
				"user": map[string]any{"id": 1, "sub": parts[1] + "@example.com", "email": parts[1] + "@example.com", "name": parts[1], "role": "member"},
				"app":  map[string]any{"id": 9, "slug": "hub", "name": "Pocket Hub"}})
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(fc.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL, c.CoreRegistrationKey = fc.URL, "key" }), fc
}

// items are the rows of a capability answer.
func items(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	l, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("no items list in %v", m)
	}
	out := []map[string]any{}
	for _, v := range l {
		out = append(out, v.(map[string]any))
	}
	return out
}

// item finds the row with that title.
func item(t *testing.T, m map[string]any, title string) map[string]any {
	t.Helper()
	for _, it := range items(t, m) {
		if it["title"] == title {
			return it
		}
	}
	t.Fatalf("no %q in %v", title, m["items"])
	return nil
}

// seedMaths gives a person a notebook in a folder, a calculation of their
// own, a kept answer and a practice round of a child; it returns their ids.
func (a *app) seedMaths(auth string) map[string]string {
	a.t.Helper()
	nb, _ := a.newNotebook(auth, "Trip to the coast", "distance = 2 * 320 km\nuse = 6.5 L/100km", "fuel = distance * use")
	a.ok(auth, "PUT", "/api/v1/notebooks/"+nb, map[string]any{"subject": "Holidays"})
	f := a.ok(auth, "POST", "/api/v1/formulas", map[string]any{"name": "Tiles for the bath", "about": "How many boxes to buy", "code": "n = a * 2",
		"inputs": []map[string]any{{"name": "a", "default": "3"}}, "outputs": []map[string]any{{"label": "N", "expr": "n", "main": true}}})["formula"].(map[string]any)
	run := a.ok(auth, "POST", "/api/v1/library/paint/run", map[string]any{"values": map[string]string{"wall_area": "30"}, "keep": true, "title": "The bedroom walls"})["run"].(map[string]any)
	pr := a.ok(auth, "POST", "/api/v1/practice", map[string]any{"learner": "Mila", "skill": "tables", "level": 2, "questions": 10, "correct": 9, "seconds": 60})["round"].(map[string]any)
	return map[string]string{"notebook": nb, "formula": id(f), "run": id(run), "practice": id(pr), "main": str(run, "main"), "formula_name": str(run, "formula_name")}
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seedMaths(a.alice)
	// a notebook by its name: its first line beside it
	nb := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=coast", nil), "Trip to the coast")
	if nb["id"] != "notebook:"+ids["notebook"] || nb["kind"] != "notebook" || nb["url"] != "/n/"+ids["notebook"] || nb["snippet"] != "distance = 2 * 320 km" || nb["date"] == nil {
		t.Fatalf("notebook: %v", nb)
	}
	// by a sum inside: the line with its answer; a ready calculation of that name comes before it
	res := a.ok(a.alice, "GET", "/api/v1/pocket/search?q=fuel", nil)
	got := items(t, res)
	if item(t, res, "Trip to the coast")["snippet"] != "fuel = distance * use = 41.6 L" || got[0]["id"] != "formula:trip_fuel" || got[0]["title"] != "Fuel for a trip" {
		t.Fatalf("by a line: %v", got)
	}
	// the person's own calculation, then the ready ones
	got = items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=tiles", nil))
	if len(got) < 2 || got[0]["id"] != "formula:"+ids["formula"] || got[0]["kind"] != "formula" || got[0]["url"] != "/f/"+ids["formula"] || got[0]["snippet"] != "How many boxes to buy" {
		t.Fatalf("own calculation first: %v", got)
	}
	for _, it := range got[1:] {
		if it["kind"] != "formula" || !strings.HasPrefix(it["url"].(string), "/f/") || it["id"] != "formula:"+strings.TrimPrefix(it["url"].(string), "/f/") || it["snippet"] == nil {
			t.Fatalf("a ready calculation: %v", it)
		}
	}
	// a kept answer: what it was for, with the calculation and the answer
	ans := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=bedroom", nil), "The bedroom walls")
	if ans["id"] != "answer:"+ids["run"] || ans["kind"] != "answer" || ans["url"] != "/answers#"+ids["run"] || ans["snippet"] != ids["formula_name"]+": "+ids["main"] || ids["main"] == "" {
		t.Fatalf("kept answer: %v", ans)
	}
	// a sum is worked out, and comes first
	got = items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q="+strings.ReplaceAll("15%25 of 240", " ", "+"), nil))
	if len(got) == 0 || got[0]["title"] != "15% of 240 = 36" || got[0]["kind"] != "sum" || got[0]["id"] != "sum" || got[0]["url"] != "/?q=15%25+of+240" {
		t.Fatalf("a sum: %v", got)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=tiles&limit=1", nil)); len(got) != 1 || got[0]["id"] != "formula:"+ids["formula"] {
		t.Fatalf("limit 1: %v", got)
	}
	// no words: nothing, even where the app's own list would match everything
	for _, q := range []string{"", "%25", "_", "%3F%3F", "*"} {
		if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("q=%q: %v", q, got)
		}
	}
	// another person's notebooks, calculations and answers never appear: only
	// the ready calculations, which are everybody's
	for _, q := range []string{"coast", "fuel", "bedroom", "bath", "tiles", "paint"} {
		for _, it := range items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q="+q, nil)) {
			if _, ready := libraryByKey(strings.TrimPrefix(str(it, "id"), "formula:")); !ready || it["kind"] != "formula" {
				t.Fatalf("bob finds alice's %q: %v", q, it)
			}
		}
	}
	if got := items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q=coast", nil)); len(got) != 0 {
		t.Fatalf("bob finds something for coast: %v", got)
	}
	a.expect(401, "", "GET", "/api/v1/pocket/search?q=coast", nil)
	// the old route still answers as before
	old := a.ok(a.alice, "GET", "/api/v1/search?q=coast", nil)["results"].([]any)
	if r := old[0].(map[string]any); len(old) != 1 || r["kind"] != "notebook" || r["path"] != "/n/"+ids["notebook"] || r["url"] != "http://math.test/n/"+ids["notebook"] || r["subtitle"] != "Holidays" ||
		r["line"] != "distance = 2 * 320 km" {
		t.Fatalf("/search changed: %v", old)
	}
	if sum := a.ok(a.alice, "GET", "/api/v1/search?q=2%2B2", nil)["results"].([]any)[0].(map[string]any); sum["kind"] != "sum" || sum["title"] != "2+2 = 4" || sum["text"] != "4" || num(sum, "number") != 4 {
		t.Fatalf("/search sum changed: %v", sum)
	}
}

var clockRe = regexp.MustCompile(`^\d\d:\d\d$`)

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seedMaths(a.alice)
	a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"skill": "add", "level": 1, "questions": 10, "correct": 10, "seconds": 45})
	day := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+today(), nil)
	if day["date"] != today() || day["summary"] != "1 notebook, 1 answer kept, 2 practice rounds" || len(items(t, day)) != 4 {
		t.Fatalf("day: %v", day)
	}
	if ms := day["metrics"].([]any); len(ms) != 1 || ms[0].(map[string]any)["label"] != "Practice" || ms[0].(map[string]any)["value"] != "19 of 20 right" {
		t.Fatalf("metrics: %v", ms)
	}
	nb := item(t, day, "Trip to the coast")
	if nb["id"] != "notebook:"+ids["notebook"] || nb["kind"] != "notebook" || nb["sub"] != "Started · Holidays" || nb["url"] != "/n/"+ids["notebook"] || !clockRe.MatchString(str(nb, "time")) {
		t.Fatalf("notebook: %v", nb)
	}
	if it := item(t, day, "The bedroom walls"); it["id"] != "answer:"+ids["run"] || it["kind"] != "answer" || it["sub"] != ids["formula_name"]+": "+ids["main"] || it["url"] != "/answers#"+ids["run"] {
		t.Fatalf("kept answer: %v", it)
	}
	if it := item(t, day, "Mila practised times tables"); it["id"] != "practice:"+ids["practice"] || it["kind"] != "practice" || it["sub"] != "9 of 10 right" || it["url"] != "/practice?learner=Mila" {
		t.Fatalf("a child's practice: %v", it)
	}
	if it := item(t, day, "Practised adding"); it["sub"] != "10 of 10 right" || it["url"] != "/practice" {
		t.Fatalf("own practice: %v", it)
	}
	// an answer kept without a name goes by its calculation
	a.ok(a.alice, "POST", "/api/v1/runs", map[string]any{"key": "paint", "values_text": "wall_area = 12"})
	if it := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/day/"+today(), nil), ids["formula_name"]); it["sub"] == nil || strings.Contains(str(it, "sub"), ids["formula_name"]) {
		t.Fatalf("an answer without a name: %v", it)
	}
	// the caller's time zone decides where the day starts and what the clock says
	loc, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Skip("no time zone data")
	}
	there := time.Now().In(loc)
	d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+there.Format("2006-01-02")+"?tz=Pacific/Kiritimati", nil)
	if it := item(t, d, "Trip to the coast"); str(it, "time") != there.Format("15:04") && str(it, "time") != there.Add(-time.Minute).Format("15:04") {
		t.Fatalf("over there it is %s: %v", there.Format("15:04"), it)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+there.AddDate(0, 0, -2).Format("2006-01-02")+"?tz=Pacific/Kiritimati", nil); len(items(t, d)) != 0 {
		t.Fatalf("two days before, over there: %v", d)
	}
	// a day with nothing, and somebody else's day
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2019-03-04", nil); d["summary"] != nil || d["metrics"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("an empty day: %v", d)
	}
	if d := a.ok(a.bob, "GET", "/api/v1/pocket/day/"+today(), nil); d["summary"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("bob sees alice's day: %v", d)
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/yesterday", nil)
	// the old route still answers as before
	old := a.ok(a.alice, "GET", "/api/v1/day/"+today(), nil)["items"].([]any)
	texts := []string{}
	for _, it := range old {
		texts = append(texts, str(it.(map[string]any), "kind")+" "+str(it.(map[string]any), "text"))
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{"notebook Started “Trip to the coast”", "answer The bedroom walls — " + ids["formula_name"] + ": " + ids["main"], "practice Mila: Practised times tables — 9 of 10 right",
		"practice Practised adding — 10 of 10 right"} {
		if len(old) != 5 || !strings.Contains(all, want) {
			t.Fatalf("/day changed: no %q in\n%s", want, all)
		}
	}
	if first := old[0].(map[string]any); first["url"] != "http://math.test/n/"+ids["notebook"] || first["emoji"] != "🧮" {
		t.Fatalf("/day changed: %v", first)
	}
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	// nothing worked out yet: nothing to show
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); w["line"] != nil || w["metrics"] != nil || w["url"] != nil || len(items(t, w)) != 0 {
		t.Fatalf("without notebooks: %v", w)
	}
	first, _ := a.newNotebook(a.alice, "Budget", "rent = 600 EUR")
	a.ok(a.alice, "PUT", "/api/v1/notebooks/"+first, map[string]any{"subject": "Home", "pinned": true})
	second, _ := a.newNotebook(a.alice, "Homework", "3 * 4")
	hidden, _ := a.newNotebook(a.alice, "Old sums", "1 + 1")
	a.ok(a.alice, "PUT", "/api/v1/notebooks/"+hidden, map[string]any{"archived": true})
	// last changed a while ago, another year
	if _, err := a.st.DB.Exec(`UPDATE notebooks SET updated_at = '2021-03-04T10:00:00Z' WHERE id = ?`, n(first)); err != nil {
		t.Fatal(err)
	}
	w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	rows := items(t, w)
	if w["line"] != nil || w["url"] != "/notebooks" || len(rows) != 2 ||
		rows[0]["title"] != "Budget" || rows[0]["detail"] != "Home · 4 Mar 2021" || rows[0]["id"] != "notebook:"+first || rows[0]["url"] != "/n/"+first ||
		rows[1]["title"] != "Homework" || rows[1]["detail"] != "today" || rows[1]["url"] != "/n/"+second {
		t.Fatalf("pinned first, then the last one worked on: %v", w)
	}
	// at most eight rows
	for i := range 9 {
		a.newNotebook(a.alice, "More "+itoa(int64(i)), "1 + 1")
	}
	if rows := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)); len(rows) != 8 || rows[0]["title"] != "Budget" || rows[1]["title"] != "More 8" {
		t.Fatalf("eight rows: %v", rows)
	}
	// practice done today is worth a line
	a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"skill": "add", "level": 1, "questions": 12, "correct": 10, "seconds": 45})
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != "12 of 20 practice questions today" {
		t.Fatalf("own practice: %v", w["line"])
	}
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"learners": []string{"Mila", "Luka"}})
	a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"learner": "Mila", "skill": "tables", "level": 2, "questions": 20, "correct": 18, "seconds": 90})
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != "Practice today: you 12 of 20, Mila 20 of 20" {
		t.Fatalf("with a child: %v", w["line"])
	}
	// only practice, no notebook: the line opens the practice page
	a.ok(a.bob, "POST", "/api/v1/practice", map[string]any{"skill": "add", "level": 1, "questions": 1, "correct": 1, "seconds": 5})
	a.ok(a.bob, "PUT", "/api/v1/settings", map[string]any{"daily_goal": 0})
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); w["line"] != "1 practice question today" || w["url"] != "/practice" || len(items(t, w)) != 0 {
		t.Fatalf("bob's widget: %v", w)
	}
	a.expect(401, "", "GET", "/api/v1/pocket/widget", nil)
}

func TestWhenWords(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 30, 0, 0, time.FixedZone("x", 3600))
	for ts, want := range map[string]string{"2026-11-26T23:10:00Z": "today", "2026-11-26T22:59:59.5Z": "yesterday", "2026-11-25T12:00:00Z": "Wed 25 Nov", "2025-12-31T22:00:00Z": "31 Dec 2025",
		"2025-12-31T23:30:00Z": "Thu 1 Jan", "": ""} {
		if got := whenWords(ts, now); got != want {
			t.Errorf("%s → %q, want %q", ts, got, want)
		}
	}
}

// capture posts a quick line and returns the answer.
func capture(a *app, tok, requestID, text string, more map[string]any) (int, map[string]any) {
	body := map[string]any{"request_id": requestID, "kind": "calculation", "text": text}
	for k, v := range more {
		body[k] = v
	}
	code, m, _ := a.call(tok, "POST", "/api/v1/pocket/capture", body)
	return code, m
}

// notebooksOf is everything a person has, newest change first.
func (a *app) notebooksOf(tok string) []map[string]any {
	a.t.Helper()
	out := []map[string]any{}
	for _, nb := range a.ok(tok, "GET", "/api/v1/notebooks", nil)["notebooks"].([]any) {
		out = append(out, nb.(map[string]any))
	}
	return out
}

func TestPocketCapture(t *testing.T) {
	a := newApp(t, nil)
	// keep posts a line that must be kept and returns the answer with the notebook made
	keep := func(tok, requestID, text string, more map[string]any) (map[string]any, map[string]any) {
		t.Helper()
		code, m := capture(a, tok, requestID, text, more)
		if code != 201 || !strings.HasPrefix(str(m, "id"), "notebook:") || m["url"] != "/n/"+strings.TrimPrefix(str(m, "id"), "notebook:") {
			t.Fatalf("capture %q → %d %v", text, code, m)
		}
		return m, a.ok(tok, "GET", "/api/v1/notebooks/"+strings.TrimPrefix(str(m, "id"), "notebook:"), nil)
	}
	refuse := func(tok, requestID, text, want string) {
		t.Helper()
		before := len(a.notebooksOf(tok))
		if code, m := capture(a, tok, requestID, text, nil); code != 400 || m["error"] != want {
			t.Fatalf("capture %q → %d %v, want 400 %q", text, code, m, want)
		}
		if len(a.notebooksOf(tok)) != before {
			t.Fatalf("the refused line %q was kept", text)
		}
	}
	first, full := keep(a.alice, "req-0000000000000001", "15% of 240", nil)
	if first["title"] != "15% of 240 = 36" {
		t.Fatalf("the answer: %v", first)
	}
	nb, cells := full["notebook"].(map[string]any), full["cells"].([]any)
	if nb["title"] != "Quick sums" || nb["source"] != "" || nb["shared"] != false || len(cells) != 1 {
		t.Fatalf("the notebook: %v %v", nb, cells)
	}
	if c := cells[0].(map[string]any); c["kind"] != "calc" || c["body"] != "15% of 240" || text(results(c)[0]) != "36" || c["summary"] != "15% of 240 = 36" {
		t.Fatalf("the cell: %v", c)
	}
	// the same request again: the same answer, nothing added
	if code, again := capture(a, a.alice, "req-0000000000000001", "15% of 240", nil); code != 201 || again["id"] != first["id"] || again["title"] != first["title"] ||
		again["url"] != first["url"] || len(a.notebooksOf(a.alice)) != 1 {
		t.Fatalf("a retry: %d %v, notebooks %v", code, again, a.notebooksOf(a.alice))
	}
	// the same request with another sum is not a retry
	refuse(a.alice, "req-0000000000000001", "15% of 250", "this was already kept with another sum")

	for i, c := range []struct{ text, title string }{
		{"3.5 km in m", "3.5 km in m = 3,500 m"},
		{"  20% off 80  ", "20% off 80 = 64"},
		{"Paint: 3 * 4", "Paint: 3 * 4 = 12"},
		{"x = 5", "x = 5"},
		{"x + 2 = 5", "x + 2 = 5: x = 3"},
		{"42", "42"},
		{"a = 3\r\n# doubled\na * 2", "a * 2 = 6"},
	} {
		if m, _ := keep(a.alice, "req-10000000000000"+itoa(int64(10+i)), c.text, nil); m["title"] != c.title {
			t.Fatalf("%q → %q, want %q", c.text, m["title"], c.title)
		}
	}
	// a title names the notebook; alone it is the sum
	if m, full := keep(a.alice, "req-0000000000000002", "2 * 21", map[string]any{"title": "  The   answer "}); m["title"] != "2 * 21 = 42" || full["notebook"].(map[string]any)["title"] != "The answer" {
		t.Fatalf("with a name: %v %v", m, full["notebook"])
	}
	if code, m, _ := a.call(a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000003", "title": "7 * 6"}); code != 201 || m["title"] != "7 * 6 = 42" {
		t.Fatalf("a title alone: %d %v", code, m)
	}
	// degrees or radians, as the person set it
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"angle": "rad"})
	if m, full := keep(a.alice, "req-0000000000000004", "cos(0) + sin(0)", nil); m["title"] != "cos(0) + sin(0) = 1" || full["notebook"].(map[string]any)["angle"] != "rad" {
		t.Fatalf("own settings: %v %v", m, full["notebook"])
	}

	// what the calculator cannot work out is not kept
	const noSum = "write a sum to work out, like 15% of 240 or 3.5 km in miles"
	refuse(a.alice, "req-0000000000000005", "buy milk", noSum)
	refuse(a.alice, "req-0000000000000006", "# only a note", noSum)
	refuse(a.alice, "req-0000000000000007", "f(x) = x^2", noSum)
	refuse(a.alice, "req-0000000000000008", "5 +", "Pocket Math could not work that out: something is missing at the end")
	refuse(a.alice, "req-0000000000000009", "1/0", "Pocket Math could not work that out: can't divide by zero")
	refuse(a.alice, "req-000000000000000a", "a = 3\na * b", `Pocket Math could not work out line 2: I don't know "b" yet`)
	// a refused request id is free again
	if m, _ := keep(a.alice, "req-0000000000000005", "5 + 5", nil); m["title"] != "5 + 5 = 10" {
		t.Fatalf("after a refusal: %v", m)
	}

	// what the route checks itself
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "short", "text": "1 + 1"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-000000000000000b", "kind": "notebook", "text": "1 + 1"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-000000000000000b"})
	a.expect(401, "", "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-000000000000000b", "text": "1 + 1"})

	// per person: a request id is one's own, and bob's sum never reaches alice
	mine := len(a.notebooksOf(a.alice))
	if m, _ := keep(a.bob, "req-0000000000000001", "1 + 1", nil); m["title"] != "1 + 1 = 2" || m["id"] == first["id"] {
		t.Fatalf("bob's sum: %v", m)
	}
	if len(a.notebooksOf(a.bob)) != 1 || len(a.notebooksOf(a.alice)) != mine {
		t.Fatalf("bob has %d, alice %d (was %d)", len(a.notebooksOf(a.bob)), len(a.notebooksOf(a.alice)), mine)
	}
	a.expect(404, a.bob, "GET", "/api/v1/notebooks/"+strings.TrimPrefix(str(first, "id"), "notebook:"), nil)
	// a kept sum is found, shows on the day and on the home screen
	if got := items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q=quick", nil)); len(got) != 1 || got[0]["snippet"] != "1 + 1" {
		t.Fatalf("bob's search: %v", got)
	}
	if d := a.ok(a.bob, "GET", "/api/v1/pocket/day/"+today(), nil); d["summary"] != "1 notebook" || item(t, d, "Quick sums")["sub"] != "Started" {
		t.Fatalf("bob's day: %v", d)
	}
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 1 || item(t, w, "Quick sums")["detail"] != "today" {
		t.Fatalf("bob's widget: %v", w)
	}
}

// The same request sent several times at once keeps one notebook, and every
// sender gets the same answer.
func TestPocketCaptureAtOnce(t *testing.T) {
	a := newApp(t, nil)
	type answer struct {
		code int
		m    map[string]any
	}
	got := make(chan answer, 8)
	for range cap(got) {
		go func() {
			code, m := capture(a, a.alice, "req-0000000000000001", "12 * 12", nil)
			got <- answer{code, m}
		}()
	}
	first := <-got
	for range cap(got) - 1 {
		if next := <-got; next.code != 201 || first.code != 201 || next.m["id"] != first.m["id"] || next.m["title"] != "12 * 12 = 144" || first.m["title"] != "12 * 12 = 144" {
			t.Fatalf("answers differ: %v, %v", first, next)
		}
	}
	all := a.notebooksOf(a.alice)
	if len(all) != 1 || num(all[0], "cells") != 1 {
		t.Fatalf("notebooks: %v", all)
	}
}

// A kept sum tells the Core bus once, a retry not again.
func TestPocketCaptureEvent(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.CoreURL = "http://core.invalid"
	st := openTestStore(t, cfg)
	s := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()
	if err := s.core.Init(ctx); err != nil {
		t.Fatal(err)
	}
	u := &store.User{Email: "ana@example.com", Name: "Ana", Role: "admin"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := st.CreateAPIToken(ctx, u.ID, "t")
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)
	a := &app{t: t, url: ts.URL, s: s, st: st}
	for range 2 {
		if code, m := capture(a, tok, "req-0000000000000001", "2 + 2", nil); code != 201 {
			t.Fatalf("capture → %d %v", code, m)
		}
	}
	rows, err := st.DB.QueryContext(ctx, `SELECT body FROM pocket_outbox ORDER BY created_at, rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	made := 0
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		var ev struct {
			Topic   string
			Payload map[string]any
		}
		if err := json.Unmarshal(b, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Topic != "math.notebook_created" {
			continue
		}
		made++
		if ev.Payload["title"] != "Quick sums" || ev.Payload["user_email"] != "ana@example.com" {
			t.Errorf("payload = %v", ev.Payload)
		}
	}
	if made != 1 {
		t.Fatalf("notebook_created events = %d, want 1", made)
	}
}

// A link token with one pocket:* scope reaches its one route and nothing else.
func TestPocketLinkTokens(t *testing.T) {
	a, _ := newCapApp(t)
	ids := a.seedMaths(a.alice)
	day := today()
	sum := func(requestID string) map[string]any {
		return map[string]any{"request_id": requestID, "kind": "calculation", "text": "6 * 7"}
	}
	for _, c := range []struct {
		tok, method, path string
		body              any
		want              int
	}{
		{"pcx_alice_search", "GET", "/api/v1/pocket/search?q=coast", nil, 200},
		{"pcx_alice_day", "GET", "/api/v1/pocket/day/" + day, nil, 200},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil, 200},
		{"pcx_alice_capture", "POST", "/api/v1/pocket/capture", sum("req-0000000000000001"), 201},
		{"pcx_alice_widget", "GET", "/api/v1/auth/me", nil, 200},
		// another capability's route
		{"pcx_alice_search", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_search", "POST", "/api/v1/pocket/capture", sum("req-0000000000000002"), 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/search?q=coast", nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/search?q=coast", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_widget", "POST", "/api/v1/pocket/capture", sum("req-0000000000000003"), 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/search?q=coast", nil, 403},
		// the rest of the app
		{"pcx_alice_widget", "GET", "/api/v1/home", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/summary", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/notebooks", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/search?q=coast", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/notebooks/" + ids["notebook"], nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/calculate?q=1%2B1", nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/day/" + day, nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/runs", nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/notebooks", map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "POST", "/api/v1/notebooks/" + ids["notebook"] + "/cells", map[string]any{"body": "1"}, 403},
		{"pcx_alice_capture", "PUT", "/api/v1/notebooks/" + ids["notebook"], map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "DELETE", "/api/v1/notebooks/" + ids["notebook"], nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/runs", map[string]any{"key": "paint"}, 403},
		{"pcx_alice_capture", "GET", "/api/v1/settings", nil, 403},
		// the app's own scopes do not open the new routes
		{"pcx_alice_read", "GET", "/api/v1/pocket/search?q=coast", nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_write", "POST", "/api/v1/pocket/capture", sum("req-0000000000000004"), 403},
		{"pcx_alice_read", "GET", "/api/v1/search?q=coast", nil, 200},
		{"pcx_alice_read", "GET", "/api/v1/day/" + day, nil, 200},
		{"pcx_nobody", "GET", "/api/v1/pocket/widget", nil, 401},
	} {
		if code, m, _ := a.call(c.tok, c.method, c.path, c.body); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%v)", c.tok, c.method, c.path, code, c.want, m)
		}
	}
	// the token acts for its person only
	if got := items(t, a.ok("pcx_alice_search", "GET", "/api/v1/pocket/search?q=coast", nil)); len(got) != 1 || got[0]["title"] != "Trip to the coast" {
		t.Fatalf("alice through the link: %v", got)
	}
	if got := items(t, a.ok("pcx_bob_search", "GET", "/api/v1/pocket/search?q=coast", nil)); len(got) != 0 {
		t.Fatalf("bob through the link finds alice's notebook: %v", got)
	}
	if w := a.ok("pcx_bob_widget", "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 {
		t.Fatalf("bob's widget through the link: %v", w)
	}
	if d := a.ok("pcx_bob_day", "GET", "/api/v1/pocket/day/"+day, nil); len(items(t, d)) != 0 {
		t.Fatalf("bob's day through the link: %v", d)
	}
	// only the one allowed capture landed, made by the app that sent it; a retry adds nothing
	code, m := capture(a, "pcx_alice_capture", "req-0000000000000001", "6 * 7", nil)
	if code != 201 || m["title"] != "6 * 7 = 42" {
		t.Fatalf("retry through the link: %d %v", code, m)
	}
	all := a.notebooksOf(a.alice)
	if len(all) != 2 || all[0]["title"] != "Quick sums" || all[0]["source"] != "hub" || "notebook:"+id(all[0]) != m["id"] {
		t.Fatalf("alice's notebooks: %v", all)
	}
	if c := a.ok(a.alice, "GET", "/api/v1/notebooks/"+id(all[0]), nil)["cells"].([]any)[0].(map[string]any); c["source"] != "hub" || c["body"] != "6 * 7" {
		t.Fatalf("the cell: %v", c)
	}
	// the same request id sent by the person herself is another request
	if code, own := capture(a, a.alice, "req-0000000000000001", "6 * 7", nil); code != 201 || own["id"] == m["id"] {
		t.Fatalf("own request: %d %v", code, own)
	}
	if len(a.notebooksOf(a.bob)) != 0 {
		t.Fatal("bob got a notebook")
	}
}

func TestPocketLinkScopes(t *testing.T) {
	req := func(m, p string) *http.Request { return httptest.NewRequest(m, p, nil) }
	for _, c := range []struct {
		scope string
		r     *http.Request
		want  bool
	}{
		{core.ScopeSearch, req("GET", "/api/v1/pocket/search?q=x"), true},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/2026-01-02"), true},
		{core.ScopeWidget, req("GET", "/api/v1/pocket/widget"), true},
		{core.ScopeCapture, req("POST", "/api/v1/pocket/capture"), true},
		{core.ScopeWidget, req("GET", "/api/v1/auth/me"), true},
		{core.ScopeSearch, req("POST", "/api/v1/pocket/search"), false},
		{core.ScopeSearch, req("GET", "/api/v1/search"), false},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/today"), false},
		{core.ScopeDay, req("GET", "/api/v1/day/2026-01-02"), false},
		{core.ScopeWidget, req("GET", "/api/v1/home"), false},
		{core.ScopeCapture, req("GET", "/api/v1/pocket/capture"), false},
		{core.ScopeCapture, req("POST", "/api/v1/notebooks"), false},
		{"math:read", req("GET", "/api/v1/pocket/widget"), false},
		{"math:read", req("GET", "/api/v1/pocket/search"), false},
		{"math:write", req("POST", "/api/v1/pocket/capture"), false},
	} {
		if got := linkAllows([]string{c.scope}, c.r); got != c.want {
			t.Errorf("%s %s %s = %v, want %v", c.scope, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

// Math tells Core what it can do, so Search, Day and Hub find it on their own.
func TestManifestCapabilities(t *testing.T) {
	a, fc := newCapApp(t)
	if err := a.s.registerWithCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	fc.mu.Lock()
	m := fc.manifest
	fc.mu.Unlock()
	if m["slug"] != "math" || m["category"] != "Tools" {
		t.Fatalf("manifest: %v %v", m["slug"], m["category"])
	}
	search, _ := m["search"].(map[string]any)
	day, _ := m["day"].(map[string]any)
	widget, _ := m["widget"].(map[string]any)
	cap, _ := m["capture"].(map[string]any)
	if search == nil || len(search["kinds"].([]any)) != 4 || day == nil || day["label"] != "Math" || day["one"] != "sum" || day["many"] != "sums" || widget == nil || widget["title"] != "Math" || cap == nil {
		t.Fatalf("capabilities: %v %v %v %v", search, day, widget, cap)
	}
	bangs := []string{}
	for _, b := range search["bangs"].([]any) {
		bangs = append(bangs, b.(string))
	}
	if strings.Join(bangs, " ") != "math calc sum" {
		t.Fatalf("bangs: %v", bangs)
	}
	if k := cap["kinds"].([]any); len(k) != 1 || k[0].(map[string]any)["kind"] != "calculation" || k[0].(map[string]any)["label"] != "Calculation" || k[0].(map[string]any)["fields"] != nil {
		t.Fatalf("capture kinds: %v", cap)
	}
	offered := map[string]bool{}
	for _, o := range m["offers"].([]any) {
		offered[o.(map[string]any)["scope"].(string)] = true
	}
	for _, sc := range []string{"math:read", "math:write", core.ScopeMoveExport, core.ScopeMoveImport, core.ScopeSearch, core.ScopeDay, core.ScopeWidget, core.ScopeCapture} {
		if !offered[sc] {
			t.Errorf("the manifest does not offer %s: %v", sc, m["offers"])
		}
	}
}
