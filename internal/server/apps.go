package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// The other Pocket apps. None of it is needed: when an app is not there,
// the page greys the button and says what to do.

func linkTo(sib *siblingConn, path string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + path
}

func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	return a.name + ": " + err.Error()
}

func (s *Server) appFail(w http.ResponseWriter, a pocketApp, err error) {
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if ue, ok := err.(*userErr); ok {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	writeErr(w, http.StatusBadGateway, plainErr(a, err))
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}

func (s *Server) sib(w http.ResponseWriter, r *http.Request, a pocketApp, scopes ...string) (*siblingConn, bool) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, a, scopes...)
	if err != nil {
		s.appFail(w, a, err)
		return nil, false
	}
	return sib, true
}

func (s *Server) actorName(u *store.User) string {
	if u == nil {
		return "Someone"
	}
	if n := strings.TrimSpace(u.Name); n != "" {
		return n
	}
	if i := strings.Index(u.Email, "@"); i > 0 {
		return u.Email[:i]
	}
	return "Someone"
}

// column is one column of a table of data, as the data cell keeps it.
type column struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func fnum(f float64, dec int) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return ""
	}
	return strconv.FormatFloat(math.Round(f*math.Pow10(dec))/math.Pow10(dec), 'f', -1, 64)
}

// ---- Pocket Notes ----

