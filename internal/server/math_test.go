package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newNotebook makes a notebook for auth with calculation cells.
func (a *app) newNotebook(auth, title string, bodies ...string) (string, []any) {
	a.t.Helper()
	cells := []map[string]any{}
	for _, b := range bodies {
		cells = append(cells, map[string]any{"kind": "calc", "body": b})
	}
	m := a.ok(auth, "POST", "/api/v1/notebooks", map[string]any{"title": title, "cells": cells})
	nb := m["notebook"].(map[string]any)
	return id(nb), m["cells"].([]any)
}

func results(c any) []any { return c.(map[string]any)["results"].([]any) }
func text(r any) string   { return str(r.(map[string]any), "text") }

func TestNotebookWorksOutAcrossCells(t *testing.T) {
	a := newApp(t, nil)
	nb, cells := a.newNotebook(a.alice, "Trip", "distance = 2 * 320 km\nuse = 6.5 L/100km", "fuel = distance * use\nfuel * 1.55 EUR/L")
	if got := text(results(cells[1])[0]); got != "41.6 L" {
		t.Fatalf("a variable from the cell above: got %q", got)
	}
	if got := text(results(cells[1])[1]); got != "64.48 EUR" {
		t.Fatalf("money: got %q", got)
	}
	// change the first cell: the second follows
	c0 := id(cells[0].(map[string]any))
	m := a.ok(a.alice, "PUT", "/api/v1/cells/"+c0, map[string]any{"body": "distance = 100 km\nuse = 5 L/100km"})
	res := m["results"].(map[string]any)
	second := res[id(cells[1].(map[string]any))].([]any)
	if got := text(second[0]); got != "5 L" {
		t.Fatalf("after the change: %q", got)
	}
	// summaries are kept for search and the journal
	full := a.ok(a.alice, "GET", "/api/v1/notebooks/"+nb, nil)
	sum := str(full["cells"].([]any)[1].(map[string]any), "summary")
	if !strings.Contains(sum, "fuel = distance * use = 5 L") {
		t.Fatalf("summary %q", sum)
	}
	// angles: degrees by default, radians when the notebook says so
	nb2, c2 := a.newNotebook(a.alice, "Angles", "sin(30)")
	if got := text(results(c2[0])[0]); got != "0.5" {
		t.Fatalf("degrees: %q", got)
	}
	m = a.ok(a.alice, "PUT", "/api/v1/notebooks/"+nb2, map[string]any{"angle": "rad"})
	if got := text(results(m["cells"].([]any)[0])[0]); !strings.HasPrefix(got, "-0.988") {
		t.Fatalf("radians: %q", got)
	}
	md := a.get(a.alice, "/api/v1/notebooks/"+nb+"/export.md")
	if !strings.Contains(md, "= 5 L") || !strings.Contains(md, "# Trip") {
		t.Fatalf("markdown:\n%s", md)
	}
}

