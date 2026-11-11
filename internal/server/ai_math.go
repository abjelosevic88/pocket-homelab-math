package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// ---- AI help: explain, write sums from words, read a photo, draft a
// saved calculation, check a notebook. The numbers themselves always come
// from the calculator here, never from the AI. ----

const syntaxGuide = `Pocket Math calculator syntax (one calculation per line):
- numbers and + - * / ^ ( ); multiplication can be written as 2x or 3(4+5)
- units and currencies after numbers: 5 km, 3 m², 60 km/h, 20 °C, 1.5 L, 250 g, 100 EUR, $20, 2 h 30 min
- conversions: 60 mph to km/h, 100 EUR in BAM, 6.5 L/100km to mpg, 0.25 to %, 255 to hex
- percentages: 20% of 50, 50 + 10%, 200 - 25%, 10% off 80
- variables: price = 12.5 EUR ; functions: f(x) = x^2 + 1 then f(3)
- labels before a colon are ignored: Rent: 450 EUR ; "total" adds up the lines above (back to an empty line)
- equations with one unknown are solved: 2x + 3 = 11 ; solve(x^2 = 9)
- functions: sqrt, cbrt, abs, round(x, 2), floor, ceil, ln, log, sin/cos/tan (degrees unless the notebook uses radians), asin, mean, median, sd, min, max, sum, gcd, lcm, nCr, pmt(rate, periods, amount), fv, pv, compound(start, rate, years, times), days(date1, date2), integral(expr, x, a, b), deriv(expr, x, at)
- dates: 2026-12-25 - today, today + 3 weeks
- comments start with #`

type aiImage struct {
	Mime string `json:"mime"`
	Data []byte `json:"-"`
}

// aiAsk runs one question, with pictures when there are any, and decodes
// the JSON answer into v.
func (s *Server) aiAsk(ctx context.Context, uid int64, st store.Settings, system, prompt string, images []aiImage, v any) error {
	cl, used, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		return &userErr{firstNonEmpty(why, "AI is not set up (Settings → AI)")}
	}
	if !st.AIHelp {
		return &userErr{"AI help is switched off (Settings → AI)"}
	}
	if len(images) == 0 {
		return cl.JSON(ctx, system, prompt, v)
	}
	var text string
	var err error
	if used == "pocket" {
		text, err = s.pocketVision(ctx, uid, st, system, prompt, images)
	} else {
		text, err = s.openaiVision(ctx, st, system, prompt, images)
	}
	if err != nil {
		return err
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(text), "```json"), "```"), "```"))
	if i := strings.IndexByte(text, '{'); i > 0 {
		text = text[i:]
	}
	if err := json.Unmarshal([]byte(text), v); err != nil {
		return fmt.Errorf("the AI's answer was not valid JSON: %w", err)
	}
	return nil
}

type pocketImage struct {
	Mime string `json:"mime"`
	Data string `json:"data"` // base64
}

func (s *Server) pocketVision(ctx context.Context, uid int64, st store.Settings, system, prompt string, images []aiImage) (string, error) {
	u, err := s.store.GetUser(ctx, uid)
	if err != nil {
		return "", err
	}
	tok, err := s.core.ActAs(ctx, u.Email, "ai", core.InferenceScope)
	if err != nil {
		return "", fmt.Errorf("Pocket AI: %w", err)
	}
	body := map[string]any{"system": system, "prompt": prompt, "json": true}
	if st.AIPocketModel != "" {
		body["model"] = st.AIPocketModel
	}
	var imgs []pocketImage
	for _, im := range images {
		imgs = append(imgs, pocketImage{Mime: im.Mime, Data: base64.StdEncoding.EncodeToString(im.Data)})
	}
	body["images"] = imgs
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 170*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(tok.URL, "/")+"/api/v1/inference", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 170 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("Pocket AI: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		if res.StatusCode == http.StatusBadRequest && strings.Contains(e.Error, "provide a prompt") {
			return "", &userErr{"this Pocket AI cannot look at photos yet: update Pocket AI, or use your own OpenAI key for photos (Settings → AI)"}
		}
		return "", fmt.Errorf("Pocket AI HTTP %d: %s", res.StatusCode, e.Error)
	}
	var out core.InferenceReply
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Text) == "" {
		return "", errors.New("Pocket AI returned no answer")
	}
	return out.Text, nil
}

func (s *Server) openaiVision(ctx context.Context, st store.Settings, system, prompt string, images []aiImage) (string, error) {
	base := strings.TrimRight(firstNonEmpty(st.OpenAIBaseURL, "https://api.openai.com/v1"), "/")
	model := firstNonEmpty(st.OpenAIModel, "gpt-4o-mini")
	parts := []map[string]any{{"type": "text", "text": prompt}}
	for _, im := range images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + im.Mime + ";base64," + base64.StdEncoding.EncodeToString(im.Data), "detail": "low"}})
	}
	body := map[string]any{"model": model, "response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": parts}}}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+st.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		return "", fmt.Errorf("AI HTTP %d: %s", res.StatusCode, e.Error.Message)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil || len(out.Choices) == 0 {
		return "", errors.New("the AI returned no answer")
	}
	return out.Choices[0].Message.Content, nil
}

