// Demonstrates structpages with the standard library's html/template,
// organized in an atomic-design layout (layout / ui atoms / ui molecules /
// feature partials / pages).
//
// structpages is render-engine agnostic: a Page() method can return any
// value with a Render(ctx context.Context, w io.Writer) error method.
// The `tpl` type here is a thin wrapper around an html/template set, plus
// two small template funcs (urlFor and args) defined right beside it.
//
// urlFor needs the request context, so each Render Clones the base
// template, binds urlFor to the request ctx, and Executes. Clone is the
// price for being able to write `{{ urlFor "Product" }}` without
// threading ctx through every call site (and through every partial).
// The page's data is passed as the template dot directly — no wrapper —
// so templates read `.Title` rather than `.Data.Title`.
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

// pageTmpls holds one base template set per page, parsed once at init.
// Per request we Clone the relevant base and bind the request-scoped
// urlFor before Execute — Clone is necessary for ctx-bound funcs to be
// concurrent-safe across requests.
var pageTmpls = map[string]*template.Template{
	"index":   parseSet("pages/index.html"),
	"product": parseSet("pages/product.html"),
	"team":    parseSet("pages/team.html"),
	"contact": parseSet("pages/contact.html"),
	"post":    parseSet("post/page.html"),
}

// parseSet builds a base template set for one page: the page's own body
// file (which defines "body") plus all shared partials — layout, ui atoms,
// ui molecules, and the post-feature partials. urlFor is registered as a
// placeholder so the parser accepts references to it; the real ctx-bound
// urlFor is bound on each Clone in Render. Only the body varies per page.
func parseSet(body string) *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"urlFor": urlForPlaceholder,
		"args":   args,
	})
	return template.Must(t.ParseFS(tmplFS,
		"templates/layout/public.html",
		"templates/ui/atoms/*.html",
		"templates/ui/molecules/*.html",
		"templates/post/*.html",
		"templates/"+body,
	))
}

// urlForPlaceholder satisfies parse-time func resolution. It is replaced
// with a ctx-bound closure on the cloned template before Execute; if it
// is ever invoked, it indicates Render forgot to rebind.
func urlForPlaceholder(string, ...any) (string, error) {
	return "", fmt.Errorf("urlFor invoked without per-request rebinding")
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
	base, ok := pageTmpls[p.page]
	if !ok {
		return fmt.Errorf("unknown page %q", p.page)
	}
	t, err := base.Clone()
	if err != nil {
		return err
	}
	t.Funcs(template.FuncMap{
		"urlFor": func(name string, a ...any) (string, error) {
			return structpages.URLFor(ctx, structpages.Ref(name), a...)
		},
	})
	return t.ExecuteTemplate(w, p.entry, p.data)
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
