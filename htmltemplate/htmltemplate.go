// Package htmltemplate provides helpers for using html/template with structpages.
//
// structpages itself is render-engine agnostic: any value with a
// Render(ctx context.Context, w io.Writer) error method can be returned from
// a Page() method. This package only adds the small piece of glue that is
// awkward to write by hand — a [template.FuncMap] that lets templates call
// urlFor / idFor / idTarget with the request context.
//
// Typical usage:
//
//	// At parse time, register placeholder funcs so the parser knows the names:
//	var tmpl = template.Must(template.New("").
//	    Funcs(htmltemplate.Funcs(nil)).
//	    ParseFS(fs, "*.html"))
//
//	// At render time, clone and rebind with the request context:
//	t, _ := tmpl.Clone()
//	t.Funcs(htmltemplate.Funcs(ctx))
//	_ = t.ExecuteTemplate(w, "layout", data)
//
// In templates:
//
//	<a href="{{ urlFor "ProductPage" }}">Product</a>
//	<a href="{{ urlFor "UserPage" "id" 42 }}">User 42</a>
//	<button hx-target="{{ idTarget "Dashboard.UserList" }}">Refresh</button>
package htmltemplate

import (
	"context"
	"html/template"

	"github.com/jackielii/structpages"
)

// Funcs returns a [template.FuncMap] that binds urlFor, idFor, and idTarget
// to ctx. Page references use the [structpages.Ref] convention: page name
// (e.g. "ProductPage") or route path (e.g. "/products"); for ids, the dotted
// "PageName.MethodName" form is also supported.
//
// Pass a nil ctx at parse time to register placeholder funcs, then call Funcs
// again with the request context before [template.Template.Execute]. Calls
// made with a nil ctx return empty strings rather than panicking, so a
// forgotten rebind surfaces as missing URLs rather than a 500.
func Funcs(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"urlFor": func(name string, args ...any) (string, error) {
			if ctx == nil {
				return "", nil
			}
			return structpages.URLFor(ctx, structpages.Ref(name), args...)
		},
		"idFor": func(name string) (string, error) {
			if ctx == nil {
				return "", nil
			}
			return structpages.ID(ctx, structpages.Ref(name))
		},
		"idTarget": func(name string) (string, error) {
			if ctx == nil {
				return "", nil
			}
			return structpages.IDTarget(ctx, structpages.Ref(name))
		},
	}
}
