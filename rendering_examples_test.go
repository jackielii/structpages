package structpages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// These tests pin the worked examples in docs/rendering.md: one dashboard
// page, traced through the requests in its table.

type exStats struct{ n int }
type exNote string

type exStore struct{}

func (exStore) Stats() exStats  { return exStats{3} }
func (exStore) Notes() []exNote { return []exNote{"a", "b"} }
func (exStore) Summary() string { return "summary" }

// ExStatsWidget is a standalone component, like a ui.StatsWidget.
func ExStatsWidget(s exStats) component { return testComponent{"widget:" + strings.Repeat("s", s.n)} }

type exReport struct{}

func (exReport) Page(s string) component    { return testComponent{"report-page"} }
func (exReport) Summary(s string) component { return testComponent{"report-summary:" + s} }

type exDashboard struct{}

func (p exDashboard) Props(t RenderTarget, store *exStore) (exStats, []exNote, error) {
	switch {
	case t.Is(p.Notes):
		return exStats{}, store.Notes(), nil
	case t.Is(ExStatsWidget):
		return exStats{}, nil, RenderComponent(ExStatsWidget(store.Stats()))
	case t.Is(exReport.Summary):
		return exStats{}, nil, RenderComponent(exReport{}.Summary(store.Summary()))
	}
	return store.Stats(), store.Notes(), nil
}

func (exDashboard) Page(s exStats, notes []exNote) component {
	return testComponent{"<html>page " + exJoin(s, notes) + "</html>"}
}
func (exDashboard) Content(s exStats, notes []exNote) component {
	return testComponent{"content " + exJoin(s, notes)}
}
func (exDashboard) Notes(notes []exNote) component {
	return testComponent{"notes " + exJoin(exStats{}, notes)}
}

func exJoin(s exStats, notes []exNote) string {
	parts := []string{strings.Repeat("s", s.n)}
	for _, n := range notes {
		parts = append(parts, string(n))
	}
	return strings.Join(parts, ",")
}

type exRoot struct {
	Dashboard exDashboard `route:"/dashboard Dashboard"`
	Report    exReport    `route:"/report Report"`
	Save      exSaveNote  `route:"POST /notes Save"`
}

// exSaveNote is an action handler: a boosted form inside a region posts
// here and gets the region's new content back.
type exSaveNote struct{}

func (exSaveNote) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return RenderComponent(testComponent{"saved"})
}

func TestRenderingExamples_table(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, exRoot{}, "/", "App",
		WithTargetSelector(HTMXv4RenderTarget), WithArgs(&exStore{}))
	if err != nil {
		t.Fatal(err)
	}
	id := func(v any) string {
		s, err := sp.ID(v)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	type row struct {
		name    string
		headers map[string]string
		want    string
	}
	partial := func(target string) map[string]string {
		return map[string]string{"HX-Request": "true", "HX-Request-Type": "partial", "HX-Target": target}
	}
	rows := []row{
		{"full load", nil, "<html>page sss,a,b</html>"},
		{"boosted link", map[string]string{"HX-Request": "true", "HX-Request-Type": "full", "HX-Target": "body"},
			"<html>page sss,a,b</html>"},
		{"Content's id", partial("main#" + id(exDashboard.Content)), "content sss,a,b"},
		{"Notes' id", partial("ul#" + id(exDashboard.Notes)), "notes ,a,b"},
		{"standalone widget's id", partial("div#" + id(ExStatsWidget)), "widget:sss"},
		{"another page's region", partial("div#" + id(exReport.Summary)), "report-summary:summary"},
		{"an id nobody owns", partial("div#sidebar"), "<html>page sss,a,b</html>"},
	}
	for _, r := range rows {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
		for k, v := range r.headers {
			req.Header.Set(k, v)
		}
		if _, body := serve(t, mux, req); body != r.want {
			t.Errorf("%s: rendered %q, want %q", r.name, body, r.want)
		}
	}
}

