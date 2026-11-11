package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// exportFile is everything one person made, as one JSON file.
type exportFile struct {
	App        string           `json:"app"`
	Version    int              `json:"version"`
	ExportedAt string           `json:"exported_at"`
	Notebooks  []exportNotebook `json:"notebooks"`
	Formulas   []store.Formula  `json:"formulas"`
	Runs       []store.Run      `json:"runs"`
	Practice   []store.Practice `json:"practice"`
}

type exportNotebook struct {
	store.Notebook
	CellList []store.Cell `json:"cell_list"`
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	f := exportFile{App: "pocket-math", Version: 1, ExportedAt: store.Now(), Notebooks: []exportNotebook{}}
	for _, archived := range []bool{false, true} {
		nbs, err := s.store.Notebooks(ctx, uid, store.NotebookFilter{Archived: archived})
		if err != nil {
			s.fail(w, err, "export")
			return
		}
		for _, nb := range nbs {
			cells, _ := s.store.Cells(ctx, uid, nb.ID)
			nb.ShareKey, nb.Shared = "", false
			f.Notebooks = append(f.Notebooks, exportNotebook{Notebook: nb, CellList: cells})
		}
	}
	f.Formulas, _ = s.store.Formulas(ctx, uid, "")
	f.Runs, _ = s.store.Runs(ctx, uid, store.RunFilter{Limit: 1000})
	f.Practice, _ = s.store.Practices(ctx, uid, store.PracticeFilter{Details: true, Limit: 5000})
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-math-%s.json"`, time.Now().Format("2006-01-02")))
	writeJSON(w, http.StatusOK, f)
}

// handleImport brings an export back. Importing the same file twice adds
// nothing twice: a notebook, calculation, answer or round already here (same
// name and the same time it was made) is skipped.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var f exportFile
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f.App != "pocket-math" {
		writeErr(w, http.StatusBadRequest, "this is not a Pocket Math export")
		return
	}
	added := map[string]int{}
	exists := func(q string, args ...any) bool {
		var n int
		_ = s.store.DB.QueryRowContext(ctx, q, args...).Scan(&n)
		return n > 0
	}
	for _, en := range f.Notebooks {
		nb := en.Notebook
		if exists(`SELECT COUNT(1) FROM notebooks WHERE user_id = ? AND title = ? AND created_at = ?`, uid, nb.Title, nb.CreatedAt) {
			continue
		}
		nb.ID, nb.UserID, nb.Shared, nb.ShareKey, nb.Source, nb.SourceRef = 0, uid, false, "", "", ""
		if nb.Angle != "rad" {
			nb.Angle = "deg"
		}
		if err := s.store.CreateNotebook(ctx, &nb); err != nil {
			s.fail(w, err, "import")
			return
		}
		for _, c := range en.CellList {
			if !oneOf(c.Kind, cellKinds) {
				continue
			}
			c.ID, c.UserID, c.NotebookID, c.Position, c.Source = 0, uid, nb.ID, -1, ""
			if err := s.store.CreateCell(ctx, &c); err != nil {
				s.fail(w, err, "import")
				return
			}
		}
		_, _ = s.recompute(ctx, uid, &nb)
		added["notebooks"]++
	}
	formulaIDs := map[int64]int64{}
	for _, fm := range f.Formulas {
		var id int64
		if err := s.store.DB.QueryRowContext(ctx, `SELECT id FROM formulas WHERE user_id = ? AND name = ? AND created_at = ?`, uid, fm.Name, fm.CreatedAt).Scan(&id); err == nil {
			formulaIDs[fm.ID] = id
			continue
		}
		old := fm.ID
		fm.ID, fm.UserID, fm.Source, fm.SourceRef = 0, uid, "", ""
		if err := s.store.CreateFormula(ctx, &fm); err != nil {
			s.fail(w, err, "import")
			return
		}
		formulaIDs[old] = fm.ID
		added["formulas"]++
	}
	for _, rn := range f.Runs {
		if exists(`SELECT COUNT(1) FROM runs WHERE user_id = ? AND formula_name = ? AND created_at = ?`, uid, rn.FormulaName, rn.CreatedAt) {
			continue
		}
		if rn.FormulaID != nil {
			if nid, ok := formulaIDs[*rn.FormulaID]; ok {
				rn.FormulaID = &nid
			} else {
				rn.FormulaID = nil
			}
		}
		rn.ID, rn.UserID, rn.Source = 0, uid, ""
		if err := s.store.CreateRun(ctx, &rn); err != nil {
			s.fail(w, err, "import")
			return
		}
		added["answers"]++
	}
	for _, p := range f.Practice {
		if !oneOf(p.Skill, skills) || exists(`SELECT COUNT(1) FROM practice WHERE user_id = ? AND skill = ? AND learner = ? AND created_at = ?`, uid, p.Skill, p.Learner, p.CreatedAt) {
			continue
		}
		p.ID, p.UserID = 0, uid
		if err := s.store.CreatePractice(ctx, &p); err != nil {
			s.fail(w, err, "import")
			return
		}
		added["practice"]++
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": added})
}

// handleExportCSV: the kept answers as a spreadsheet.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runs, err := s.store.Runs(ctx, actorID(ctx), store.RunFilter{Limit: 1000})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-math-answers-%s.csv"`, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Date", "Calculation", "For", "Answer", "Filled in", "All answers", "Note"})
	for _, rn := range runs {
		_ = cw.Write([]string{rn.CreatedAt[:10], rn.FormulaName, rn.Title, rn.Main, inputsText(rn.Inputs), outputsText(rn.Outputs), rn.Note})
	}
	cw.Flush()
}

func inputsText(raw json.RawMessage) string {
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	parts := []string{}
	for k, v := range m {
		parts = append(parts, k+" = "+v)
	}
	return strings.Join(sortedStrings(parts), "; ")
}

func outputsText(raw json.RawMessage) string {
	var outs []struct {
		Label string `json:"label"`
		Text  string `json:"text"`
	}
	_ = json.Unmarshal(raw, &outs)
	parts := []string{}
	for _, o := range outs {
		if o.Text != "" {
			parts = append(parts, o.Label+": "+o.Text)
		}
	}
	return strings.Join(parts, "; ")
}

var _ = context.Background