func (s *Server) handleNotesSearch(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appNotes, scopeNotesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/search", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"30"}})
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "hits", "notes") {
		n := m
		if inner, ok := m["note"].(map[string]any); ok {
			n = inner
		}
		out = append(out, map[string]any{"id": idOf(n), "title": firstNonEmpty(str(n, "title"), "Untitled"), "folder": str(n, "category"), "snippet": clip(firstNonEmpty(str(m, "snippet"), str(n, "excerpt")), 200)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": out})
}

// handleNoteText brings a note's words in (for a text cell).
func (s *Server) handleNoteText(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "nid")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeErr(w, http.StatusBadRequest, "pick a note")
		return
	}
	sib, ok := s.sib(w, r, appNotes, scopeNotesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/notes/"+id, nil)
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	m, _ := v.(map[string]any)
	if inner := obj(m, "note"); inner != nil {
		m = inner
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": str(m, "title"), "text": clip(firstNonEmpty(str(m, "content"), str(m, "body"), str(m, "text")), maxCellBody), "url": linkTo(sib, "/notes/"+id)})
}

// handleToNotes keeps a notebook (with its answers) as a note.
func (s *Server) handleToNotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		NotebookID int64 `json:"notebook_id"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	nb, ok := s.notebookOf(w, r, in.NotebookID)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appNotes, scopeNotesWrite)
	if !ok {
		return
	}
	if _, err := s.recompute(ctx, nb.UserID, nb); err != nil {
		s.fail(w, err, "work out")
		return
	}
	folder := firstNonEmpty(s.settingsOf(ctx, nb.UserID).NotesFolder, "Math")
	md := s.notebookMarkdown(ctx, nb) + "\n---\n[Open in Pocket Math](" + s.notebookURL(nb) + ")\n"
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(nb.Title, 200), "content": md, "category": folder})
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/notes/"+idOf(res)), "text": "Kept in Pocket Notes (" + folder + ")"})
}

// ---- Pocket Files ----

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appFiles, scopeFilesRead)
	if !ok {
		return
	}
	space := firstNonEmpty(r.URL.Query().Get("space"), "home")
	path := firstNonEmpty(r.URL.Query().Get("path"), "/")
	v, err := getJSONFrom(ctx, sib, "/api/v1/fs/list", url.Values{"space": {space}, "path": {path}})
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	out := []map[string]any{}
	for _, e := range rows(v, "entries") {
		name := str(e, "name")
		dir := e["is_dir"] == true
		ext := ""
		if i := strings.LastIndex(name, "."); i >= 0 {
			ext = strings.ToLower(name[i+1:])
		}
		kind := map[string]string{"csv": "table", "tsv": "table", "txt": "table"}[ext]
		if dir {
			kind = "folder"
		}
		if kind == "" || strings.HasPrefix(name, ".") {
			continue
		}
		out = append(out, map[string]any{"name": name, "path": str(e, "path"), "kind": kind, "size": num(e, "size"), "modified": str(e, "modified")})
	}
	spaces := []map[string]any{}
	if sv, err := getJSONFrom(ctx, sib, "/api/v1/spaces", nil); err == nil {
		for _, sp := range rows(sv, "spaces") {
			spaces = append(spaces, map[string]any{"id": str(sp, "id"), "name": str(sp, "name")})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "space": space, "path": path, "spaces": spaces})
}

// parseGrid reads CSV (or tab/semicolon separated) text into columns.
func parseGrid(b []byte) ([]column, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	first := string(b)
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	cr := csv.NewReader(bytes.NewReader(b))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	switch {
	case strings.Count(first, "\t") > strings.Count(first, ","):
		cr.Comma = '\t'
	case strings.Count(first, ";") > strings.Count(first, ","):
		cr.Comma = ';'
	}
	grid, err := cr.ReadAll()
	if err != nil || len(grid) == 0 {
		return nil, &userErr{"the table could not be read"}
	}
	return gridColumns(grid), nil
}

func gridColumns(grid [][]string) []column {
	if len(grid) == 0 {
		return []column{}
	}
	w := 0
	for _, row := range grid {
		if len(row) > w {
			w = len(row)
		}
	}
	if w > 40 {
		w = 40
	}
	if len(grid) > 5001 {
		grid = grid[:5001]
	}
	cols := make([]column, w)
	for i := range cols {
		if i < len(grid[0]) {
			cols[i].Name = clip(strings.TrimSpace(grid[0][i]), 60)
		}
		if cols[i].Name == "" {
			cols[i].Name = fmt.Sprintf("Column %d", i+1)
		}
		cols[i].Values = []string{}
	}
	for _, row := range grid[1:] {
		for i := range cols {
			v := ""
			if i < len(row) {
				v = clip(strings.TrimSpace(row[i]), 200)
			}
			cols[i].Values = append(cols[i].Values, v)
		}
	}
	return cols
}

// handleFromFiles reads a CSV from Pocket Files into columns.
func (s *Server) handleFromFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Space string `json:"space"`
		Path  string `json:"path"`
	}
	if err := decode(r, &in); err != nil || in.Path == "" {
		writeErr(w, http.StatusBadRequest, "pick a file")
		return
	}
	sib, ok := s.sib(w, r, appFiles, scopeFilesRead)
	if !ok {
		return
	}
	b, _, err := sib.client.Bytes(ctx, "/api/v1/fs/raw", url.Values{"space": {firstNonEmpty(in.Space, "home")}, "path": {in.Path}}, 8<<20)
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	cols, err := parseGrid(b)
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	name := in.Path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "name": name})
}

// handleToFiles saves something made here in Pocket Files: a table (CSV), a
// drawing or graph (SVG or PNG), a notebook (Markdown).
func (s *Server) handleToFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"` // csv | svg | png | md
		Text   string `json:"text"`
		Base64 string `json:"base64"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	mime := map[string]string{"csv": "text/csv", "svg": "image/svg+xml", "png": "image/png", "md": "text/markdown"}[in.Kind]
	if mime == "" {
		writeErr(w, http.StatusBadRequest, "save a csv, svg, png or md")
		return
	}
	data := []byte(in.Text)
	if in.Base64 != "" {
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(in.Base64, "data:image/png;base64,"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the picture could not be read")
			return
		}
		data = b
	}
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "there is nothing to save")
		return
	}
	sib, ok := s.sib(w, r, appFiles, scopeFilesWrite)
	if !ok {
		return
	}
	name := fileSafe(firstNonEmpty(in.Name, "Pocket Math")) + "." + in.Kind
	dir := strings.Trim(firstNonEmpty(s.settingsOf(ctx, actorID(ctx)).FilesFolder, "Math"), "/")
	var res map[string]any
	q := url.Values{"space": {"home"}, "path": {"/"}, "name": {dir + "/" + name}, "conflict": {"rename"}, "mtime": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	if err := sib.client.PutRaw(ctx, "/api/v1/fs/upload", q, data, mime, &res); err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	p := firstNonEmpty(str(res, "path"), "/"+dir+"/"+name)
	folder := p
	if i := strings.LastIndex(p, "/"); i > 0 {
		folder = p[:i]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Saved in Pocket Files as " + p, "url": linkTo(sib, "/f/home"+folder)})
}

// ---- Pocket Photos ----

func (s *Server) handlePhotosList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	body := map[string]any{"filter": map[string]any{"trashedAt": map[string]any{"eq": nil}, "type": map[string]any{"eq": "IMAGE"}, "visibility": map[string]any{"eq": "timeline"}},
		"size": 60, "withExif": false, "orderBy": map[string]any{"field": "fileCreatedAt", "direction": "desc"}}
	path := "/api/v1/immich/search/metadata"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		body["query"] = q
		path = "/api/v1/immich/search/smart"
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		body["cursor"] = c
	}
	res, err := postJSONTo(r.Context(), sib, path, body)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	assets := obj(res, "assets")
	out := []map[string]any{}
	for _, m := range rows(assets["items"]) {
		id := str(m, "id")
		if !reUUID.MatchString(id) {
			continue
		}
		out = append(out, map[string]any{"id": id, "name": str(m, "originalFileName"), "date": day10(m["fileCreatedAt"]), "thumb_url": "/api/v1/apps/photos/" + id + "/image"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": out, "cursor": assets["nextCursor"]})
}

func (s *Server) handlePhotoImage(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	b, ctype, err := sib.client.Bytes(r.Context(), "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"thumbnail"}}, 16<<20)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	w.Header().Set("Content-Type", firstNonEmpty(ctype, http.DetectContentType(b)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(b)
}

// photoBytes fetches a larger picture (for AI to read).
func (s *Server) photoBytes(ctx context.Context, uid int64, id string) ([]byte, string, error) {
	if !reUUID.MatchString(id) {
		return nil, "", &userErr{"pick a photo"}
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appPhotos, scopePhotosRead)
	if err != nil {
		return nil, "", err
	}
	b, ctype, err := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}, 16<<20)
	if err != nil {
		return nil, "", err
	}
	return b, firstNonEmpty(ctype, http.DetectContentType(b)), nil
}

// ---- Pocket Chat ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appChat, scopeChatRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/conversations", nil)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "conversations") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "name"), str(m, "title")), "kind": str(m, "kind"), "emoji": str(m, "emoji")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

// handleShareChat sends an answer (and a notebook's link) to a chat. A
// notebook is shared (read-only link) when it is sent.
func (s *Server) handleShareChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		To         string   `json:"to"`
		Text       string   `json:"text"`
		NotebookID int64    `json:"notebook_id"`
		Title      string   `json:"title"`
		Lines      []string `json:"lines"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	uid := actorID(ctx)
	var card map[string]any
	if in.NotebookID > 0 {
		nb, ok := s.notebookOf(w, r, in.NotebookID)
		if !ok {
			return
		}
		if !nb.Shared || nb.ShareKey == "" {
			nb.Shared = true
			if nb.ShareKey == "" {
				nb.ShareKey = newToken()[:20]
			}
			if err := s.store.UpdateNotebook(ctx, nb); err != nil {
				s.fail(w, err, "share")
				return
			}
		}
		card = map[string]any{"title": nb.Title, "subtitle": firstNonEmpty(nb.Subject, "Pocket Math notebook"), "lines": firstLines(strings.Join(in.Lines, "\n"), 4), "icon": "🧮", "url": s.shareURL(nb), "app": "math", "kind": "card"}
	} else if len(in.Lines) > 0 {
		card = map[string]any{"title": firstNonEmpty(in.Title, "Worked out"), "lines": firstLines(strings.Join(in.Lines, "\n"), 6), "icon": "🧮", "app": "math", "kind": "card"}
	}
	text := strings.TrimSpace(in.Text)
	if text == "" && card == nil {
		writeErr(w, http.StatusBadRequest, "there is nothing to send")
		return
	}
	sib, ok := s.sib(w, r, appChat, scopeChatWrite)
	if !ok {
		return
	}
	st := s.settingsOf(ctx, uid)
	body := map[string]any{"to": firstNonEmpty(strings.TrimSpace(in.To), st.ChatTo, "me"), "text": clip(text, 4000), "client_id": fmt.Sprintf("math-%d-%d-%d", uid, in.NotebookID, time.Now().Unix()/10)}
	if card != nil {
		body["card"] = card
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", body)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/c/"+str(res, "conversation_id")), "text": "Sent to Pocket Chat"})
}

