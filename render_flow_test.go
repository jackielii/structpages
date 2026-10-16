package structpages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests pin the v0.9.0 render flow: the selector says what the request
// wants, Props is the normal flow and an error breaks it, and with no error
// the component the target Is renders, else Page.

type flowNotes struct{ n int }
type flowStats struct{ s string }

type flowPage struct{}

func (flowPage) Props(t RenderTarget) (flowStats, flowNotes, error) {
	return flowStats{"stats"}, flowNotes{2}, nil
}
func (flowPage) Page(s flowStats, n flowNotes) component {
	return testComponent{"page:" + s.s + ":" + strings.Repeat("n", n.n)}
}
func (flowPage) Notes(n flowNotes) component {
	return testComponent{"notes:" + strings.Repeat("n", n.n)}
}
func (flowPage) Stats(s flowStats) component { return testComponent{"stats:" + s.s} }

type flowRoot struct {
	Flow flowPage `route:"/flow Flow"`
}

func serve(t *testing.T, mux *http.ServeMux, req *http.Request) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func htmxRequest(path, target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", target)
	return req
}

func TestFlow_noSelectorRendersPageForHTMXRequests(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, flowRoot{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, htmxRequest("/flow", "flow-notes")); body != "page:stats:nn" {
		t.Errorf("without a selector, an htmx partial request got %q, want the Page", body)
	}
}

func TestFlow_componentsTakeTheirPropsByType(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Mount(mux, flowRoot{}, "/", "App", WithTargetSelector(HTMXRenderTarget)); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"":             "page:stats:nn",
		"flow-notes":   "notes:nn",
		"flow-stats":   "stats:stats",
		"nothing-here": "page:stats:nn",
	} {
		req := httptest.NewRequest(http.MethodGet, "/flow", http.NoBody)
		if target != "" {
			req = htmxRequest("/flow", target)
		}
		if _, body := serve(t, mux, req); body != want {
			t.Errorf("HX-Target %q rendered %q, want %q", target, body, want)
		}
	}
}

// queryTarget is a custom RenderTarget: the request wants the component
// named in ?partial=.
type queryTarget string

func (q queryTarget) Is(c any) bool {
	info, err := extractMethodInfo(c)
	return err == nil && !info.isFunction && info.methodName == string(q)
}

func TestFlow_customTargetsTakePartInSelection(t *testing.T) {
	mux := http.NewServeMux()
	sel := func(r *http.Request, _ *PageNode) (RenderTarget, error) {
		return queryTarget(r.URL.Query().Get("partial")), nil
	}
	if _, err := Mount(mux, flowRoot{}, "/", "App", WithTargetSelector(sel)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/flow?partial=Stats", http.NoBody)
	if _, body := serve(t, mux, req); body != "stats:stats" {
		t.Errorf("?partial=Stats rendered %q", body)
	}
	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/flow", http.NoBody)); body != "page:stats:nn" {
		t.Errorf("no partial rendered %q, want Page", body)
	}
}

// greedyTarget Is everything, so selection can't pick one.
type greedyTarget struct{}

func (greedyTarget) Is(any) bool { return true }

func TestFlow_targetThatIsSeveralComponentsIsAnError(t *testing.T) {
	mux := http.NewServeMux()
	var got error
	_, err := Mount(mux, flowRoot{}, "/", "App",
		WithTargetSelector(func(*http.Request, *PageNode) (RenderTarget, error) { return greedyTarget{}, nil }),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			got = err
			w.WriteHeader(http.StatusInternalServerError)
		}))
	if err != nil {
		t.Fatal(err)
	}
	serve(t, mux, httptest.NewRequest(http.MethodGet, "/flow", http.NoBody))
	if got == nil || !strings.Contains(got.Error(), "Is more than one component (Notes, Page, Stats)") {
		t.Errorf("error = %v, want one naming every component the target Is", got)
	}
}

// recognisingPage's Props asks about a standalone function and then returns
// no error: that's normal flow, so Page renders. Is records nothing.
type recognisingPage struct{}

func (recognisingPage) Props(t RenderTarget) (string, error) {
	if t.Is(StandaloneWidget) {
		return "recognised", nil
	}
	return "other", nil
}
func (recognisingPage) Page(s string) component { return testComponent{"page:" + s} }

