package structpages

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
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

// guardRegions, isDocument and teeWriter are the docs' test guard
// ("Catching it: decide from the response"), verbatim.
func guardRegions(t testing.TB) MiddlewareFunc {
	return func(next http.Handler, _ *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := r.Header.Get("HX-Target")
			if r.Header.Get("HX-Request") != "true" || r.Header.Get("HX-Request-Type") == "full" ||
				target == "" || target == "body" {
				next.ServeHTTP(w, r)
				return
			}
			tw := &teeWriter{ResponseWriter: w}
			next.ServeHTTP(tw, r)
			if w.Header().Get("HX-Retarget") != "body" && isDocument(tw.body.String()) {
				t.Errorf("%s %s: a whole document answers an htmx request for %q", r.Method, r.URL, target)
			}
		})
	}
}

var (
	firstHead = regexp.MustCompile(`(?i)<head(\s[^>]*)?>[\s\S]*?</head>`)
	firstTag  = regexp.MustCompile(`(?i)<([a-z][^/>\x20\t\r\n\f]*)`)
)

func isDocument(body string) bool {
	if loc := firstHead.FindStringIndex(body); loc != nil {
		body = body[:loc[0]] + body[loc[1]:]
	}
	m := firstTag.FindStringSubmatch(body)
	return m != nil && (strings.EqualFold(m[1], "html") || strings.EqualFold(m[1], "body"))
}

type teeWriter struct {
	http.ResponseWriter
	body bytes.Buffer
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *teeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// recordingTB records the guard's failures instead of failing the test.
type recordingTB struct {
	testing.TB
	errs []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// exRows is a region page whose controls target it by selector
// (hx-target:inherited="closest tbody"): its Page is a fragment.
type exRows struct{}

func (exRows) Page() component { return testComponent{"<tbody><tr><td>row</td></tr></tbody>"} }

func TestRenderingExamples_guardRegions(t *testing.T) {
	type root struct {
		Dashboard exDashboard `route:"/dashboard Dashboard"`
		Report    exReport    `route:"/report Report"`
		Save      exSaveNote  `route:"POST /notes Save"`
		Rows      exRows      `route:"/rows Rows"`
	}
	rec := &recordingTB{TB: t}
	mux := http.NewServeMux()
	sp, err := Mount(mux, root{}, "/", "App", WithArgs(&exStore{}),
		WithTargetSelector(HTMXv4RenderTarget), WithMiddlewares(guardRegions(rec)))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := sp.ID(exDashboard.Notes)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := sp.ID(exReport.Summary)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, method, path, target, reqType string
		flagged                             bool
	}{
		{"link from another page's region gets the whole page", "GET", "/dashboard", "div#report-region", "partial", true},
		{"own region", "GET", "/dashboard", "ul#" + notes, "partial", false},
		{"borrowed id that Props handles with Is", "GET", "/dashboard", "div#" + summary, "partial", false},
		{"region control targeting by selector", "GET", "/rows", "tbody", "partial", false},
		{"action answering for another page's region", "POST", "/notes", "div#" + summary, "partial", false},
		{"boosted link to body", "GET", "/dashboard", "body", "full", false},
	}
	for _, c := range cases {
		rec.errs = nil
		req := httptest.NewRequest(c.method, c.path, http.NoBody)
		req.Header.Set("HX-Request", "true")
		req.Header.Set("HX-Target", c.target)
		req.Header.Set("HX-Request-Type", c.reqType)
		mux.ServeHTTP(httptest.NewRecorder(), req)
		if got := len(rec.errs) > 0; got != c.flagged {
			t.Errorf("%s: flagged = %v (%q), want %v", c.name, got, rec.errs, c.flagged)
		}
	}
}

func TestRenderingExamples_isDocument(t *testing.T) {
	for body, want := range map[string]bool{
		"<!doctype html><html><body>x</body></html>":  true,
		"<head><title>t</title></head><body>x</body>": true,
		"<tbody><tr><td>row</td></tr></tbody>":        false,
		"<div>fragment with <html> later</div>":       false,
		"<template><html></html></template>":          false,
		"":                                            false,
	} {
		if got := isDocument(body); got != want {
			t.Errorf("isDocument(%q) = %v, want %v", body, got, want)
		}
	}
}
