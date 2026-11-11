package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

var skills = []string{"add", "subtract", "times", "divide", "tables", "fractions", "decimals", "percent", "equations", "powers", "rounding", "mixed"}

type learnerStats struct {
	Learner   string           `json:"learner"`
	Today     int              `json:"today"`
	Goal      int              `json:"goal"`
	Streak    int              `json:"streak"` // days in a row with practice
	Rounds    int              `json:"rounds"`
	Questions int              `json:"questions"`
	Correct   int              `json:"correct"`
	Days      []map[string]any `json:"days"`   // the last 30 days
	Skills    []map[string]any `json:"skills"` // accuracy per skill
	Last      *store.Practice  `json:"last"`
}

func (s *Server) learnerStats(ctx context.Context, uid int64, learner string, goal int) learnerStats {
	l := learner
	rounds, _ := s.store.Practices(ctx, uid, store.PracticeFilter{Learner: &l, From: store.AddDays(today(), -400), Limit: 5000})
	ls := learnerStats{Learner: learner, Goal: goal, Days: []map[string]any{}, Skills: []map[string]any{}}
	perDay := map[string][2]int{}
	perSkill := map[string][3]int{}
	for i, p := range rounds {
		if i == 0 {
			c := p
			ls.Last = &c
		}
		ls.Rounds++
		ls.Questions += p.Questions
		ls.Correct += p.Correct
		d := perDay[p.Day]
		perDay[p.Day] = [2]int{d[0] + p.Questions, d[1] + p.Correct}
		if p.Day >= store.AddDays(today(), -30) {
			k := perSkill[p.Skill]
			perSkill[p.Skill] = [3]int{k[0] + p.Questions, k[1] + p.Correct, k[2] + p.Seconds}
		}
	}
	ls.Today = perDay[today()][0]
	for i := 29; i >= 0; i-- {
		d := store.AddDays(today(), -i)
		ls.Days = append(ls.Days, map[string]any{"day": d, "questions": perDay[d][0], "correct": perDay[d][1]})
	}
	day := today()
	if perDay[day][0] == 0 {
		day = store.AddDays(day, -1) // today may still come
	}
	for perDay[day][0] > 0 {
		ls.Streak++
		day = store.AddDays(day, -1)
	}
	for k, v := range perSkill {
		pct := 0
		if v[0] > 0 {
			pct = v[1] * 100 / v[0]
		}
		sec := 0.0
		if v[0] > 0 {
			sec = float64(v[2]) / float64(v[0])
		}
		ls.Skills = append(ls.Skills, map[string]any{"skill": k, "questions": v[0], "correct": v[1], "pct": pct, "seconds_each": sec})
	}
	sort.Slice(ls.Skills, func(i, j int) bool { return ls.Skills[i]["questions"].(int) > ls.Skills[j]["questions"].(int) })
	return ls
}

func (s *Server) handlePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	learners := append([]string{""}, st.Learners...)
	q := r.URL.Query()
	var lp *string
	if q.Has("learner") {
		l := q.Get("learner")
		lp = &l
	}
	rounds, err := s.store.Practices(ctx, uid, store.PracticeFilter{Learner: lp, Limit: queryInt(r, "limit")})
	if err != nil {
		s.fail(w, err, "practice")
		return
	}
	// children who practised before but are no longer listed still show
	seen := map[string]bool{}
	for _, l := range learners {
		seen[l] = true
	}
	for _, p := range rounds {
		if !seen[p.Learner] {
			seen[p.Learner] = true
			learners = append(learners, p.Learner)
		}
	}
	stats := []learnerStats{}
	for _, l := range learners {
		stats = append(stats, s.learnerStats(ctx, uid, l, st.DailyGoal))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rounds": rounds, "learners": stats, "goal": st.DailyGoal, "round_size": st.RoundSize, "skills": skills})
}

func (s *Server) handleGetPractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.GetPractice(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "practice")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"round": p})
}

func (s *Server) handleSavePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Learner   string          `json:"learner"`
		Skill     string          `json:"skill"`
		Level     int             `json:"level"`
		Questions int             `json:"questions"`
		Correct   int             `json:"correct"`
		Seconds   int             `json:"seconds"`
		Details   json.RawMessage `json:"details"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !oneOf(in.Skill, skills) {
		writeErr(w, http.StatusBadRequest, "unknown skill")
		return
	}
	if in.Questions < 1 || in.Questions > 500 || in.Correct < 0 || in.Correct > in.Questions {
		writeErr(w, http.StatusBadRequest, "the counts do not add up")
		return
	}
	if len(in.Details) > 256<<10 || len(in.Details) > 0 && !json.Valid(in.Details) {
		in.Details = json.RawMessage("[]")
	}
	st := s.settingsOf(ctx, uid)
	before, _ := s.store.QuestionsOn(ctx, uid, today())
	p := &store.Practice{UserID: uid, Learner: cleanText(in.Learner, 40), Skill: in.Skill, Level: clampInt(in.Level, 1, 10), Questions: in.Questions, Correct: in.Correct,
		Seconds: clampInt(in.Seconds, 0, 24*3600), Details: in.Details, Day: today()}
	if err := s.store.CreatePractice(ctx, p); err != nil {
		s.fail(w, err, "save practice")
		return
	}
	s.publish(ctx, uid, "math.practised", fmt.Sprint(p.ID), map[string]any{"learner": p.Learner, "skill": p.Skill, "questions": p.Questions, "correct": p.Correct})
	out := map[string]any{"round": p}
	goal := st.DailyGoal
	if goal > 0 && before[p.Learner] < goal && before[p.Learner]+p.Questions >= goal {
		out["goal_met"] = true
		if p.Learner == "" && st.PracticeHabit > 0 {
			if ok, err := s.tickHabit(ctx, uid, st.PracticeHabit); err == nil && ok {
				out["habit"] = "Ticked in Pocket Habits"
			}
		}
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleDeletePractice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePractice(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete practice")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// practiceReminders tells people at their hour when the day's practice is
// not done (once a day, only if they asked for it).
func (s *Server) practiceReminders(ctx context.Context) {
	now := time.Now()
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || st.DailyGoal <= 0 || now.Hour() != st.NotifyHour || (!st.NotifyPractice && !st.AlertsGoal) {
			continue
		}
		key := fmt.Sprintf("remind:%d:%s", uid, today())
		if _, _, done := s.store.CacheGet(ctx, key); done {
			continue
		}
		_ = s.store.CachePut(ctx, key, "1")
		done, _ := s.store.QuestionsOn(ctx, uid, today())
		var left []string
		for _, l := range append([]string{""}, st.Learners...) {
			if n := done[l]; n < st.DailyGoal {
				who := firstNonEmpty(l, "You")
				left = append(left, fmt.Sprintf("%s: %d of %d", who, n, st.DailyGoal))
			}
		}
		if len(left) == 0 {
			continue
		}
		title := "Practice is not done yet today"
		body := strings.Join(left, "\n")
		if st.AlertsGoal {
			s.alert(ctx, uid, title, body, "/practice", "default")
		}
		if st.NotifyPractice && st.NotifyURL != "" {
			_ = notify.New(st.NotifyURL, st.NotifyFormat).Send(ctx, notify.Message{Title: title, Body: body, Priority: "default", Tags: []string{"abacus"}, Link: s.link("/practice")})
		}
	}
}
