// Package htmltemplate provides a small set of helpers for using
// html/template with structpages — nothing more.
//
// structpages itself is render-engine agnostic: any value with a
// Render(ctx context.Context, w io.Writer) error method can be returned from
// a Page() method. This package contributes:
//
//   - [URLFor], [ID], [IDTarget]: thin wrappers over [structpages.URLFor],
//     [structpages.ID], [structpages.IDTarget] that take a name string (a
//     [structpages.Ref]). All of them take ctx as their first argument so
//     they can be registered as template funcs once at parse time and
//     evaluated per request without [template.Template.Clone].
//
//   - [Args]: a `map[string]any` builder for passing multiple inputs to a
//     partial template in lieu of a per-call Go struct.
//
//   - [Funcs]: returns the helpers above bundled as a [template.FuncMap]
//     under the conventional names `urlFor`, `idFor`, `idTarget`, `args`.
//
// The package does not define a "view" type or dictate how ctx is exposed
// inside templates. The ctx-first signature is the entire mechanism — the
// caller is free to pass ctx via any field, key, or convention they like.
//
// Typical usage:
//
//	var tmpl = template.Must(template.New("").
//	    Funcs(htmltemplate.Funcs()).
//	    ParseFS(fs, "*.html"))
//
//	// Per request — no Clone:
//	_ = tmpl.ExecuteTemplate(w, "layout", struct {
//	    Ctx  context.Context
//	    Data any
//	}{r.Context(), pageProps})
//
// In templates:
//
//	<a href="{{ urlFor .Ctx "ProductPage" }}">Product</a>
//	<a href="{{ urlFor .Ctx "UserPage" "id" 42 }}">User 42</a>
//	<button hx-target="{{ idTarget .Ctx "Dashboard.UserList" }}">Refresh</button>
//	{{ template "card" (args "Title" .Data.Title "Body" .Data.Body) }}
package htmltemplate

import (
	"context"
	"fmt"
	"html/template"

	"github.com/jackielii/structpages"
)

// URLFor resolves a structpages page reference to a URL. name follows the
// [structpages.Ref] convention (page name like "ProductPage" or route path
// like "/products"); args are forwarded to [structpages.URLFor].
//
// Use directly inside Go code, or via the `urlFor` template func returned
// by [Funcs]:
//
//	{{ urlFor .Ctx "ProductPage" }}
//	{{ urlFor .Ctx "UserPage" "id" 42 }}
func URLFor(ctx context.Context, name string, args ...any) (string, error) {
	return structpages.URLFor(ctx, structpages.Ref(name), args...)
}

// ID resolves a structpages component reference to a raw HTML id (no
// leading "#"). The name supports the dotted "PageName.MethodName" form
// as well as a bare method name when unambiguous.
//
//	<div id="{{ idFor .Ctx "Dashboard.UserList" }}">…
func ID(ctx context.Context, name string) (string, error) {
	return structpages.ID(ctx, structpages.Ref(name))
}

// IDTarget resolves a structpages component reference to a CSS selector
// (with leading "#"), suitable for an HTMX hx-target attribute.
//
//	<button hx-target="{{ idTarget .Ctx "Dashboard.UserList" }}">…
func IDTarget(ctx context.Context, name string) (string, error) {
	return structpages.IDTarget(ctx, structpages.Ref(name))
}

// Args returns a map[string]any built from alternating key/value pairs.
// Use it as a template func to pass multiple inputs to partial templates:
//
//	{{ template "ui/molecules/card" (args "Title" .Data.Title "Body" .Data.Body) }}
//
// Returns an error if the number of arguments is odd or if any key is
// not a string. For partials with four or more inputs, prefer a typed
// Go struct over repeated args calls.
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

// Funcs returns the helpers in this package bundled as a [template.FuncMap]
// under the names `urlFor`, `idFor`, `idTarget`, and `args`.
//
// Register once at parse time:
//
//	var tmpl = template.Must(template.New("").
//	    Funcs(htmltemplate.Funcs()).
//	    ParseFS(fs, "*.html"))
//
// All ctx-aware helpers take ctx as their first argument, so the same
// parsed *template.Template can be executed concurrently across requests
// with no [template.Template.Clone]. Make ctx available on the template
// dot in whatever shape suits you — a `Ctx` field on a wrapping struct
// is the simplest:
//
//	{{ urlFor .Ctx "ProductPage" }}
//	{{ idTarget .Ctx "Dashboard.UserList" }}
func Funcs() template.FuncMap {
	return template.FuncMap{
		"urlFor":   URLFor,
		"idFor":    ID,
		"idTarget": IDTarget,
		"args":     Args,
	}
}