// exRecognisingPage recognises the widget but returns no error: normal
// flow, so Page renders, not the widget.
type exRecognisingPage struct{}

func (exRecognisingPage) Props(t RenderTarget, store *exStore) (exStats, error) {
	if t.Is(ExStatsWidget) {
		return store.Stats(), nil
	}
	return exStats{}, nil
}
func (exRecognisingPage) Page(s exStats) component { return testComponent{"page " + exJoin(s, nil)} }

// exPropsOnly has no Page, so a request that wants none of its components
// needs Props to choose.
type exPropsOnly struct{}

func (exPropsOnly) Props(r *http.Request) (string, error) {
	if r.URL.Query().Has("table") {
		return "", RenderComponent(testComponent{"table"})
	}
	return "", nil
}
func (exPropsOnly) Card(s string) component { return testComponent{"card"} }

func TestRenderingExamples_subtleCases(t *testing.T) {
	type root struct {
		R exRecognisingPage `route:"/r R"`
		P exPropsOnly       `route:"/p P"`
	}
	mux := http.NewServeMux()
	var got error
	sp, err := Mount(mux, root{}, "/", "App",
		WithTargetSelector(HTMXv4RenderTarget), WithArgs(&exStore{}),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			got = err
			w.WriteHeader(http.StatusInternalServerError)
		}))
	if err != nil {
		t.Fatal(err)
	}
	widget, err := sp.ID(ExStatsWidget)
	if err != nil {
		t.Fatal(err)
	}
	req := htmxRequest("/r", "div#"+widget)
	if _, body := serve(t, mux, req); body != "page sss" {
		t.Errorf("recognised but not rendered: got %q, want Page with the widget's data", body)
	}

	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/p?table", http.NoBody)); body != "table" {
		t.Errorf("props-only with RenderComponent: got %q", body)
	}
	if code, _ := serve(t, mux, httptest.NewRequest(http.MethodGet, "/p", http.NoBody)); code != http.StatusInternalServerError ||
		got == nil || !strings.Contains(got.Error(), "has no Page component") {
		t.Errorf("props-only without a choice: got %d, %v", code, got)
	}
}

// exQueryTarget is the docs' custom selector: ?partial=<id> wants the
// component with that generated id.
type exQueryTarget struct {
	r  *http.Request
	id string
}

func (q exQueryTarget) Is(component any) bool {
	id, err := ID(q.r.Context(), component)
	return err == nil && id == q.id
}

func TestRenderingExamples_customSelector(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, exRoot{}, "/", "App", WithArgs(&exStore{}),
		WithTargetSelector(func(r *http.Request, _ *PageNode) (RenderTarget, error) {
			return exQueryTarget{r: r, id: r.URL.Query().Get("partial")}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := sp.ID(exDashboard.Notes)
	if err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{
		"?partial=" + notes: "notes ,a,b",
		"":                  "<html>page sss,a,b</html>",
	} {
		req := httptest.NewRequest(http.MethodGet, "/dashboard"+query, http.NoBody)
		if _, body := serve(t, mux, req); body != want {
			t.Errorf("%q rendered %q, want %q", query, body, want)
		}
	}
}

// exRetargetNavigation is the docs' middleware: a boosted request whose
// target is none of the served page's components is navigation from
// another page's region, so the whole page goes to body.
func exRetargetNavigation(next http.Handler, pn *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasPage := pn.Components["Page"]
		if r.Method == http.MethodGet && hasPage &&
			r.Header.Get("HX-Boosted") == "true" &&
			r.Header.Get("HX-Request-Type") == "partial" &&
			!exWantsOwnComponent(r, pn) {
			w.Header().Set("HX-Retarget", "body")
			w.Header().Set("HX-Reswap", "innerMorph")
		}
		next.ServeHTTP(w, r)
	})
}

