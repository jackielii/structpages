package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackielii/structpages"
)

// BadLinks mirrors the templ original — it deliberately uses hard-coded
// internal URLs so the structpages-lint [url-attr] rule has targets to flag.
// In gsx the component uses inline params just like the templ version.
//
// NOTE ON gsx AUTO-ESCAPING: gsx escapes URL-context attributes
// (href, hx-get, action, …) by context. That means a dynamically-
// constructed javascript: or data: URL would be neutralised, preventing
// XSS via URL sinks. However, the [url-attr] lint rule catches a
// different problem — routing-correctness: hard-coded path strings that
// bypass structpages.URLFor break when routes are renamed. That class of
// bug is orthogonal to XSS escaping, so gsx's auto-escaping does NOT
// make these findings go away. The lint still has value in gsx projects.
component BadLinks(id int, name string) {
	<a href="/login">Hard-coded internal</a>
	<a href={ "/" + "admin" }>Expression literal</a>
	<a href={ "/items/" + strconv.Itoa(id) }>Concat</a>
	<a href={ fmt.Sprintf("/users/%s", name) }>Sprintf</a>
	<a hx-get={ "/api/items" }>Bad hx-get</a>
	<form action={ "/submit" }>Bad action</form>
	<a href="https://example.com/external">External (allowed)</a>
}

// urlFor is the app-level convenience wrapper used by gsx templates
// (avoids repeating the package qualifier; returns (string, error) so
// gsx auto-unwraps the error at the call site in URL-context attrs).
func urlFor(ctx context.Context, page any, args ...any) (string, error) {
	return structpages.URLFor(ctx, page, args...)
}
