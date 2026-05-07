// Package htmltemplate provides helpers for using html/template with structpages.
//
// structpages itself is render-engine agnostic: any value with a
// Render(ctx context.Context, w io.Writer) error method can be returned from
// a Page() method. This package adds the small pieces of glue that are
// awkward to write by hand:
//
//   - [View], a request-scoped data wrapper exposing URL / ID / Target methods
//     so templates can call them via the dot, with no per-request Clone.
//   - [Args], a `map[string]any` builder for passing multiple inputs to
//     partial templates.
//   - [Funcs], a small [template.FuncMap] of the same helpers in func form,
//     for cases where you'd rather call `urlFor` than `.URL`. Calling [Funcs]
//     forces a Clone-per-render to rebind the context, so [View] is the
//     recommended path; reach for [Funcs] only when methods-on-data don't fit.
//
// Recommended usage with [View]:
//
//	var tmpl = template.Must(template.New("").
//	    Funcs(template.FuncMap{"args": htmltemplate.Args}).
//	    ParseFS(fs, "*.html"))
//
//	// At render time, no Clone needed:
//	v := htmltemplate.NewView(r.Context(), pageProps)
//	_ = tmpl.ExecuteTemplate(w, "layout", v)
//
// In templates:
//
//	<a href="{{ .URL "ProductPage" }}">Product</a>
//	<a href="{{ .URL "UserPage" "id" 42 }}">User 42</a>
//	<button hx-target="{{ .Target "Dashboard.UserList" }}">Refresh</button>
//	{{ template "ui/molecules/card" (.Sub .Data.Card) }}
//	{{ template "ui/molecules/card" (args "Title" .Data.Title "Body" .Data.Body) }}
package htmltemplate

import (
	"context"
	"fmt"
	"html/template"

	"github.com/jackielii/structpages"
)

// View is the request-scoped data wrapper passed as the dot when executing
// templates. It carries the request context plus the page's typed data, and
// exposes URL / ID / Target methods so templates can build links and ids
// without a ctx-aware FuncMap (no per-request Clone needed).
//
// Build one with [NewView] and pass it as the data argument to
// [template.Template.ExecuteTemplate]. In templates, access fields via
// .Data and helpers directly:
//
//	{{ .URL "Product" }}        // page link
//	{{ .Data.Title }}           // typed page data
//	{{ template "ui/molecules/card" (.Sub .Data.Post) }}
type View[T any] struct {
	// View is the per-request data wrapper passed through templates;
	// carrying ctx is its whole point.
	Ctx  context.Context //nolint:containedctx
	Data T
}

// NewView builds a View carrying ctx and data.
func NewView[T any](ctx context.Context, data T) View[T] {
	return View[T]{Ctx: ctx, Data: data}
}

// URL resolves a structpages page reference to a URL using the View's
// context. name follows the [structpages.Ref] convention (page name or
// route path); args are forwarded to [structpages.URLFor].
func (v View[T]) URL(name string, args ...any) (string, error) {
	if v.Ctx == nil {
		return "", nil
	}
	return structpages.URLFor(v.Ctx, structpages.Ref(name), args...)
}

// ID resolves a structpages component reference to a raw HTML id (no leading
// "#"). The name supports the dotted "PageName.MethodName" form as well as
// a bare method name when unambiguous.
func (v View[T]) ID(name string) (string, error) {
	if v.Ctx == nil {
		return "", nil
	}
	return structpages.ID(v.Ctx, structpages.Ref(name))
}

// Target resolves a structpages component reference to a CSS selector
// (with leading "#"), suitable for an HTMX hx-target attribute.
func (v View[T]) Target(name string) (string, error) {
	if v.Ctx == nil {
		return "", nil
	}
	return structpages.IDTarget(v.Ctx, structpages.Ref(name))
}

// Sub returns a new View carrying the same context with d as Data, so a
// partial template can keep the URL / ID / Target helpers while seeing its
// own slice of the page state on .Data.
//
//	{{ template "ui/molecules/card" (.Sub .Data.Post) }}
//
// Sub erases the type parameter to View[any]; if the partial needs typed
// access, type-assert .Data inside the partial's Go-side rendering or just
// rely on template-side reflection (the common case).
func (v View[T]) Sub(d any) View[any] {
	return View[any]{Ctx: v.Ctx, Data: d}
}

// Args returns a map[string]any built from alternating key/value pairs.
// Use it as a template func to pass multiple inputs to partial templates:
//
//	{{ template "ui/molecules/card" (args "Title" .Data.Title "Body" .Data.Body) }}
//
// Register it once at parse time:
//
//	template.New("").Funcs(template.FuncMap{"args": htmltemplate.Args})
//
// For organisms with four or more fields, prefer a typed Go struct over
// repeated args calls.
//
// Args returns an error if the number of arguments is odd or if any key is
// not a string.
func Args(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("args: odd number of arguments (%d) — expected key/value pairs", len(kv))
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

// Funcs returns a [template.FuncMap] that binds urlFor, idFor, and idTarget
// to ctx. Page references use the [structpages.Ref] convention.
//
// Prefer [View] for new code: View threads context through template data
// (the dot) and avoids the per-request [template.Template.Clone] +
// [template.Template.Funcs] dance that Funcs forces. Funcs is kept for the
// minority of cases where you'd rather write `{{ urlFor "x" }}` than
// `{{ .URL "x" }}`.
//
// Pass a nil ctx at parse time to register placeholder funcs, then call
// Funcs again with the request context (after Clone) before
// [template.Template.Execute]. Calls made with a nil ctx return empty
// strings rather than panicking, so a forgotten rebind surfaces as missing
// URLs rather than a 500.
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