// aiSlot keeps at most three AI questions running at once.
func (s *Server) aiSlot(w http.ResponseWriter) (func(), bool) {
	select {
	case s.aiSem <- struct{}{}:
		return func() { <-s.aiSem }, true
	default:
		writeErr(w, http.StatusTooManyRequests, "the AI is busy, try again in a moment")
		return nil, false
	}
}

func (s *Server) aiFail(w http.ResponseWriter, err error) {
	var ue *userErr
	if errors.As(err, &ue) {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
}

// workedLines works lines out and writes them with their answers, for a
// prompt (so the AI explains the real numbers).
func (s *Server) workedLines(ctx context.Context, uid int64, text string, deg bool) ([]calc.Line, string) {
	o := s.calcOptions(ctx, uid, nil)
	o.Deg = deg
	lines, _ := calc.EvalSheet(strings.Split(text, "\n"), o)
	var b strings.Builder
	for i, l := range strings.Split(text, "\n") {
		b.WriteString(l)
		if i < len(lines) {
			switch lines[i].Kind {
			case "value", "set":
				b.WriteString("   → " + lines[i].Text)
			case "error":
				b.WriteString("   → (not understood: " + lines[i].Error + ")")
			}
		}
		b.WriteString("\n")
	}
	return lines, b.String()
}

// handleAIExplain explains a calculation (or a question) step by step.
func (s *Server) handleAIExplain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Text     string `json:"text"`
		Question string `json:"question"`
		Level    string `json:"level"` // child | adult
		Angle    string `json:"angle"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Text+in.Question) == "" || len(in.Text) > 20000 {
		writeErr(w, http.StatusBadRequest, "write what to explain")
		return
	}
	done, ok := s.aiSlot(w)
	if !ok {
		return
	}
	defer done()
	st := s.settingsOf(ctx, uid)
	_, worked := s.workedLines(ctx, uid, in.Text, in.Angle != "rad")
	who := "an adult who is not a mathematician"
	if in.Level == "child" {
		who = "a child of about ten: short sentences, everyday examples, no jargon"
	}
	system := "You explain maths clearly and kindly, step by step, for " + who + ". Use the worked answers you are given; never redo arithmetic differently. Reply as JSON {\"steps\": [\"…\"], \"summary\": \"…\", \"tip\": \"…\"}. Each step is one or two sentences. Use plain text with simple maths (x², √, ÷), no LaTeX."
	prompt := ""
	if strings.TrimSpace(in.Text) != "" {
		prompt += "The calculation, with the calculator's answers:\n" + worked + "\n"
	}
	if q := strings.TrimSpace(in.Question); q != "" {
		prompt += "The question: " + clip(q, 4000) + "\n"
	}
	var out struct {
		Steps   []string `json:"steps"`
		Summary string   `json:"summary"`
		Tip     string   `json:"tip"`
	}
	if err := s.aiAsk(ctx, uid, st, system, prompt, nil, &out); err != nil {
		s.aiFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type aiLines struct {
	Title    string   `json:"title"`
	Problem  string   `json:"problem"`
	Lines    []string `json:"lines"`
	Note     string   `json:"note"`
	Graph    []string `json:"graph"`
	Equation string   `json:"equation"`
}

func (s *Server) finishLines(w http.ResponseWriter, r *http.Request, out aiLines) {
	ctx := r.Context()
	if len(out.Lines) > 60 {
		out.Lines = out.Lines[:60]
	}
	text := strings.Join(out.Lines, "\n")
	lines, _ := s.workedLines(ctx, actorID(ctx), text, true)
	writeJSON(w, http.StatusOK, map[string]any{"title": out.Title, "problem": out.Problem, "text": text, "results": lines, "note": out.Note, "graph": out.Graph, "equation": out.Equation})
}

const linesSystem = "You turn a maths or everyday problem into calculator lines that the calculator below can work out. Write the working as short lines with named variables and units, ending with the answer line(s). Do not write the answers yourself — the calculator works them out. If it helps, also give up to three graph expressions (like \"y = 2x + 3\") and one equation to solve. Reply as JSON {\"title\": \"…\", \"problem\": \"the problem in one or two sentences\", \"lines\": [\"…\"], \"note\": \"anything to check\", \"graph\": [], \"equation\": \"\"}.\n\n" + syntaxGuide

// handleAIWrite turns a problem in words into calculator lines.
func (s *Server) handleAIWrite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Text) == "" || len(in.Text) > 8000 {
		writeErr(w, http.StatusBadRequest, "write the problem in words")
		return
	}
	done, ok := s.aiSlot(w)
	if !ok {
		return
	}
	defer done()
	var out aiLines
	if err := s.aiAsk(ctx, uid, s.settingsOf(ctx, uid), linesSystem, "The problem:\n"+in.Text, nil, &out); err != nil {
		s.aiFail(w, err)
		return
	}
	s.finishLines(w, r, out)
}

// handleAIPhoto reads a sum or homework question from a picture (uploaded,
// or from Pocket Photos) and writes it as calculator lines.
func (s *Server) handleAIPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Image   string `json:"image"` // data:image/…;base64,…
		PhotoID string `json:"photo_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 24<<20)
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var img aiImage
	switch {
	case in.PhotoID != "":
		b, mime, err := s.photoBytes(ctx, uid, in.PhotoID)
		if err != nil {
			s.appFail(w, appPhotos, err)
			return
		}
		img = aiImage{Mime: mime, Data: b}
	case strings.HasPrefix(in.Image, "data:image/"):
		head, data, ok := strings.Cut(in.Image, ",")
		b, err := base64.StdEncoding.DecodeString(data)
		if !ok || err != nil || len(b) == 0 {
			writeErr(w, http.StatusBadRequest, "the picture could not be read")
			return
		}
		img = aiImage{Mime: strings.TrimSuffix(strings.TrimPrefix(head, "data:"), ";base64"), Data: b}
	default:
		writeErr(w, http.StatusBadRequest, "pick a picture")
		return
	}
	done, ok := s.aiSlot(w)
	if !ok {
		return
	}
	defer done()
	var out aiLines
	if err := s.aiAsk(ctx, uid, s.settingsOf(ctx, uid), linesSystem+"\n\nThe problem is in the picture (handwriting or print). Copy the problem into \"problem\" exactly first.", "Read the problem in this picture and write it as calculator lines.", []aiImage{img}, &out); err != nil {
		s.aiFail(w, err)
		return
	}
	s.finishLines(w, r, out)
}

