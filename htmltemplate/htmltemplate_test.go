package htmltemplate_test

import (
	"bytes"
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

// pageData is the typed page data the View carries.
type pageData struct {
	Title string
	Card  struct {
		Heading string
		Body    string
	}
}

type indexPage struct {
	Product productPage `route:"/products Product"`
	User    userPage    `route:"/users/{id} User"`
}

// --- Funcs (legacy ctx-rebind path) ---

var funcsTmpl = template.Must(template.New("t").
	Funcs(htmltemplate.Funcs(nil)).
	Parse(`{{ urlFor "Product" }}|{{ urlFor "User" "id" 42 }}|{{ urlFor "/products" }}`))

type funcsTpl struct{ name string }

func (p funcsTpl) Render(ctx context.Context, w io.Writer) error {
	clone, err := funcsTmpl.Clone()
	if err != nil {
		return err
	}
	clone.Funcs(htmltemplate.Funcs(ctx))
	return clone.ExecuteTemplate(w, p.name, nil)
}

func (indexPage) Page() funcsTpl { return funcsTpl{name: "t"} }

func TestFuncs_NilCtx_ReturnsEmpty(t *testing.T) {
	tmpl := template.Must(template.New("nilctx").
		Funcs(htmltemplate.Funcs(nil)).
		Parse(`url={{ urlFor "X" }} id={{ idFor "Y" }} target={{ idTarget "Z" }}`))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatalf("execute with nil ctx: %v", err)
	}
	if !strings.Contains(buf.String(), "url= id= target=") {
		t.Errorf("expected empty values, got %q", buf.String())
	}
}

func TestFuncs_BoundCtx_ResolvesURLs(t *testing.T) {
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
	want := "/products|/users/42|/products"
	if got := strings.TrimSpace(rr.Body.String()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- View (recommended path; no Clone) ---

// viewTmpl is parsed once and executed concurrently across requests with no
// Clone. URL / ID / Target come through the View dot, not a FuncMap.
//
// Two partial conventions are exercised:
//   - "card" is an atom/molecule — receives an ad-hoc map via args, accesses
//     fields directly (.Heading, .Body). No framework helpers visible.
//   - "summary" is an organism — receives a re-wrapped View via .Sub,
//     accesses typed data via .Data and uses .URL.
var viewTmpl = template.Must(template.New("v").
	Funcs(template.FuncMap{"args": htmltemplate.Args}).
	Parse(`title={{ .Data.Title }}|url={{ .URL "Product" }}|user={{ .URL "User" "id" 42 }}|` +
		`{{ template "summary" (.Sub .Data.Card) }}|` +
		`{{ template "card" (args "Heading" "From args" "Body" "Inline") }}` +
		`{{ define "card" }}({{ .Heading }}|{{ .Body }}){{ end }}` +
		`{{ define "summary" }}[{{ .Data.Heading }}@{{ .URL "Product" }}]{{ end }}`))

type viewTpl struct{ data pageData }

func (p viewTpl) Render(ctx context.Context, w io.Writer) error {
	return viewTmpl.Execute(w, htmltemplate.NewView(ctx, p.data))
}

// viewIndexPage is a separate root so we don't collide with funcsTpl's Page method.
type viewIndexPage struct {
	Product productPage `route:"/products Product"`
	User    userPage    `route:"/users/{id} User"`
}

func (viewIndexPage) Page() viewTpl {
	d := pageData{Title: "Hello"}
	d.Card.Heading = "From .Sub"
	d.Card.Body = "Body"
	return viewTpl{data: d}
}

func TestView_NoClone_ResolvesURLsAndPartials(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, viewIndexPage{}, "/", "Index",
		structpages.WithWarnEmptyRoute(func(*structpages.PageNode) {})); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	for _, want := range []string{
		"title=Hello",
		"url=/products",
		"user=/users/42",
		"[From .Sub@/products]", // .Sub re-wraps; organism partial sees .Data + keeps .URL
		"(From args|Inline)",    // args helper builds a flat map; molecule partial accesses fields directly
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in body:\n%s", want, body)
		}
	}
}

func TestView_NilCtx_ReturnsEmpty(t *testing.T) {
	v := htmltemplate.NewView[any](nil, nil)
	if got, err := v.URL("X"); err != nil || got != "" {
		t.Errorf("URL nil ctx: %q %v", got, err)
	}
	if got, err := v.ID("X"); err != nil || got != "" {
		t.Errorf("ID nil ctx: %q %v", got, err)
	}
	if got, err := v.Target("X"); err != nil || got != "" {
		t.Errorf("Target nil ctx: %q %v", got, err)
	}
}

func TestView_Sub_PreservesCtx_ReplacesData(t *testing.T) {
	type ck struct{}
	ctx := context.WithValue(context.Background(), ck{}, "marker")

	v := htmltemplate.NewView(ctx, pageData{Title: "outer"})
	sub := v.Sub("inner-data")

	if sub.Ctx.Value(ck{}) != "marker" {
		t.Errorf("Sub did not preserve ctx")
	}
	if sub.Data != "inner-data" {
		t.Errorf("Sub data = %v, want inner-data", sub.Data)
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
