// Demonstrates structpages with the standard library's html/template,
// organized in an atomic-design layout (layout / ui atoms / ui molecules /
// feature partials / pages).
//
// structpages is render-engine agnostic: a Page() method can return any
// value with a Render(ctx context.Context, w io.Writer) error method. The
// `tpl` type here is a thin wrapper around an html/template set, and
// htmltemplate.View is the request-scoped data wrapper that exposes
// .URL / .ID / .Target inside templates without per-request Cloning.
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
	"github.com/jackielii/structpages/htmltemplate"
)

//go:embed templates
var tmplFS embed.FS

// pageTmpls holds one fully-parsed template set per page. Each set has its
// own "body" definition (page-specific) plus shared layout / ui / feature
// partials. We never Clone at render time — htmltemplate.View threads the
// request context through the template data instead of mutating funcs.
var pageTmpls = map[string]*template.Template{
	"index":   parseSet("layout/public.html", "pages/index.html"),
	"product": parseSet("layout/public.html", "pages/product.html"),
	"team":    parseSet("layout/public.html", "pages/team.html"),
	"contact": parseSet("layout/public.html", "pages/contact.html"),
	"post":    parseSet("layout/public.html", "post/page.html"),
}

func parseSet(layout, body string) *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"args": htmltemplate.Args,
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

// tpl is a renderable component backed by one of the parsed page sets.
// `entry` selects which named template to execute — "layout/public" for the
// full page, "body" for the HTMX content swap, or e.g. "post/comments-list"
// for an organism partial.
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
	return t.ExecuteTemplate(w, p.entry, htmltemplate.NewView(ctx, p.data))
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

func loadPostProps() postProps {
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

type post struct{}

func (post) Page() tpl {
	return tpl{page: "post", entry: "layout/public", data: loadPostProps()}
}

// Main renders just the page body — used for HTMX nav swaps targeting <main>.
func (post) Main() tpl {
	return tpl{page: "post", entry: "body", data: loadPostProps()}
}

// Comments renders just the comments organism. structpages's
// HTMXv4RenderTarget routes HX-Target=section#comments (or tag-only
// "comments") here by matching the method name to the kebab-cased component
// id. The partial template is wrapped in <section id="comments"> so it
// targets itself for subsequent swaps.
func (post) Comments() tpl {
	return tpl{page: "post", entry: "post/comments-list", data: loadPostProps().Comments}
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