func TestFlow_isHasNoSideEffect(t *testing.T) {
	type root struct {
		P recognisingPage `route:"/r R"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithTargetSelector(HTMXRenderTarget)); err != nil {
		t.Fatal(err)
	}
	code, body := serve(t, mux, htmxRequest("/r", "structpages-standalone-widget"))
	if code != http.StatusOK || body != "page:recognised" {
		t.Errorf("got %d %q, want 200 %q", code, body, "page:recognised")
	}
}

// sharedRegionPage renders flowPage's Notes region when a request targets
// it: Is(OtherPage.Method) matches another page's component by exact id.
type sharedRegionPage struct{}

func (p sharedRegionPage) Props(t RenderTarget) (string, error) {
	if t.Is(flowPage.Notes) {
		return "", RenderComponent(flowPage{}.Notes(flowNotes{1}))
	}
	return "own", nil
}
func (sharedRegionPage) Page(s string) component { return testComponent{"shared-page:" + s} }

func TestFlow_isMatchesAnotherPagesComponentByExactID(t *testing.T) {
	type root struct {
		Flow   flowPage         `route:"/flow Flow"`
		Shared sharedRegionPage `route:"/shared Shared"`
	}
	// htmx 1/2 send the bare id; htmx 4 sends "<tag>#<id>".
	for prefix, sel := range map[string]TargetSelector{"": HTMXRenderTarget, "div#": HTMXv4RenderTarget} {
		mux := http.NewServeMux()
		sp, err := Mount(mux, root{}, "/", "App", WithTargetSelector(sel))
		if err != nil {
			t.Fatal(err)
		}
		id, err := sp.ID(flowPage.Notes)
		if err != nil {
			t.Fatal(err)
		}
		if _, body := serve(t, mux, htmxRequest("/shared", prefix+id)); body != "notes:n" {
			t.Errorf("target %q rendered %q, want flowPage's Notes", id, body)
		}
		// Only the exact id: the bare component name doesn't name another
		// page's component.
		if _, body := serve(t, mux, htmxRequest("/shared", prefix+"notes")); body != "shared-page:own" {
			t.Errorf("bare target rendered %q, want Page", body)
		}
	}
}

type severalPage struct{}

func (p severalPage) Props() (string, error) {
	return "", RenderComponent(testComponent{"<main>"}, testComponent{"<hx-partial>"}, testComponent{"</hx-partial>"})
}
func (severalPage) Page(string) component { return testComponent{"page"} }

type nilComponentPage struct{}

func (nilComponentPage) Props() (string, error) {
	return "", RenderComponent(testComponent{"a"}, nil)
}
func (nilComponentPage) Page(string) component { return testComponent{"page"} }

func TestFlow_renderComponentRendersSeveralInOrder(t *testing.T) {
	type root struct {
		Several severalPage      `route:"/several Several"`
		Nil     nilComponentPage `route:"/nil Nil"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/several", http.NoBody)
	if _, body := serve(t, mux, req); body != "<main><hx-partial></hx-partial>" {
		t.Errorf("rendered %q", body)
	}
	req = httptest.NewRequest(http.MethodGet, "/nil", http.NoBody)
	if code, _ := serve(t, mux, req); code != http.StatusInternalServerError {
		t.Errorf("a nil component gave %d, want the error handler's 500", code)
	}
}

// componentAmbiguityPage's Content asks for a namer, which two
// registrations implement and Props doesn't return.
type componentAmbiguityPage struct{}

func (componentAmbiguityPage) Props() (string, error)    { return "", nil }
func (componentAmbiguityPage) Page(string) component     { return testComponent{"page"} }
func (componentAmbiguityPage) Content(n namer) component { return testComponent{n.Name()} }

// componentFromPropsPage's Content gets its namer from Props, so the two
// registrations don't make it ambiguous.
type componentFromPropsPage struct{}

func (componentFromPropsPage) Props() (namer, error)     { return &pgStore{name: "props"}, nil }
func (componentFromPropsPage) Page(n namer) component    { return testComponent{n.Name()} }
func (componentFromPropsPage) Content(n namer) component { return testComponent{n.Name()} }

func TestFlow_mountChecksComponentParameters(t *testing.T) {
	args := WithArgs(&pgStore{name: "pg"}, &memCache{})
	type ambiguous struct {
		P componentAmbiguityPage `route:"/a A"`
	}
	_, err := Mount(http.NewServeMux(), ambiguous{}, "/", "App", args)
	if err == nil || !strings.Contains(err.Error(), "Content") ||
		!strings.Contains(err.Error(), "implemented by more than one registered value") {
		t.Errorf("Mount error = %v, want Content's namer reported as ambiguous", err)
	}

	type fromProps struct {
		P componentFromPropsPage `route:"/p P"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, fromProps{}, "/", "App", args); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/p", http.NoBody)); body != "props" {
		t.Errorf("rendered %q, want Props' namer", body)
	}
}

// requestPathPage's Page takes the request, which components can now get.
type requestPathPage struct{}

func (requestPathPage) Page(r *http.Request) component { return testComponent{r.URL.Path} }

func TestFlow_componentsGetRequestValues(t *testing.T) {
	type root struct {
		P requestPathPage `route:"/path Path"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/path", http.NoBody)); body != "/path" {
		t.Errorf("rendered %q", body)
	}
}

func TestFlow_propsErrorsStillBreakTheFlow(t *testing.T) {
	type root struct {
		P skipPage `route:"/skip Skip"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/skip", http.NoBody))
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Body.String(), "page") {
		t.Errorf("got %d %q, want the redirect Props wrote and nothing rendered", rec.Code, rec.Body.String())
	}
}

type skipPage struct{}

func (skipPage) Props(w http.ResponseWriter, r *http.Request) (string, error) {
	http.Redirect(w, r, "/elsewhere", http.StatusSeeOther)
	return "", ErrSkipPageRender
}
func (skipPage) Page(string) component { return testComponent{"page"} }
