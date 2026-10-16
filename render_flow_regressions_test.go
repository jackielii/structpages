package structpages

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Regressions found reviewing the v0.9.0 render flow.

type twinPage struct{}

func (twinPage) Page() component    { return testComponent{"page"} }
func (twinPage) Content() component { return testComponent{"content"} }

// A type mounted twice: a request to one mount targeting the other's
// region isn't one of the served page's components, so Page renders.
func TestRegression_twinMountUnmatchedRendersPage(t *testing.T) {
	type root struct {
		A twinPage `route:"/a A"`
		B twinPage `route:"/b B"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithTargetSelector(HTMXRenderTarget)); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, htmxRequest("/a", "b-content")); body != "page" {
		t.Errorf("rendered %q, want page", body)
	}
	if _, body := serve(t, mux, htmxRequest("/a", "a-content")); body != "content" {
		t.Errorf("own region rendered %q, want content", body)
	}
}

// writerPage's component asks for an io.Writer: components get the
// response writer only for a parameter of exactly http.ResponseWriter, so
// the registered buffer fills it, as before v0.9.0.
type writerComponentPage struct{}

func (writerComponentPage) Page(out io.Writer) component {
	_, isBuffer := out.(*bytes.Buffer)
	return testComponent{fmt.Sprint(isBuffer)}
}

type exactWriterPage struct{}

func (exactWriterPage) Page(w http.ResponseWriter, r *http.Request, t RenderTarget) component {
	return testComponent{fmt.Sprint(w != nil, r != nil, t != nil)}
}

func TestRegression_componentsGetRequestValuesByExactTypeOnly(t *testing.T) {
	type root struct {
		W writerComponentPage `route:"/w W"`
		E exactWriterPage     `route:"/e E"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App", WithArgs(&bytes.Buffer{})); err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/w", http.NoBody)); body != "true" {
		t.Errorf("io.Writer param got the buffer = %s, want true", body)
	}
	if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/e", http.NoBody)); body != "true true true" {
		t.Errorf("exact params = %s, want all filled", body)
	}
}

// anyParamPage's Page takes `any`: it gets Props' first result, every
// time, not whichever candidate a map iteration happens to offer.
type anyParamPage struct{}

func (anyParamPage) Props() (int, error) { return 42, nil }
func (anyParamPage) Page(v any) component {
	return testComponent{fmt.Sprintf("%T", v)}
}

func TestRegression_fillOrderIsDeterministic(t *testing.T) {
	type root struct {
		P anyParamPage `route:"/p P"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	for range 50 {
		if _, body := serve(t, mux, httptest.NewRequest(http.MethodGet, "/p", http.NoBody)); body != "int" {
			t.Fatalf("any param got %s, want Props' int", body)
		}
	}
}

// listPage's LoadMore has id "list-load-more", which dashPage's
// EventListLoadMore would match loosely by suffix. The exact id names
// listPage's component, so dashPage doesn't claim it.
type listPage struct{}

func (listPage) Page() component     { return testComponent{"list"} }
func (listPage) LoadMore() component { return testComponent{"list-load-more"} }

type dashPage struct{}

func (dashPage) Props(t RenderTarget) (string, error) {
	if t.Is(listPage.LoadMore) {
		return "", RenderComponent(testComponent{"recognised list.LoadMore"})
	}
	return "", nil
}
func (dashPage) Page(string) component { return testComponent{"dash"} }
func (dashPage) EventListLoadMore(string) component {
	return testComponent{"dash-event-list-load-more"}
}

func TestRegression_exactIDOfAnotherPageBeatsLooseMatch(t *testing.T) {
	type root struct {
		List listPage `route:"/list List"`
		Dash dashPage `route:"/dash Dash"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, root{}, "/", "App", WithTargetSelector(HTMXRenderTarget))
	if err != nil {
		t.Fatal(err)
	}
	id, err := sp.ID(listPage.LoadMore)
	if err != nil {
		t.Fatal(err)
	}
	if _, body := serve(t, mux, htmxRequest("/dash", id)); body != "recognised list.LoadMore" {
		t.Errorf("rendered %q, want Props to recognise list's LoadMore", body)
	}
}

// Request-arg signals reach extended ServeHTTP in both of its forms.
type raVoidHandler struct{}

func (raVoidHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request, u *raUser) {
	_, err := io.WriteString(w, "handler:"+u.name)
	if err != nil {
		panic(err)
	}
}

func TestRegression_requestArgSignalsInExtendedServeHTTP(t *testing.T) {
	type root struct {
		V raVoidHandler `route:"/v V"`
		E raHandler     `route:"/e E"`
	}
	signin := func(*http.Request) (*raUser, error) {
		return nil, RenderComponent(testComponent{"sign in"})
	}
	mux := http.NewServeMux()
	var handled error
	_, err := Mount(mux, root{}, "/", "App", WithRequestArgs(signin),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) { handled = err }))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v", "/e"} {
		handled = nil
		if _, body := serve(t, mux, raGet(path, "")); body != "sign in" || handled != nil {
			t.Errorf("%s: rendered %q, error handler got %v; want the provider's RenderComponent", path, body, handled)
		}
	}
}

// genericPage is a generic page type. Outside generic code, a bound
// method value of it ("genericPage[...].Notes-fm") identifies its
// component. (Inside the type's own methods, Go compiles method values to
// anonymous closures, so Is can't identify them there; the docs say so.)
type genericPage[T any] struct{}

func (genericPage[T]) Page() component  { return testComponent{"page"} }
func (genericPage[T]) Notes() component { return testComponent{"notes"} }

func TestRegression_isWithBoundValueOfGenericPage(t *testing.T) {
	type root struct {
		G genericPage[int] `route:"/g G"`
	}
	sp, err := Mount(http.NewServeMux(), root{}, "/", "App")
	if err != nil {
		t.Fatal(err)
	}
	pn := sp.pc.root.Children[0]
	notes := pn.Components["Notes"]
	target := newMethodRenderTarget("Notes", &notes)
	if !target.Is(genericPage[int]{}.Notes) {
		t.Error("Is(bound value of a generic page's component) = false, want true")
	}
	if target.Is(genericPage[int]{}.Page) {
		t.Error("Is(another component) = true, want false")
	}
}

// jsonPage sets its own Content-Type; rendering keeps it.
type jsonPage struct{}

func (jsonPage) Props(w http.ResponseWriter) (string, error) {
	w.Header().Set("Content-Type", "application/json")
	return "", nil
}
func (jsonPage) Page(string) component { return testComponent{`{"a":1}`} }

func TestRegression_renderKeepsContentType(t *testing.T) {
	type root struct {
		J jsonPage `route:"/j J"`
	}
	mux := http.NewServeMux()
	if _, err := Mount(mux, root{}, "/", "App"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/j", http.NoBody))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestRegression_extendedServeHTTPSelectorError(t *testing.T) {
	type root struct {
		E raHandler `route:"/e E"`
	}
	boom := errors.New("selector failed")
	mux := http.NewServeMux()
	var got error
	_, err := Mount(mux, root{}, "/", "App", WithRequestArgs(raCurrentUser),
		WithTargetSelector(func(*http.Request, *PageNode) (RenderTarget, error) { return nil, boom }),
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			got = err
			w.WriteHeader(http.StatusInternalServerError)
		}))
	if err != nil {
		t.Fatal(err)
	}
	serve(t, mux, raGet("/e", "ana"))
	if !errors.Is(got, boom) {
		t.Errorf("error handler got %v, want the selector's error", got)
	}
}
