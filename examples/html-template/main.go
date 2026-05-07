// Demonstrates structpages with the standard library's html/template.
//
// structpages is render-engine agnostic: a Page() method can return any value
// with a Render(ctx context.Context, w io.Writer) error method. Here, the
// `tpl` type wraps an html/template set; structpages/htmltemplate.Funcs
// supplies the urlFor / idFor / idTarget bindings inside templates.
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

//go:embed templates/*.html
var tmplFS embed.FS

// pageTmpls holds one fully-parsed (layout + body) template set per page.
// Parsing per page avoids the html/template name collision that happens when
// every page defines a "body" template inside one shared set.
var pageTmpls = map[string]*template.Template{
	"index":   parsePage("index.html"),
	"product": parsePage("product.html"),
	"team":    parsePage("team.html"),
	"contact": parsePage("contact.html"),
}

func parsePage(file string) *template.Template {
	// Funcs(nil) registers placeholder names so the parser accepts urlFor/idFor
	// references; we rebind with a real context inside Render.
	t := template.New("").Funcs(htmltemplate.Funcs(nil))
	return template.Must(t.ParseFS(tmplFS, "templates/layout.html", "templates/"+file))
}

// tpl is a renderable component backed by one of the parsed page templates.
// `entry` selects which named template to execute (e.g. "layout" for the full
// page, "body" for the HTMX partial swap).
type tpl struct {
	page  string
	entry string
	data  any
}

func (p tpl) Render(ctx context.Context, w io.Writer) error {
	t, ok := pageTmpls[p.page]
	if !ok {
		return fmt.Errorf("unknown template %q", p.page)
	}
	clone, err := t.Clone()
	if err != nil {
		return err
	}
	clone.Funcs(htmltemplate.Funcs(ctx))
	return clone.ExecuteTemplate(w, p.entry, p.data)
}

func full(name string) tpl    { return tpl{page: name, entry: "layout"} }
func partial(name string) tpl { return tpl{page: name, entry: "body"} }

type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
}

func (index) Page() tpl { return full("index") }
func (index) Main() tpl { return partial("index") }

type product struct{}

func (product) Page() tpl { return full("product") }
func (product) Main() tpl { return partial("product") }

type team struct{}

func (team) Page() tpl { return full("team") }
func (team) Main() tpl { return partial("team") }

type contact struct{}

func (contact) Page() tpl { return full("contact") }
func (contact) Main() tpl { return partial("contact") }

func main() {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, index{}, "/", "Index",
		// htmx 4 sends HX-Target as "<tag>#<id>" instead of the bare id.
		structpages.WithTargetSelector(structpages.HTMXv4RenderTarget),
	); err != nil {
		log.Fatalf("mount: %v", err)
	}
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