func (a *app) get(auth, path string) string {
	a.t.Helper()
	req, _ := http.NewRequest("GET", a.url+path, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	b := new(strings.Builder)
	_, _ = fmt.Fprint(b, "")
	buf := make([]byte, 1<<20)
	for {
		k, err := res.Body.Read(buf)
		b.Write(buf[:k])
		if err != nil {
			break
		}
	}
	return b.String()
}

func TestEveryoneSeesOnlyTheirOwn(t *testing.T) {
	a := newApp(t, nil)
	nb, cells := a.newNotebook(a.alice, "Alice's secret sums", "salary = 4000 EUR")
	cell := id(cells[0].(map[string]any))
	f := a.ok(a.alice, "POST", "/api/v1/formulas", map[string]any{"name": "Alice's tiles", "code": "n = a * 2", "inputs": []map[string]any{{"name": "a", "default": "3"}}, "outputs": []map[string]any{{"label": "N", "expr": "n", "main": true}}})["formula"].(map[string]any)
	run := a.ok(a.alice, "POST", "/api/v1/library/paint/run", map[string]any{"values": map[string]string{"wall_area": "30"}, "keep": true, "title": "Alice's bedroom"})["run"].(map[string]any)
	pr := a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"learner": "Mila", "skill": "tables", "level": 2, "questions": 10, "correct": 9, "seconds": 60})["round"].(map[string]any)

	for _, p := range []string{"/api/v1/notebooks/" + nb, "/api/v1/notebooks/" + nb + "/export.md", "/api/v1/formulas/" + id(f), "/api/v1/practice/" + id(pr)} {
		a.expect(404, a.bob, "GET", p, nil)
	}
	a.expect(404, a.bob, "PUT", "/api/v1/cells/"+cell, map[string]any{"body": "salary = 0"})
	a.expect(404, a.bob, "DELETE", "/api/v1/cells/"+cell, nil)
	a.expect(404, a.bob, "PUT", "/api/v1/notebooks/"+nb, map[string]any{"title": "mine now"})
	a.expect(404, a.bob, "DELETE", "/api/v1/notebooks/"+nb, nil)
	a.expect(404, a.bob, "POST", "/api/v1/notebooks/"+nb+"/cells", map[string]any{"kind": "calc"})
	a.expect(404, a.bob, "PUT", "/api/v1/notebooks/"+nb+"/share", map[string]any{"shared": true})
	a.expect(404, a.bob, "POST", "/api/v1/formulas/"+id(f)+"/run", map[string]any{})
	a.expect(404, a.bob, "DELETE", "/api/v1/runs/"+id(run), nil)
	a.expect(404, a.bob, "DELETE", "/api/v1/practice/"+id(pr), nil)

	if l := a.ok(a.bob, "GET", "/api/v1/notebooks", nil)["notebooks"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d notebooks", len(l))
	}
	if l := a.ok(a.bob, "GET", "/api/v1/runs", nil)["runs"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d answers", len(l))
	}
	if l := a.ok(a.bob, "GET", "/api/v1/formulas", nil)["mine"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d formulas", len(l))
	}
	if l := a.ok(a.bob, "GET", "/api/v1/practice", nil)["rounds"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d rounds", len(l))
	}
	for _, q := range []string{"secret", "Alice", "bedroom"} {
		if l := a.ok(a.bob, "GET", "/api/v1/search?q="+q, nil)["results"].([]any); len(l) != 0 {
			t.Fatalf("bob finds %q: %v", q, l)
		}
	}
	day := time.Now().Format("2006-01-02")
	if l := a.ok(a.bob, "GET", "/api/v1/day/"+day, nil)["items"].([]any); len(l) != 0 {
		t.Fatalf("bob's day shows alice's: %v", l)
	}
	if l := a.ok(a.alice, "GET", "/api/v1/day/"+day, nil)["items"].([]any); len(l) < 3 {
		t.Fatalf("alice's day: %v", l)
	}
	// admins do not see other people's notebooks either
	a.expect(404, a.alice, "GET", "/api/v1/notebooks/"+a.mustBobNotebook(), nil)
}

func (a *app) mustBobNotebook() string {
	nb, _ := a.newNotebook(a.bob, "Bob's", "1 + 1")
	return nb
}

func TestShareLink(t *testing.T) {
	a := newApp(t, nil)
	nb, _ := a.newNotebook(a.alice, "Shared sums", "2 + 2")
	a.expect(404, "", "GET", "/api/v1/public/s/nothing", nil)
	m := a.ok(a.alice, "PUT", "/api/v1/notebooks/"+nb+"/share", map[string]any{"shared": true})
	url := str(m, "share_url")
	key := url[strings.LastIndex(url, "/")+1:]
	pub := a.expect(200, "", "GET", "/api/v1/public/s/"+key, nil)
	if text(results(pub["cells"].([]any)[0])[0]) != "4" || str(pub["notebook"].(map[string]any), "title") != "Shared sums" {
		t.Fatalf("shared: %v", pub)
	}
	if _, leak := pub["notebook"].(map[string]any)["share_key"]; leak {
		t.Fatal("the share link shows the key")
	}
	// a new link: the old one stops working
	m = a.ok(a.alice, "PUT", "/api/v1/notebooks/"+nb+"/share", map[string]any{"shared": true, "new": true})
	a.expect(404, "", "GET", "/api/v1/public/s/"+key, nil)
	url = str(m, "share_url")
	key = url[strings.LastIndex(url, "/")+1:]
	a.ok(a.alice, "PUT", "/api/v1/notebooks/"+nb+"/share", map[string]any{"shared": false})
	a.expect(404, "", "GET", "/api/v1/public/s/"+key, nil)
}