// exWantsOwnComponent reports whether the request's target is one of pn's
// components other than Page.
func exWantsOwnComponent(r *http.Request, pn *PageNode) bool {
	t, err := HTMXv4RenderTarget(r, pn)
	if err != nil {
		return false
	}
	for name, m := range pn.Components {
		if name != "Page" && t.Is(m.Func.Interface()) {
			return true
		}
	}
	return false
}

func TestRenderingExamples_retargetNavigation(t *testing.T) {
	mux := http.NewServeMux()
	sp, err := Mount(mux, exRoot{}, "/", "App", WithArgs(&exStore{}),
		WithTargetSelector(HTMXv4RenderTarget), WithMiddlewares(exRetargetNavigation))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := sp.ID(exDashboard.Notes)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, method, path, target, reqType, boosted string
		retarget                                     bool
	}{
		{"boosted link from another page's region", "GET", "/dashboard", "div#report-region", "partial", "true", true},
		{"boosted link targeting this page's own region", "GET", "/dashboard", "ul#" + notes, "partial", "true", false},
		{"boosted link to body", "GET", "/dashboard", "body", "full", "true", false},
		{"region reload, not boosted", "GET", "/dashboard", "div#report-region", "partial", "", false},
		{"boosted form in a region posting to an action", "POST", "/notes", "div#report-region", "partial", "true", false},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, http.NoBody)
		req.Header.Set("HX-Request", "true")
		req.Header.Set("HX-Target", c.target)
		req.Header.Set("HX-Request-Type", c.reqType)
		if c.boosted != "" {
			req.Header.Set("HX-Boosted", c.boosted)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if got := rec.Header().Get("HX-Retarget") == "body"; got != c.retarget {
			t.Errorf("%s: retargeted = %v, want %v", c.name, got, c.retarget)
		}
	}
}

// exBorrowedRegions is the docs' development middleware: it reports a
// partial request that isn't navigation and targets another page's
// component, which the served page doesn't have.
func exBorrowedRegions(report func(served *PageNode, owner *PageNode, component string)) MiddlewareFunc {
	var (
		once   sync.Once
		owners map[string]componentOwner
	)
	return func(next http.Handler, pn *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			once.Do(func() { owners = exComponentOwners(r.Context(), pn) })
			key := r.Header.Get("HX-Target")
			if _, id, ok := strings.Cut(key, "#"); ok {
				key = id
			}
			if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
				if o, ok := owners[key]; ok && o.page != pn {
					report(pn, o.page, o.component)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type componentOwner struct {
	page      *PageNode
	component string
}

// exComponentOwners maps every component id in the tree to its page.
func exComponentOwners(ctx context.Context, pn *PageNode) map[string]componentOwner {
	root := pn
	for root.Parent != nil {
		root = root.Parent
	}
	owners := map[string]componentOwner{}
	for n := range root.All() {
		for name, m := range n.Components {
			if id, err := ID(ctx, m.Func.Interface()); err == nil {
				owners[id] = componentOwner{page: n, component: name}
			}
		}
	}
	return owners
}

func TestRenderingExamples_borrowedRegions(t *testing.T) {
	var reports []string
	mux := http.NewServeMux()
	sp, err := Mount(mux, exRoot{}, "/", "App", WithArgs(&exStore{}),
		WithTargetSelector(HTMXv4RenderTarget),
		WithMiddlewares(exBorrowedRegions(func(served, owner *PageNode, component string) {
			reports = append(reports, served.Name+" <- "+owner.Name+"."+component)
		})))
	if err != nil {
		t.Fatal(err)
	}
	reportContent, err := sp.ID(exReport.Page)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := sp.ID(exDashboard.Notes)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"div#" + reportContent, "ul#" + notes, "div#eos-bell-panel"} {
		serve(t, mux, htmxRequest("/dashboard", target))
	}
	if want := []string{"Dashboard <- Report.Page"}; strings.Join(reports, ";") != strings.Join(want, ";") {
		t.Errorf("reports = %q, want %q", reports, want)
	}
}
