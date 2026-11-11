package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

var reVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,40}$`)

func (s *Server) handleFormulas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	mine, err := s.store.Formulas(ctx, uid, q)
	if err != nil {
		s.fail(w, err, "formulas")
		return
	}
	lib := []store.Formula{}
	lq := strings.ToLower(q)
	counts := map[string]int{}
	if runs, err := s.store.Runs(ctx, uid, store.RunFilter{Limit: 1000}); err == nil {
		for _, r := range runs {
			if r.FormulaKey != "" {
				counts[r.FormulaKey]++
			}
		}
	}
	for i := range library {
		b := &library[i]
		if lq != "" && !strings.Contains(strings.ToLower(b.Name+" "+b.About+" "+b.Category), lq) {
			continue
		}
		f := b.asFormula()
		f.Runs = counts[b.Key]
		lib = append(lib, f)
	}
	writeJSON(w, http.StatusOK, map[string]any{"mine": mine, "library": lib, "categories": libraryCategories})
}

type formulaIn struct {
	Name      *string        `json:"name"`
	About     *string        `json:"about"`
	Category  *string        `json:"category"`
	Icon      *string        `json:"icon"`
	Code      *string        `json:"code"`
	Inputs    *[]calc.Input  `json:"inputs"`
	Outputs   *[]calc.Output `json:"outputs"`
	Pinned    *bool          `json:"pinned"`
	SourceRef string         `json:"source_ref"`
}

func (in formulaIn) apply(f *store.Formula) error {
	if in.Name != nil {
		f.Name = cleanText(*in.Name, maxName)
	}
	if in.About != nil {
		f.About = cleanText(*in.About, 2000)
	}
	if in.Category != nil {
		f.Category = cleanText(*in.Category, 60)
	}
	if in.Icon != nil {
		f.Icon = cleanText(*in.Icon, 30)
	}
	if in.Code != nil {
		if len(*in.Code) > 20000 {
			return &userErr{"the working is too long"}
		}
		f.Code = strings.TrimRight(*in.Code, "\n ")
	}
	if in.Inputs != nil {
		if len(*in.Inputs) > 30 {
			return &userErr{"at most 30 boxes to fill in"}
		}
		seen := map[string]bool{}
		ins := []calc.Input{}
		for _, i := range *in.Inputs {
			i.Name = strings.TrimSpace(i.Name)
			if !reVarName.MatchString(i.Name) {
				return &userErr{fmt.Sprintf("“%s” can't be a name: use letters, digits and _", i.Name)}
			}
			if calc.Reserved(i.Name) {
				return &userErr{fmt.Sprintf("“%s” is a word the calculator uses; pick another name", i.Name)}
			}
			if seen[i.Name] {
				return &userErr{"two boxes are called " + i.Name}
			}
			seen[i.Name] = true
			i.Label, i.Unit, i.Default, i.Help = cleanText(i.Label, 80), cleanText(i.Unit, 40), cleanText(i.Default, 200), cleanText(i.Help, 300)
			if !oneOf(i.Kind, []string{"", "number", "percent", "date", "choice"}) {
				i.Kind = ""
			}
			i.Options, i.From = cleanText(i.Options, 1000), cleanText(i.From, 40)
			ins = append(ins, i)
		}
		b, _ := json.Marshal(ins)
		f.Inputs = b
	}
	if in.Outputs != nil {
		if len(*in.Outputs) > 20 {
			return &userErr{"at most 20 answers"}
		}
		outs := []calc.Output{}
		for _, o := range *in.Outputs {
			o.Label, o.Expr, o.Unit = cleanText(o.Label, 80), cleanText(o.Expr, 500), cleanText(o.Unit, 40)
			if o.Expr == "" {
				continue
			}
			if o.Decimals != nil && (*o.Decimals < 0 || *o.Decimals > 12) {
				o.Decimals = nil
			}
			outs = append(outs, o)
		}
		b, _ := json.Marshal(outs)
		f.Outputs = b
	}
	if in.Pinned != nil {
		f.Pinned = *in.Pinned
	}
	return nil
}

func (s *Server) handleCreateFormula(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in formulaIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	src, ref := sourceOf(ctx), cleanText(in.SourceRef, 200)
	if ref != "" {
		if old, err := s.store.FormulaByRef(ctx, uid, src, ref); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"formula": old})
			return
		}
	}
	f := &store.Formula{UserID: uid, Source: src, SourceRef: ref, Category: "Mine"}
	if err := in.apply(f); err != nil {
		s.userFail(w, err, "formula")
		return
	}
	if f.Name == "" {
		f.Name = "My calculation"
	}
	if err := s.store.CreateFormula(ctx, f); err != nil {
		s.fail(w, err, "save formula")
		return
	}
	s.publish(ctx, uid, "math.formula_saved", fmt.Sprint(f.ID), map[string]any{"formula_id": f.ID, "name": f.Name})
	writeJSON(w, http.StatusCreated, map[string]any{"formula": f})
}

func (s *Server) formulaOf(w http.ResponseWriter, r *http.Request) (*store.Formula, bool) {
	ctx := r.Context()
	f, err := s.store.GetFormula(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "formula")
		return nil, false
	}
	return f, true
}

func (s *Server) handleGetFormula(w http.ResponseWriter, r *http.Request) {
	f, ok := s.formulaOf(w, r)
	if !ok {
		return
	}
	runs, _ := s.store.Runs(r.Context(), f.UserID, store.RunFilter{FormulaID: f.ID, Limit: 50})
	writeJSON(w, http.StatusOK, map[string]any{"formula": f, "runs": runs})
}

func (s *Server) handleLibraryFormula(w http.ResponseWriter, r *http.Request) {
	b, ok := libraryByKey(chiParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	runs, _ := s.store.Runs(r.Context(), actorID(r.Context()), store.RunFilter{FormulaKey: b.Key, Limit: 50})
	writeJSON(w, http.StatusOK, map[string]any{"formula": b.asFormula(), "runs": runs})
}

func (s *Server) handleUpdateFormula(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.formulaOf(w, r)
	if !ok {
		return
	}
	if !mayChange(ctx, f.Source) {
		writeErr(w, http.StatusForbidden, "this calculation was not made by that app")
		return
	}
	var in formulaIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := in.apply(f); err != nil {
		s.userFail(w, err, "formula")
		return
	}
	if f.Name == "" {
		f.Name = "My calculation"
	}
	if err := s.store.UpdateFormula(ctx, f); err != nil {
		s.fail(w, err, "save formula")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"formula": f})
}

func (s *Server) handleDeleteFormula(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.formulaOf(w, r)
	if !ok {
		return
	}
	if !mayChange(ctx, f.Source) {
		writeErr(w, http.StatusForbidden, "this calculation was not made by that app")
		return
	}
	if err := s.store.DeleteFormula(ctx, f.UserID, f.ID); err != nil {
		s.fail(w, err, "delete formula")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCopyLibrary makes one's own editable copy of a built-in.
func (s *Server) handleCopyLibrary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, ok := libraryByKey(chiParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	f := b.asFormula()
	f.UserID, f.FromKey, f.Builtin, f.Key, f.Source = actorID(ctx), b.Key, false, "", sourceOf(ctx)
	if err := s.store.CreateFormula(ctx, &f); err != nil {
		s.fail(w, err, "copy")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"formula": f})
}

type runIn struct {
	Values map[string]string `json:"values"`
	Keep   bool              `json:"keep"`
	Title  string            `json:"title"`
	Note   string            `json:"note"`
	// for trying a formula that is not saved yet
	Code    string        `json:"code"`
	Inputs  []calc.Input  `json:"inputs"`
	Outputs []calc.Output `json:"outputs"`
}

// runFormula works one out with the person's settings (degrees always for
// built-ins: "angle at a" reads 36.87°).
func (s *Server) runFormula(ctx context.Context, uid int64, code string, inputs []calc.Input, outputs []calc.Output, values map[string]string) calc.RunResult {
	o := s.calcOptions(ctx, uid, nil)
	o.Units, o.Deg = true, true
	if values == nil {
		values = map[string]string{}
	}
	return calc.Run(code, inputs, values, outputs, o)
}

func mainAnswer(res calc.RunResult) string {
	for _, o := range res.Outputs {
		if o.Main && o.Error == "" {
			return o.Text
		}
	}
	for _, o := range res.Outputs {
		if o.Error == "" {
			return o.Text
		}
	}
	return ""
}

func (s *Server) finishRun(w http.ResponseWriter, r *http.Request, in runIn, run *store.Run, res calc.RunResult) {
	ctx := r.Context()
	out := map[string]any{"result": res}
	// another app keeps answers through POST /runs (math:write), not here
	if in.Keep && res.Error == "" && linkApp(ctx) == "" {
		run.UserID, run.Title, run.Note, run.Source = actorID(ctx), cleanText(in.Title, maxName), cleanText(in.Note, 2000), sourceOf(ctx)
		run.Inputs = mustJSON(in.Values)
		run.Outputs = mustJSON(res.Outputs)
		run.Main = mainAnswer(res)
		if err := s.store.CreateRun(ctx, run); err != nil {
			s.fail(w, err, "keep answer")
			return
		}
		s.publish(ctx, run.UserID, "math.answer_kept", fmt.Sprint(run.ID), map[string]any{"run_id": run.ID, "formula": run.FormulaName, "title": run.Title, "answer": run.Main})
		out["run"] = run
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRunFormula(w http.ResponseWriter, r *http.Request) {
	f, ok := s.formulaOf(w, r)
	if !ok {
		return
	}
	var in runIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var ins []calc.Input
	var outs []calc.Output
	_ = json.Unmarshal(f.Inputs, &ins)
	_ = json.Unmarshal(f.Outputs, &outs)
	res := s.runFormula(r.Context(), f.UserID, f.Code, ins, outs, in.Values)
	id := f.ID
	s.finishRun(w, r, in, &store.Run{FormulaID: &id, FormulaName: f.Name}, res)
}

func (s *Server) handleRunLibrary(w http.ResponseWriter, r *http.Request) {
	b, ok := libraryByKey(chiParam(r, "key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var in runIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	res := s.runFormula(r.Context(), actorID(r.Context()), b.Code, b.Inputs, b.Outputs, in.Values)
	s.finishRun(w, r, in, &store.Run{FormulaKey: b.Key, FormulaName: b.Name}, res)
}

// handleTryFormula works out a formula being written (nothing is kept).
func (s *Server) handleTryFormula(w http.ResponseWriter, r *http.Request) {
	var in runIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	res := s.runFormula(r.Context(), actorID(r.Context()), in.Code, in.Inputs, in.Outputs, in.Values)
	writeJSON(w, http.StatusOK, map[string]any{"result": res})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := store.RunFilter{FormulaID: int64(queryInt(r, "formula_id")), FormulaKey: r.URL.Query().Get("key"), Limit: queryInt(r, "limit")}
	runs, err := s.store.Runs(ctx, actorID(ctx), f)
	if err != nil {
		s.fail(w, err, "answers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleUpdateRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	run, err := s.store.GetRun(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "answer")
		return
	}
	var in struct {
		Title *string `json:"title"`
		Note  *string `json:"note"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Title != nil {
		run.Title = cleanText(*in.Title, maxName)
	}
	if in.Note != nil {
		run.Note = cleanText(*in.Note, 2000)
	}
	if err := s.store.UpdateRun(ctx, run); err != nil {
		s.fail(w, err, "save answer")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run})
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteRun(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete answer")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleKeepRun works a calculation out and keeps the answer: for other
// apps and Pocket AI ({"key": "paint"} or {"formula_id": 3}, values, title).
func (s *Server) handleKeepRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		runIn
		Key       string `json:"key"`
		FormulaID int64  `json:"formula_id"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	run := &store.Run{UserID: uid, Title: cleanText(in.Title, maxName), Note: cleanText(in.Note, 2000), Source: sourceOf(ctx), Inputs: mustJSON(in.Values)}
	var res calc.RunResult
	if b, ok := libraryByKey(in.Key); ok {
		res = s.runFormula(ctx, uid, b.Code, b.Inputs, b.Outputs, in.Values)
		run.FormulaKey, run.FormulaName = b.Key, b.Name
	} else if f, err := s.store.GetFormula(ctx, uid, in.FormulaID); err == nil {
		var ins []calc.Input
		var outs []calc.Output
		_ = json.Unmarshal(f.Inputs, &ins)
		_ = json.Unmarshal(f.Outputs, &outs)
		res = s.runFormula(ctx, uid, f.Code, ins, outs, in.Values)
		id := f.ID
		run.FormulaID, run.FormulaName = &id, f.Name
	} else {
		writeErr(w, http.StatusBadRequest, "say which calculation: key (a built-in) or formula_id")
		return
	}
	if res.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": res.Error, "result": res})
		return
	}
	run.Outputs, run.Main = mustJSON(res.Outputs), mainAnswer(res)
	if err := s.store.CreateRun(ctx, run); err != nil {
		s.fail(w, err, "keep answer")
		return
	}
	s.publish(ctx, uid, "math.answer_kept", fmt.Sprint(run.ID), map[string]any{"run_id": run.ID, "formula": run.FormulaName, "title": run.Title, "answer": run.Main})
	writeJSON(w, http.StatusCreated, map[string]any{"run": run, "result": res})
}