func TestReadyCalculations(t *testing.T) {
	a := newApp(t, nil)
	lib := a.ok(a.alice, "GET", "/api/v1/formulas", nil)
	if len(lib["library"].([]any)) < 30 {
		t.Fatalf("library: %d", len(lib["library"].([]any)))
	}
	// every built-in works out with its own examples
	for _, f := range lib["library"].([]any) {
		key := str(f.(map[string]any), "key")
		res := a.ok(a.alice, "POST", "/api/v1/library/"+key+"/run", map[string]any{})["result"].(map[string]any)
		if e := str(res, "error"); e != "" {
			t.Errorf("%s: %s", key, e)
			continue
		}
		for _, o := range res["outputs"].([]any) {
			if e := str(o.(map[string]any), "error"); e != "" {
				t.Errorf("%s / %s: %s", key, str(o.(map[string]any), "label"), e)
			}
		}
	}
	res := a.ok(a.alice, "POST", "/api/v1/library/paint/run", map[string]any{"values": map[string]string{"wall_area": "45", "coats": "2", "coverage": "10", "tin_size": "2.5"}})["result"].(map[string]any)
	if got := str(res["outputs"].([]any)[0].(map[string]any), "text"); got != "9 L" {
		t.Fatalf("paint: %q", got)
	}
	res = a.ok(a.alice, "POST", "/api/v1/library/trip_fuel/run", map[string]any{"values": map[string]string{"distance": "300", "ways": "One way", "use": "6", "price": "1.5 EUR/L"}})["result"].(map[string]any)
	if got := str(res["outputs"].([]any)[0].(map[string]any), "text"); got != "27.00 EUR" {
		t.Fatalf("trip: %q", got)
	}
	// keep an answer; it lists with the formula
	a.ok(a.alice, "POST", "/api/v1/library/bmi/run", map[string]any{"values": map[string]string{"weight": "80", "height": "180"}, "keep": true, "title": "November"})
	runs := a.ok(a.alice, "GET", "/api/v1/library/bmi", nil)["runs"].([]any)
	if len(runs) != 1 || str(runs[0].(map[string]any), "main") != "24.7 kg/m²" {
		t.Fatalf("kept: %v", runs)
	}
	// one's own copy, changed and run
	cp := a.ok(a.alice, "POST", "/api/v1/library/circle/copy", nil)["formula"].(map[string]any)
	a.ok(a.alice, "PUT", "/api/v1/formulas/"+id(cp), map[string]any{"name": "My circle", "pinned": true})
	res = a.ok(a.alice, "POST", "/api/v1/formulas/"+id(cp)+"/run", map[string]any{"values": map[string]string{"r": "1 m"}})["result"].(map[string]any)
	if got := str(res["outputs"].([]any)[0].(map[string]any), "text"); got != "3.142 m²" {
		t.Fatalf("circle: %q", got)
	}
	a.expect(400, a.alice, "POST", "/api/v1/formulas", map[string]any{"name": "bad", "inputs": []map[string]any{{"name": "1x"}}})
	// the exact calculator for other apps and AI
	c := a.ok(a.alice, "GET", "/api/v1/calculate?q=100%20EUR%20in%20BAM", nil)["result"].(map[string]any)
	if str(c, "text") != "195.58 BAM" {
		t.Fatalf("calculate: %v", c)
	}
	c = a.ok(a.alice, "POST", "/api/v1/calculate", map[string]any{"expr": "x = 4\nx^2 + 1"})["result"].(map[string]any)
	if str(c, "text") != "17" {
		t.Fatalf("calculate lines: %v", c)
	}
	c = a.ok(a.alice, "GET", "/api/v1/convert?value=5&from=km&to=mi", nil)["result"].(map[string]any)
	if !strings.HasPrefix(str(c, "text"), "3.106855") {
		t.Fatalf("convert: %v", c)
	}
}

func TestPracticeGoal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"daily_goal": 15, "learners": []string{"Mila", "Luka", "mila"}})
	st := a.ok(a.alice, "GET", "/api/v1/settings", nil)
	if l := st["learners"].([]any); len(l) != 2 {
		t.Fatalf("learners: %v", l)
	}
	if bob := a.ok(a.bob, "GET", "/api/v1/settings", nil); num(bob, "daily_goal") != 20 || len(bob["learners"].([]any)) != 0 {
		t.Fatalf("bob's settings changed: %v", bob)
	}
	first := a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"learner": "Mila", "skill": "tables", "questions": 10, "correct": 8})
	if first["goal_met"] == true {
		t.Fatal("goal met too early")
	}
	second := a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"learner": "Mila", "skill": "add", "questions": 10, "correct": 10})
	if second["goal_met"] != true {
		t.Fatalf("goal: %v", second)
	}
	a.expect(400, a.alice, "POST", "/api/v1/practice", map[string]any{"skill": "juggling", "questions": 10, "correct": 1})
	a.expect(400, a.alice, "POST", "/api/v1/practice", map[string]any{"skill": "add", "questions": 5, "correct": 6})
	p := a.ok(a.alice, "GET", "/api/v1/practice?learner=Mila", nil)
	for _, l := range p["learners"].([]any) {
		m := l.(map[string]any)
		if str(m, "learner") == "Mila" && (num(m, "today") != 20 || num(m, "streak") != 1) {
			t.Fatalf("Mila: %v", m)
		}
	}
}

