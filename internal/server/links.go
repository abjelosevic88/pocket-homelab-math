package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Math as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes, portable or capabilities stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "math:read", Name: "See your maths", Description: "Read your notebooks (calculations with their answers, graphs, tables of data), your saved calculations, the answers you kept and how practice is going — and work out sums and unit conversions exactly: for your journal, search and Pocket AI."},
	{Scope: "math:write", Name: "Work things out for you", Description: "Start a notebook with calculations, add cells to it, save a calculation and keep an answer (from Pocket AI, or another app). Changes or removes only what that app made itself; never shares or deletes your own notebooks."},
}

// portable is what Pocket Move may copy out and bring in for a person
// (docs/move.md in pocket-homelab-core); Register adds move:export/import.
var portable = &core.Portable{
	Exports: []core.PortableExport{
		{Path: "/api/v1/export.json", File: "math.json", Kind: core.KindPocket, Title: "Your notebooks with every cell, your own calculations, the answers you kept and practice rounds"},
		{Path: "/api/v1/export.csv", File: "answers.csv", Kind: core.KindCSV, Title: "The answers you kept, as a table"},
	},
	Imports: []core.PortableImport{
		{Path: "/api/v1/import", Kind: core.KindPocket, ContentType: "application/json", MaxBytes: 64 << 20, Title: "Adds the notebooks, calculations, kept answers and practice from another box; what you already have is left alone"},
	},
}

// capabilities are what Pocket Search, Pocket Day and Pocket Hub may ask
// Math (docs/capabilities.md in pocket-homelab-core); Register adds the
// pocket:* offers. The routes are in capabilities.go.
var capabilities = core.Capabilities{
	Search: &core.Searchable{What: "Your notebooks by their name and the sums in them, your own and the ready calculations, and the answers you kept; a sum you type is worked out",
		Kinds: []string{"notebook", "formula", "answer", "sum"}, Bangs: []string{"math", "calc", "sum"}},
	Day:    &core.DayFeed{Label: "Math", One: "sum", Many: "sums", What: "The notebooks you started or worked on that day, the answers you kept and the practice rounds done"},
	Widget: &core.Widget{Title: "Math", What: "The notebooks you worked on last, pinned ones first, and how today's practice is going"},
	Capture: &core.QuickCapture{Kinds: []core.CaptureKind{
		{Kind: "calculation", Label: "Calculation", What: "A sum worked out and kept in a new notebook called Quick sums"},
	}},
}

// linkRoutes is what each scope allows; everything else is refused.
var linkRoutes = []struct {
	methods string
	path    *regexp.Regexp
	scope   string
}{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(home|summary|search|notebooks|formulas|run|runs|practice|reference|rates|templates|convert|calculate|day/\d{4}-\d{2}-\d{2})$`), "math:read"},
	{"GET", regexp.MustCompile(`^/api/v1/notebooks/\d+(/export\.md)?$`), "math:read"},
	{"GET", regexp.MustCompile(`^/api/v1/formulas/\d+$`), "math:read"},
	{"GET", regexp.MustCompile(`^/api/v1/library/[a-z0-9_]+$`), "math:read"},
	{"POST", regexp.MustCompile(`^/api/v1/calculate$`), "math:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(formulas/\d+|library/[a-z0-9_]+)/run$`), "math:read"},
	{"POST", regexp.MustCompile(`^/api/v1/notebooks$`), "math:write"},
	{"POST", regexp.MustCompile(`^/api/v1/notebooks/\d+/cells$`), "math:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/cells/\d+$`), "math:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/notebooks/\d+$`), "math:write"},
	{"POST", regexp.MustCompile(`^/api/v1/(formulas|runs)$`), "math:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/formulas/\d+$`), "math:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	if portable.LinkAllows(scopes, r) || capabilities.LinkAllows(scopes, r) {
		return true
	}
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.Audience != "math" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}

// withLinkApp marks a context as acting for another app (tests, and the auth middleware).
func withLinkApp(ctx context.Context, slug string) context.Context {
	return context.WithValue(ctx, linkAppKey, slug)
}
