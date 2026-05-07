package htmltemplate_test

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackielii/structpages"
	"github.com/jackielii/structpages/htmltemplate"
)

// --- shared mount fixtures ---

type productPage struct{}
type userPage struct{}

type indexPage struct {
	Product productPage `route:"/products Product"`
	User    userPage    `route:"/users/{id} User"`
}

// view is the example data shape — a {Ctx, Data} struct passed as the
// template dot. The package itself does not provide this; users pick
// their own convention. We define one here to drive the tests.
type view struct {
	Ctx  context.Context //nolint:containedctx
	Data any
}

// --- Funcs / template integration (parsed once, no Clone) ---

// indexTmpl is parsed once at package init and executed concurrently across
// requests with no Clone — Funcs registers ctx-first helpers, and ctx
// flows through `view.Ctx`.
var indexTmpl = template.Must(template.New("t").
	Funcs(htmltemplate.Funcs()).
	Parse(
		`product={{ urlFor .Ctx "Product" }}|` +
			`user={{ urlFor .Ctx "User" "id" 42 }}|` +
			`route={{ urlFor .Ctx "/products" }}|` +
			`{{ template "card" (args "Title" "Hi" "Body" .Data) }}` +
			`{{ define "card" }}<{{ .Title }}|{{ .Body }}>{{ end }}`,
	))

type tpl struct{ data any }

func (p tpl) Render(ctx context.Context, w io.Writer) error {
	return indexTmpl.Execute(w, view{Ctx: ctx, Data: p.data})
}

func (indexPage) Page() tpl { return tpl{data: "world"} }

func TestFuncs_NoCloneAcrossRequests(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, indexPage{}, "/", "Index",
		structpages.WithWarnEmptyRoute(func(*structpages.PageNode) {})); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rr.Code, rr.Body.String())
	}
	// html/template escapes `<` (and `&`) in text context but leaves `>` alone.
	want := `product=/products|user=/users/42|route=/products|&lt;Hi|world>`
	if got := strings.TrimSpace(rr.Body.String()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- standalone helpers ---

// captureCtxPage is a page whose Page() method captures the request ctx, so
// tests can exercise htmltemplate.URLFor / ID / IDTarget against a real
// structpages-bearing context.
type captureCtxPage struct {
	Product productPage `route:"/products Product"`
}

var capturedCtx context.Context //nolint:gochecknoglobals,containedctx

type captureRender struct{}

func (captureRender) Render(ctx context.Context, _ io.Writer) error {
	capturedCtx = ctx
	return nil
}

func (captureCtxPage) Page() captureRender { return captureRender{} }

func TestStandaloneHelpers_RealCtx(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, captureCtxPage{}, "/", "Capture",
		structpages.WithWarnEmptyRoute(func(*structpages.PageNode) {})); err != nil {
		t.Fatal(err)
	}
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if capturedCtx == nil {
		t.Fatal("capturedCtx not populated; did the page handler run?")
	}

	if got, err := htmltemplate.URLFor(capturedCtx, "Product"); err != nil || got != "/products" {
		t.Errorf("URLFor: got %q %v, want /products", got, err)
	}
}

// --- Args ---

func TestArgs_HappyPath(t *testing.T) {
	got, err := htmltemplate.Args("Title", "Hello", "Count", 3)
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	if got["Title"] != "Hello" || got["Count"] != 3 {
		t.Errorf("got %#v", got)
	}
}

func TestArgs_OddCount(t *testing.T) {
	if _, err := htmltemplate.Args("k1", "v1", "k2"); err == nil {
		t.Errorf("expected error for odd argument count")
	}
}

func TestArgs_NonStringKey(t *testing.T) {
	if _, err := htmltemplate.Args("ok", 1, 42, "v2"); err == nil {
		t.Errorf("expected error for non-string key")
	}
}

func TestArgs_Empty(t *testing.T) {
	got, err := htmltemplate.Args()
	if err != nil {
		t.Fatalf("Args(): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %#v", got)
	}
}