func TestExportImportOnce(t *testing.T) {
	a := newApp(t, nil)
	a.newNotebook(a.alice, "Budget", "rent = 450 EUR\nfood = 300 EUR\ntotal")
	a.ok(a.alice, "POST", "/api/v1/library/vat/run", map[string]any{"values": map[string]string{"price": "100"}, "keep": true})
	a.ok(a.alice, "POST", "/api/v1/formulas", map[string]any{"name": "Mine", "code": "y = x * 2", "inputs": []map[string]any{{"name": "x", "default": "2"}}, "outputs": []map[string]any{{"label": "Y", "expr": "y"}}})
	a.ok(a.alice, "POST", "/api/v1/practice", map[string]any{"skill": "add", "questions": 10, "correct": 7})
	raw := a.get(a.alice, "/api/v1/export.json")
	var f map[string]any
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("POST", a.url+"/api/v1/import", strings.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+a.bob)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("import %d: %v %v", i, err, res.Status)
		}
		res.Body.Close()
	}
	nbs := a.ok(a.bob, "GET", "/api/v1/notebooks", nil)["notebooks"].([]any)
	if len(nbs) != 1 {
		t.Fatalf("bob has %d notebooks after importing twice", len(nbs))
	}
	full := a.ok(a.bob, "GET", "/api/v1/notebooks/"+id(nbs[0].(map[string]any)), nil)
	if got := text(results(full["cells"].([]any)[0])[2]); got != "750.00 EUR" {
		t.Fatalf("imported sums: %q", got)
	}
	if l := a.ok(a.bob, "GET", "/api/v1/runs", nil)["runs"].([]any); len(l) != 1 {
		t.Fatalf("answers: %d", len(l))
	}
	if l := a.ok(a.bob, "GET", "/api/v1/formulas", nil)["mine"].([]any); len(l) != 1 {
		t.Fatalf("formulas: %d", len(l))
	}
	if l := a.ok(a.bob, "GET", "/api/v1/practice", nil)["rounds"].([]any); len(l) != 1 {
		t.Fatalf("rounds: %d", len(l))
	}
}

func TestLinkScopes(t *testing.T) {
	can := func(scopes, method, path string) bool {
		r := httptest.NewRequest(method, path, nil)
		return linkAllows(strings.Fields(scopes), r)
	}
	cases := []struct {
		scopes, method, path string
		want                 bool
	}{
		{"math:read", "GET", "/api/v1/notebooks", true},
		{"math:read", "GET", "/api/v1/notebooks/3", true},
		{"math:read", "GET", "/api/v1/calculate", true},
		{"math:read", "POST", "/api/v1/calculate", true},
		{"math:read", "POST", "/api/v1/library/paint/run", true},
		{"math:read", "GET", "/api/v1/day/2026-11-11", true},
		{"math:read", "POST", "/api/v1/notebooks", false},
		{"math:read", "POST", "/api/v1/runs", false},
		{"math:write", "POST", "/api/v1/notebooks", true},
		{"math:write", "POST", "/api/v1/notebooks/3/cells", true},
		{"math:write", "PUT", "/api/v1/cells/9", true},
		{"math:write", "POST", "/api/v1/runs", true},
		{"math:write", "PUT", "/api/v1/notebooks/3/share", false},
		{"math:write", "GET", "/api/v1/notebooks", false},
		{"math:read math:write", "GET", "/api/v1/settings", false},
		{"math:read math:write", "GET", "/api/v1/export.json", false},
		{"math:read math:write", "POST", "/api/v1/practice", false},
		{"math:read math:write", "GET", "/api/v1/backup", false},
		{"", "GET", "/api/v1/auth/me", true},
	}
	for _, c := range cases {
		if got := can(c.scopes, c.method, c.path); got != c.want {
			t.Errorf("%q %s %s: %v, want %v", c.scopes, c.method, c.path, got, c.want)
		}
	}
}

func TestOtherAppsGreyWithoutCore(t *testing.T) {
	a := newApp(t, nil)
	apps := a.ok(a.alice, "GET", "/api/v1/apps", nil)["apps"].([]any)
	if len(apps) != len(pocketApps) {
		t.Fatalf("apps: %d", len(apps))
	}
	m := a.expect(400, a.alice, "GET", "/api/v1/apps/fill?source=rooms:wall", nil)
	if !strings.Contains(str(m, "error"), "Pocket Rooms") {
		t.Fatalf("plain reason: %v", m)
	}
	a.expect(400, a.alice, "GET", "/api/v1/apps/money/months", nil)
	a.expect(400, a.alice, "POST", "/api/v1/apps/slides", map[string]any{"title": "x", "labels": []string{"a"}, "values": []float64{1}})
}