// ---- Pocket Tables ----

func (s *Server) handleTablesList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appTables, scopeTablesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/bases", nil)
	if err != nil {
		s.appFail(w, appTables, err)
		return
	}
	out := []map[string]any{}
	for _, b := range rows(v, "bases") {
		for _, t := range rows(b["tables"]) {
			out = append(out, map[string]any{"id": idOf(t), "name": str(t, "name"), "base": str(b, "name"), "icon": str(b, "icon")})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": out})
}

func (s *Server) tableColumns(ctx context.Context, sib *siblingConn, id string) ([]column, error) {
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return nil, &userErr{"pick a table"}
	}
	b, _, err := sib.client.Bytes(ctx, "/api/v1/tables/"+id+"/export.csv", nil, 8<<20)
	if err != nil {
		return nil, err
	}
	return parseGrid(b)
}

// handleTablePreview gives a table as columns (its CSV export).
func (s *Server) handleTablePreview(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appTables, scopeTablesRead)
	if !ok {
		return
	}
	id := chiParam(r, "tid")
	cols, err := s.tableColumns(r.Context(), sib, id)
	if err != nil {
		s.appFail(w, appTables, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "url": linkTo(sib, "/t/"+id)})
}

// handleTableFields lists the column names (to match answers to them).
func (s *Server) handleTableFields(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appTables, scopeTablesRead)
	if !ok {
		return
	}
	cols, err := s.tableColumns(r.Context(), sib, chiParam(r, "tid"))
	if err != nil {
		s.appFail(w, appTables, err)
		return
	}
	names := []string{}
	for _, c := range cols {
		names = append(names, c.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": names})
}

// handleTableRows adds rows to a table: {rows: [{field: value}]}.
func (s *Server) handleTableRows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chiParam(r, "tid")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeErr(w, http.StatusBadRequest, "pick a table")
		return
	}
	var in struct {
		Rows []map[string]string `json:"rows"`
	}
	if err := decode(r, &in); err != nil || len(in.Rows) == 0 || len(in.Rows) > 5000 {
		writeErr(w, http.StatusBadRequest, "there are no rows to add")
		return
	}
	sib, ok := s.sib(w, r, appTables, scopeTablesWrite)
	if !ok {
		return
	}
	recs := make([]map[string]any, 0, len(in.Rows))
	for _, row := range in.Rows {
		recs = append(recs, map[string]any{"values": row})
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/tables/"+id+"/records/batch", map[string]any{"records": recs, "typecast": true})
	if err != nil {
		s.appFail(w, appTables, err)
		return
	}
	n := int(num(res, "made"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": fmt.Sprintf("%d %s added in Pocket Tables", n, plural(n, "row", "rows")), "url": linkTo(sib, "/t/"+id)})
}

// ---- Pocket Learn ----

// handleToLearn turns formulas or worked answers into flashcards.
func (s *Server) handleToLearn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Deck  string `json:"deck"`
		Cards []struct {
			Front string `json:"front"`
			Back  string `json:"back"`
			Ref   string `json:"ref"`
		} `json:"cards"`
	}
	if err := decode(r, &in); err != nil || len(in.Cards) == 0 || len(in.Cards) > 500 {
		writeErr(w, http.StatusBadRequest, "there are no cards")
		return
	}
	sib, ok := s.sib(w, r, appLearn, scopeLearnWrite)
	if !ok {
		return
	}
	uid := actorID(ctx)
	deck := firstNonEmpty(strings.TrimSpace(in.Deck), s.settingsOf(ctx, uid).LearnDeck, "Maths")
	n, dup := 0, 0
	for _, c := range in.Cards {
		if strings.TrimSpace(c.Front) == "" || strings.TrimSpace(c.Back) == "" {
			continue
		}
		res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"deck": deck, "kind": "basic", "front": clip(c.Front, 500), "back": clip(c.Back, 2000),
			"external_id": "math:" + firstNonEmpty(c.Ref, fmt.Sprintf("%x", c.Front)), "source_ref": "math", "source_url": s.link("/")})
		if err != nil {
			if n == 0 {
				s.appFail(w, appLearn, err)
				return
			}
			break
		}
		if res["duplicate"] == true {
			dup++
		} else {
			n++
		}
	}
	text := fmt.Sprintf("%d %s added to “%s” in Pocket Learn", n, plural(n, "card", "cards"), deck)
	if dup > 0 {
		text += fmt.Sprintf(" (%d already there)", dup)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text, "url": linkTo(sib, "/")})
}