// handleAIFormula drafts a saved calculation from a description; it is
// tried with its own example values before it is offered.
func (s *Server) handleAIFormula(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Text) == "" || len(in.Text) > 4000 {
		writeErr(w, http.StatusBadRequest, "say what the calculation should work out")
		return
	}
	done, ok := s.aiSlot(w)
	if !ok {
		return
	}
	defer done()
	system := "You design a reusable calculator form. Inputs are boxes the person fills in (name: a short variable name of letters/digits/_, label in plain words, unit like m, m², EUR, L/100km, kg or empty, default: an example value as a plain number, kind: number|percent|date|choice; for choice, options like \"Label=value|Label=value\"). The working (code) is calculator lines using the input names. Outputs are answers: label, expr (using variables), unit to show it in (or empty), decimals (0–4), main (true for the one big answer). Reply as JSON {\"name\", \"about\", \"category\", \"icon\", \"code\", \"inputs\": [], \"outputs\": []}.\n\n" + syntaxGuide
	var f struct {
		Name     string        `json:"name"`
		About    string        `json:"about"`
		Category string        `json:"category"`
		Icon     string        `json:"icon"`
		Code     string        `json:"code"`
		Inputs   []calc.Input  `json:"inputs"`
		Outputs  []calc.Output `json:"outputs"`
	}
	if err := s.aiAsk(ctx, uid, s.settingsOf(ctx, uid), system, "Make a calculation for: "+in.Text, nil, &f); err != nil {
		s.aiFail(w, err)
		return
	}
	res := s.runFormula(ctx, uid, f.Code, f.Inputs, f.Outputs, nil)
	writeJSON(w, http.StatusOK, map[string]any{"formula": f, "result": res})
}

// handleAICheck reads a notebook through and says what may be wrong.
func (s *Server) handleAICheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nb, ok := s.notebookOf(w, r, idParam(r, "id"))
	if !ok {
		return
	}
	if _, err := s.recompute(ctx, nb.UserID, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	done, ok := s.aiSlot(w)
	if !ok {
		return
	}
	defer done()
	md := s.notebookMarkdown(ctx, nb)
	system := "You check someone's calculations. Look for mistakes in the setup (wrong numbers, wrong units, a missing step, a percentage used the wrong way, a total that leaves something out) — the arithmetic itself is done by a calculator and is right. Be brief and kind. Reply as JSON {\"ok\": true|false, \"points\": [{\"line\": \"the line it is about\", \"text\": \"what to look at\"}], \"summary\": \"one sentence\"}."
	var out struct {
		OK     bool `json:"ok"`
		Points []struct {
			Line string `json:"line"`
			Text string `json:"text"`
		} `json:"points"`
		Summary string `json:"summary"`
	}
	if err := s.aiAsk(ctx, nb.UserID, s.settingsOf(ctx, nb.UserID), system, clip(md, 30000), nil, &out); err != nil {
		s.aiFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePhotoRead: a photo from Pocket Photos, read by AI.
func (s *Server) handlePhotoRead(w http.ResponseWriter, r *http.Request) {
	s.handleAIPhoto(w, r)
}
