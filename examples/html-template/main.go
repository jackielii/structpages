// Demonstrates structpages with the standard library's html/template,
// organized in an atomic-design layout (layout / ui atoms / ui molecules /
// feature partials / pages).
//
// structpages is render-engine agnostic: a Page() method can return any
// value with a Render(ctx context.Context, w io.Writer) error method.
// The `tpl` type here is a thin wrapper around an html/template set; two
// small template funcs (urlFor and args) live right beside it. They take
// ctx as their first argument so the FuncMap registers ONCE at parse
// time and never needs Clone-rebinding per request.
//
// The convention this example uses for exposing ctx in templates is a
// `view` struct ({Ctx, Data}) passed as the template dot. Templates call
// `{{ urlFor .Ctx "x" }}` and read page state via `.Data.X`. Pick whatever
// shape suits you — there is no library-imposed wrapper.
package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

//go:embed templates
var tmplFS embed.FS

// pageTmpls holds one fully-parsed template set per page. Each set has its
// own "body" definition (page-specific) plus shared layout / ui / feature
// partials. We never Clone at render time — the template funcs take ctx
// as their first argument, so the request context flows through the
// template data instead of mutating funcs.
var pageTmpls = map[string]*template.Template{
	"index":   parseSet("layout/public.html", "pages/index.html"),
	"product": parseSet("layout/public.html", "pages/product.html"),
	"team":    parseSet("layout/public.html", "pages/team.html"),
	"contact": parseSet("layout/public.html", "pages/contact.html"),
	"post":    parseSet("layout/public.html", "post/page.html"),
}

func parseSet(layout, body string) *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"urlFor": urlFor,
		"args":   args,
	})
	patterns := []string{
		"templates/" + layout,
		"templates/ui/atoms/*.html",
		"templates/ui/molecules/*.html",
		"templates/post/*.html",
		"templates/" + body,
	}
	return template.Must(t.ParseFS(tmplFS, patterns...))
}

// urlFor is a tiny adapter so templates can call structpages.URLFor with
// a string page reference: `{{ urlFor .Ctx "Product" }}`.
func urlFor(ctx context.Context, name string, a ...any) (string, error) {
	return structpages.URLFor(ctx, structpages.Ref(name), a...)
}

// args builds a map[string]any from alternating key/value pairs, used to
// pass multiple inputs to a partial template:
//
//	{{ template "ui/molecules/card" (args "Title" .Data.Title "Body" .Data.Body) }}
func args(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("args: odd number of arguments (%d)", len(kv))
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("args: key at position %d is %T, expected string", i, kv[i])
		}
		m[k] = kv[i+1]
	}
	return m, nil
}

// view is the example's chosen template-dot shape: ctx + page data.
// urlFor reads ctx via .Ctx; templates read page state via .Data. There
// is no library-imposed wrapper — pick your own and pass ctx however
// suits.
type view struct {
	Ctx  context.Context //nolint:containedctx
	Data any
}

// tpl is a renderable component backed by one of the parsed page sets.
// `entry` selects which named template to execute — "layout/public" for
// the full page, "body" for the HTMX content swap, or e.g.
// "post/comments-list" for an organism partial.
type tpl struct {
	page  string
	entry string
	data  any
}

func (p tpl) Render(ctx context.Context, w io.Writer) error {
	t, ok := pageTmpls[p.page]
	if !ok {
		return fmt.Errorf("unknown page %q", p.page)
	}
	return t.ExecuteTemplate(w, p.entry, view{Ctx: ctx, Data: p.data})
}

// --- simple pages: index / product / team / contact ---

type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
	post    `route:"/post Post"`
}

func (index) Page() tpl { return tpl{page: "index", entry: "layout/public"} }
func (index) Main() tpl { return tpl{page: "index", entry: "body"} }

type product struct{}

func (product) Page() tpl { return tpl{page: "product", entry: "layout/public"} }
func (product) Main() tpl { return tpl{page: "product", entry: "body"} }

type team struct{}

func (team) Page() tpl { return tpl{page: "team", entry: "layout/public"} }
func (team) Main() tpl { return tpl{page: "team", entry: "body"} }

type contact struct{}

func (contact) Page() tpl { return tpl{page: "contact", entry: "layout/public"} }
func (contact) Main() tpl { return tpl{page: "contact", entry: "body"} }

// --- post page: demonstrates atom + molecule + organism composition ---
//
// Data loading lives in a single Props method; component methods receive
// the loaded props as a parameter. structpages calls Props once per
// request before dispatching to the matched component, so Comments() and
// Main() and Page() all see the same props without re-loading.

type postProps struct {
	Title    string
	Body     string
	Recent   []postSummary
	Comments []string
}

type postSummary struct {
	Title   string
	Excerpt string
}

type post struct{}

// Props is the single place data is loaded for the post page. structpages
// calls it before any component method on this page; the return value is
// then passed as an argument to whichever method is dispatched (Page,
// Main, or Comments).
//
// In a real app this would query a store and could optionally take a
// structpages.RenderTarget parameter to skip work when only a partial is
// being rendered (see examples/blog for that pattern).
func (post) Props() postProps {
	return postProps{
		Title: "Hello, atomic design",
		Body:  "This page composes a layout, two molecule cards, and a comments organism.",
		Recent: []postSummary{
			{Title: "Why structpages", Excerpt: "Struct-tag routing for Go."},
			{Title: "html/template tips", Excerpt: "Per-page parsed sets avoid name collisions."},
		},
		Comments: []string{"First!", "Nice example.", "Going to try this."},
	}
}

func (post) Page(props postProps) tpl {
	return tpl{page: "post", entry: "layout/public", data: props}
}

// Main renders just the page body — used for HTMX nav swaps targeting <main>.
func (post) Main(props postProps) tpl {
	return tpl{page: "post", entry: "body", data: props}
}

// Comments renders just the comments organism. structpages's
// HTMXv4RenderTarget routes HX-Target=section#comments (or tag-only
// "comments") here by matching the method name to the kebab-cased
// component id. The partial template is wrapped in <section id="comments">
// so it targets itself for subsequent swaps.
//
// Comments only needs the comments slice — Props loads everything, but a
// real Props could check the RenderTarget and skip the rest when only
// this organism is being rendered.
func (post) Comments(props postProps) tpl {
	return tpl{page: "post", entry: "post/comments-list", data: props.Comments}
}

func main() {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, index{}, "/", "Index",
		structpages.WithTargetSelector(structpages.HTMXv4RenderTarget),
	); err != nil {
		log.Fatalf("mount: %v", err)
	}
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