// ---- Pocket Slides ----

// handleToSlides makes a deck with a chart (or a table) slide.
func (s *Server) handleToSlides(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Title  string     `json:"title"`
		Kind   string     `json:"kind"` // bar | line | pie | table
		Labels []string   `json:"labels"`
		Values []float64  `json:"values"`
		Unit   string     `json:"unit"`
		Rows   [][]string `json:"rows"`
		Notes  string     `json:"notes"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	title := firstNonEmpty(cleanText(in.Title, 120), "From Pocket Math")
	var slide map[string]any
	if in.Kind == "table" {
		if len(in.Rows) == 0 {
			writeErr(w, http.StatusBadRequest, "the table is empty")
			return
		}
		rowsOut := in.Rows
		if len(rowsOut) > 16 {
			rowsOut = rowsOut[:16]
		}
		for i := range rowsOut {
			if len(rowsOut[i]) > 8 {
				rowsOut[i] = rowsOut[i][:8]
			}
		}
		slide = map[string]any{"layout": "table", "title": title, "data": map[string]any{"rows": rowsOut}, "notes": in.Notes}
	} else {
		if len(in.Values) == 0 || len(in.Values) != len(in.Labels) {
			writeErr(w, http.StatusBadRequest, "the chart needs a label for every number")
			return
		}
		labels, values := in.Labels, in.Values
		if len(values) > 24 {
			labels, values = labels[len(labels)-24:], values[len(values)-24:]
		}
		kind := in.Kind
		if !oneOf(kind, []string{"bar", "hbar", "line", "pie", "donut"}) {
			kind = "bar"
		}
		slide = map[string]any{"layout": "chart", "title": title, "data": map[string]any{"chart": map[string]any{"kind": kind, "labels": labels, "values": values, "unit": clip(in.Unit, 20)}}, "notes": in.Notes}
	}
	sib, ok := s.sib(w, r, appSlides, scopeSlidesWrite)
	if !ok {
		return
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/decks", map[string]any{"title": title, "slides": []any{slide}})
	if err != nil {
		s.appFail(w, appSlides, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Made a slide in Pocket Slides", "url": firstNonEmpty(str(res, "url"), linkTo(sib, "/"))})
}

// ---- Pocket Rooms ----

// roomSizes lists every room of every plan with its sizes in metres.
func (s *Server) roomSizes(ctx context.Context, uid int64) ([]map[string]any, error) {
	sib, err := s.clients(ctx, uid).conn(ctx, appRooms, scopeRoomsRead)
	if err != nil {
		return nil, err
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/plans", nil)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, p := range rows(v, "plans") {
		if p["archived"] == true {
			continue
		}
		pid := idOf(p)
		sv, err := getJSONFrom(ctx, sib, "/api/v1/plans/"+pid+"/summary", nil)
		if err != nil {
			continue
		}
		sm, _ := sv.(map[string]any)
		for _, f := range rows(sm["floors"]) {
			ceil := num(f, "ceiling_cm")
			for _, rm := range rows(f["rooms"]) {
				h := ceil
				if rh := num(rm, "height_cm"); rh > 0 {
					h = rh
				}
				out = append(out, map[string]any{
					"plan": str(p, "name"), "floor": str(f, "floor_name"), "room": firstNonEmpty(str(rm, "name"), "Room"),
					"floor_m2": num(rm, "area_cm2") / 1e4, "wall_m2": num(rm, "wall_cm2") / 1e4, "perimeter_m": num(rm, "perimeter_cm") / 100,
					"skirting_m": num(rm, "skirting_cm") / 100, "height_m": h / 100, "url": linkTo(sib, "/p/"+pid),
				})
			}
		}
	}
	return out, nil
}

func (s *Server) handleRoomsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rooms, err := s.roomSizes(ctx, actorID(ctx))
	if err != nil {
		s.appFail(w, appRooms, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

// ---- Pocket Home ----

func (s *Server) handleMeters(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appHome, scopeHomeRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/meters", nil)
	if err != nil {
		s.appFail(w, appHome, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "meters") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": str(m, "name"), "kind": str(m, "kind"), "unit": str(m, "unit"), "home": str(m, "home_name"),
			"price": m["price_per_unit"], "currency": str(m, "currency"), "readings": num(m, "readings")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"meters": out})
}

// handleMeterData: use per month (and cost) as columns.
func (s *Server) handleMeterData(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "mid")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeErr(w, http.StatusBadRequest, "pick a meter")
		return
	}
	sib, ok := s.sib(w, r, appHome, scopeHomeRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/meters/"+id+"/usage", url.Values{"months": {"24"}})
	if err != nil {
		s.appFail(w, appHome, err)
		return
	}
	m, _ := v.(map[string]any)
	meter := obj(m, "meter")
	unit := str(meter, "unit")
	month, use, cost := column{Name: "Month", Values: []string{}}, column{Name: "Use (" + firstNonEmpty(unit, "units") + ")", Values: []string{}}, column{Name: "Cost", Values: []string{}}
	hasCost := false
	for _, mo := range rows(m["months"]) {
		month.Values = append(month.Values, str(mo, "month"))
		use.Values = append(use.Values, fnum(num(mo, "usage"), 2))
		c := ""
		if mo["cost"] != nil {
			c, hasCost = fnum(num(mo, "cost"), 2), true
		}
		cost.Values = append(cost.Values, c)
	}
	cols := []column{month, use}
	if hasCost {
		cost.Name = "Cost (" + firstNonEmpty(str(meter, "currency"), "EUR") + ")"
		cols = append(cols, cost)
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "name": str(meter, "name")})
}

// ---- Pocket Garage ----

func (s *Server) handleVehicles(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appGarage, scopeVehiclesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/vehicles", url.Values{"status": {"active"}})
	if err != nil {
		s.appFail(w, appGarage, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "vehicles") {
		sum := obj(m, "summary")
		eco := num(sum, "economy_recent")
		if eco == 0 {
			eco = num(sum, "economy")
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "display_name"), str(m, "name")), "economy": eco, "currency": str(m, "currency"), "fuel_type": str(m, "fuel_type")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": out})
}

func (s *Server) fuelLog(ctx context.Context, sib *siblingConn, id string) ([]map[string]any, error) {
	v, err := getJSONFrom(ctx, sib, "/api/v1/vehicles/"+id+"/fuel", nil)
	if err != nil {
		return nil, err
	}
	list := rows(v, "fuel", "items")
	sort.SliceStable(list, func(i, j int) bool { return str(list[i], "date") < str(list[j], "date") })
	return list, nil
}

// handleVehicleData: the fill-ups as columns.
func (s *Server) handleVehicleData(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "vid")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeErr(w, http.StatusBadRequest, "pick a vehicle")
		return
	}
	sib, ok := s.sib(w, r, appGarage, scopeVehiclesRead)
	if !ok {
		return
	}
	list, err := s.fuelLog(r.Context(), sib, id)
	if err != nil {
		s.appFail(w, appGarage, err)
		return
	}
	cur := "EUR"
	cols := []column{{Name: "Date"}, {Name: "Odometer (km)"}, {Name: "Fuel (L)"}, {Name: "Cost"}, {Name: "Price per litre"}, {Name: "L/100km"}}
	for i := range cols {
		cols[i].Values = []string{}
	}
	for _, f := range list {
		if c := str(f, "currency"); c != "" {
			cur = c
		}
		eco := ""
		if f["economy"] != nil {
			eco = fnum(num(f, "economy"), 2)
		}
		vals := []string{day10(f["date"]), fnum(num(f, "odometer"), 0), fnum(num(f, "volume"), 2), fnum(num(f, "cost")/100, 2), fnum(num(f, "price_per")/100, 3), eco}
		for i := range cols {
			cols[i].Values = append(cols[i].Values, vals[i])
		}
	}
	cols[3].Name, cols[4].Name = "Cost ("+cur+")", "Price ("+cur+"/L)"
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols})
}

// ---- Pocket Health ----

func (s *Server) handleHealthData(w http.ResponseWriter, r *http.Request) {
	metric := chiParam(r, "metric")
	if !oneOf(metric, []string{"weight", "height", "body_fat", "bmi"}) {
		writeErr(w, http.StatusBadRequest, "pick weight, height, body fat or BMI")
		return
	}
	sib, ok := s.sib(w, r, appHealth, scopeHealthRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/metrics/"+metric, url.Values{"range": {firstNonEmpty(r.URL.Query().Get("range"), "y")}})
	if err != nil {
		s.appFail(w, appHealth, err)
		return
	}
	m, _ := v.(map[string]any)
	info := obj(m, "metric")
	date, val := column{Name: "Date", Values: []string{}}, column{Name: firstNonEmpty(str(info, "name"), metric), Values: []string{}}
	if u := str(info, "unit"); u != "" {
		val.Name += " (" + u + ")"
	}
	for _, p := range rows(m["points"]) {
		date.Values = append(date.Values, day10(p["date"]))
		val.Values = append(val.Values, fnum(num(p, "value"), 2))
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": []column{date, val}, "latest": obj(m, "latest"), "unit": str(info, "unit")})
}

// ---- Pocket Money ----

func (s *Server) handleMoneyAccounts(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appMoney, scopeAccountsRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/accounts", nil)
	if err != nil {
		s.appFail(w, appMoney, err)
		return
	}
	out := []map[string]any{}
	for _, a := range rows(v, "accounts") {
		if a["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(a), "name": str(a, "name"), "currency": str(a, "currency"), "balance": num(a, "balance") / 100})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

// handleMoneyMonths: spending and income per month (whole months), as
// columns, from the transactions themselves.
func (s *Server) handleMoneyMonths(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	months := clampInt(firstPos(queryInt(r, "months"), 12), 1, 36)
	sib, ok := s.sib(w, r, appMoney, scopeTxRead)
	if !ok {
		return
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -(months - 1), 0)
	spent, income := map[string]float64{}, map[string]float64{}
	cur := ""
	for offset := 0; offset < 20000; offset += 1000 {
		q := url.Values{"from": {start.Format("2006-01-02")}, "to": {now.Format("2006-01-02")}, "limit": {"1000"}, "offset": {strconv.Itoa(offset)}}
		if c := r.URL.Query().Get("category"); c != "" {
			q.Set("category", c)
		}
		v, err := getJSONFrom(ctx, sib, "/api/v1/transactions", q)
		if err != nil {
			s.appFail(w, appMoney, err)
			return
		}
		m, _ := v.(map[string]any)
		cur = firstNonEmpty(cur, str(m, "currency"))
		for _, t := range rows(m["items"]) {
			mo := day10(t["date"])
			if len(mo) < 7 {
				continue
			}
			mo = mo[:7]
			amt := num(t, "amount_home")
			if t["amount_home"] == nil {
				amt = num(t, "amount")
			}
			switch str(t, "kind") {
			case "expense", "withdrawal":
				spent[mo] += math.Abs(amt) / 100
			case "income", "deposit":
				income[mo] += math.Abs(amt) / 100
			}
		}
		if m["has_more"] != true {
			break
		}
	}
	cur = firstNonEmpty(cur, "EUR")
	cols := []column{{Name: "Month", Values: []string{}}, {Name: "Spent (" + cur + ")", Values: []string{}}, {Name: "Came in (" + cur + ")", Values: []string{}}}
	for t := start; !t.After(now); t = t.AddDate(0, 1, 0) {
		mo := t.Format("2006-01")
		cols[0].Values = append(cols[0].Values, mo)
		cols[1].Values = append(cols[1].Values, fnum(spent[mo], 2))
		cols[2].Values = append(cols[2].Values, fnum(income[mo], 2))
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "currency": cur})
}

// ---- Pocket Wealth ----

func (s *Server) handleWealthData(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appWealth, scopeWealthRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/history", url.Values{"months": {"24"}})
	if err != nil {
		s.appFail(w, appWealth, err)
		return
	}
	m, _ := v.(map[string]any)
	cur := firstNonEmpty(str(m, "currency"), "EUR")
	cols := []column{{Name: "Date", Values: []string{}}, {Name: "Net worth (" + cur + ")", Values: []string{}}, {Name: "Savings (" + cur + ")", Values: []string{}}}
	for _, p := range rows(m["points"]) {
		cols[0].Values = append(cols[0].Values, day10(p["date"]))
		cols[1].Values = append(cols[1].Values, fnum(num(p, "net")/100, 2))
		cols[2].Values = append(cols[2].Values, fnum(num(p, "liquid")/100, 2))
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "currency": cur})
}

// ---- Pocket Habits ----

func (s *Server) handleHabitsList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appHabits, scopeHabitsRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/habits", nil)
	if err != nil {
		s.appFail(w, appHabits, err)
		return
	}
	out := []map[string]any{}
	for _, h := range rows(v, "habits") {
		out = append(out, map[string]any{"id": num(h, "id"), "name": str(h, "name"), "color": str(h, "color")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"habits": out})
}

// tickHabit marks today done for a habit in Pocket Habits.
func (s *Server) tickHabit(ctx context.Context, uid, habit int64) (bool, error) {
	sib, err := s.clients(ctx, uid).conn(ctx, appHabits, scopeEntriesWrite)
	if err != nil {
		return false, err
	}
	var res map[string]any
	if err := sib.client.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/habits/%d/entries/%s", habit, today()), nil, map[string]any{"status": 1}, &res); err != nil {
		s.log.Debug("pocket habits: tick failed", "err", err)
		return false, err
	}
	return true, nil
}

// ---- Pocket Family ----

func (s *Server) handleFamilyKids(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appFamily, scopeFamilyRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/family", nil)
	if err != nil {
		s.appFail(w, appFamily, err)
		return
	}
	m, _ := v.(map[string]any)
	if inner := obj(m, "family"); inner != nil {
		m = inner
	}
	out := []map[string]any{}
	for _, p := range rows(m["people"]) {
		if str(p, "role") != "child" || p["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"name": str(p, "name"), "emoji": str(p, "emoji"), "class": str(p, "class_name"), "birthday": str(p, "birthday")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"children": out})
}

// ---- Pocket Alerts ----

// alert tells the person through Pocket Alerts (best effort, never in the way).
func (s *Server) alert(ctx context.Context, uid int64, title, body, path, priority string) bool {
	sib, err := s.clients(ctx, uid).conn(ctx, appAlerts, scopeAlertsWrite)
	if err != nil {
		return false
	}
	_, err = postJSONTo(ctx, sib, "/api/v1/alerts", map[string]any{"title": clip(title, 200), "body": clip(body, 2000), "priority": priority, "tags": []string{"abacus"}, "click": s.link(path)})
	if err != nil {
		s.log.Debug("pocket alerts: send failed", "err", err)
		return false
	}
	return true
}

// ---- filling in a calculation from another app ----

// handleFill lists the values another app can fill a box with:
// ?source=rooms:wall | rooms:floor | rooms:perimeter | rooms:skirting |
// rooms:height | health:weight | health:height | garage:economy |
// garage:price | wealth:liquid | wealth:net | home:price | money:balance.
func (s *Server) handleFill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	app, what, _ := strings.Cut(r.URL.Query().Get("source"), ":")
	opts := []map[string]any{}
	add := func(label, value, sub string) {
		opts = append(opts, map[string]any{"label": label, "value": value, "sub": sub})
	}
	var err error
	var a pocketApp
	switch app {
	case "rooms":
		a = appRooms
		var rooms []map[string]any
		rooms, err = s.roomSizes(ctx, uid)
		key := map[string]string{"wall": "wall_m2", "floor": "floor_m2", "perimeter": "perimeter_m", "skirting": "skirting_m", "height": "height_m"}[what]
		unit := map[string]string{"wall": "m²", "floor": "m²", "perimeter": "m", "skirting": "m", "height": "m"}[what]
		for _, rm := range rooms {
			if v, _ := rm[key].(float64); v > 0 {
				add(fmt.Sprint(rm["room"]), fnum(v, 2)+" "+unit, joinNonEmpty(" · ", fmt.Sprint(rm["plan"]), fmt.Sprint(rm["floor"])))
			}
		}
	case "health":
		a = appHealth
		var sib *siblingConn
		if sib, err = s.clients(ctx, uid).conn(ctx, appHealth, scopeHealthRead); err == nil {
			var v any
			metric := map[string]string{"weight": "weight", "height": "height"}[what]
			if v, err = getJSONFrom(ctx, sib, "/api/v1/metrics/"+metric, url.Values{"range": {"all"}}); err == nil {
				m, _ := v.(map[string]any)
				latest := obj(m, "latest")
				unit := str(obj(m, "metric"), "unit")
				if val := num(latest, "value"); val > 0 {
					add("Your last "+what, fnum(val, 1)+" "+unit, day10(latest["date"]))
				}
			}
		}
	case "garage":
		a = appGarage
		var sib *siblingConn
		if sib, err = s.clients(ctx, uid).conn(ctx, appGarage, scopeVehiclesRead); err == nil {
			var v any
			if v, err = getJSONFrom(ctx, sib, "/api/v1/vehicles", url.Values{"status": {"active"}}); err == nil {
				for _, veh := range rows(v, "vehicles") {
					name := firstNonEmpty(str(veh, "display_name"), str(veh, "name"))
					sum := obj(veh, "summary")
					if what == "economy" {
						eco := num(sum, "economy_recent")
						if eco == 0 {
							eco = num(sum, "economy")
						}
						if eco > 0 {
							add(name, fnum(eco, 2)+" L/100km", "from your fill-ups")
						}
						continue
					}
					if list, e := s.fuelLog(ctx, sib, idOf(veh)); e == nil && len(list) > 0 {
						last := list[len(list)-1]
						if p := num(last, "price_per"); p > 0 {
							add(name, fnum(p/100, 3)+" "+firstNonEmpty(str(last, "currency"), str(veh, "currency"), "EUR")+"/L", "last fill-up "+day10(last["date"]))
						}
					}
				}
			}
		}
	case "wealth":
		a = appWealth
		var sib *siblingConn
		if sib, err = s.clients(ctx, uid).conn(ctx, appWealth, scopeWealthRead); err == nil {
			var v any
			if v, err = getJSONFrom(ctx, sib, "/api/v1/summary", nil); err == nil {
				m, _ := v.(map[string]any)
				cur := firstNonEmpty(str(m, "currency"), "EUR")
				add("Savings you can reach", fnum(num(m, "liquid")/100, 2)+" "+cur, "cash and savings")
				add("Net worth", fnum(num(m, "net")/100, 2)+" "+cur, "all you own less what you owe")
			}
		}
	case "home":
		a = appHome
		var sib *siblingConn
		if sib, err = s.clients(ctx, uid).conn(ctx, appHome, scopeHomeRead); err == nil {
			var v any
			if v, err = getJSONFrom(ctx, sib, "/api/v1/meters", nil); err == nil {
				for _, m := range rows(v, "meters") {
					if p, ok := m["price_per_unit"].(float64); ok && p > 0 {
						add(str(m, "name"), fnum(p, 4)+" "+firstNonEmpty(str(m, "currency"), "EUR")+"/"+firstNonEmpty(str(m, "unit"), "kWh"), str(m, "home_name"))
					}
				}
			}
		}
	case "money":
		a = appMoney
		var sib *siblingConn
		if sib, err = s.clients(ctx, uid).conn(ctx, appMoney, scopeAccountsRead); err == nil {
			var v any
			if v, err = getJSONFrom(ctx, sib, "/api/v1/accounts", nil); err == nil {
				for _, ac := range rows(v, "accounts") {
					if ac["archived"] != true {
						add(str(ac, "name"), fnum(num(ac, "balance")/100, 2)+" "+firstNonEmpty(str(ac, "currency"), "EUR"), "balance")
					}
				}
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "unknown source")
		return
	}
	if err != nil {
		s.appFail(w, a, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": opts, "app": a.name})
}
