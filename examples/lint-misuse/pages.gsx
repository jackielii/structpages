package main

import (
	"fmt"
	"strconv"
)

// BadLinks mirrors the templ original — it deliberately uses hard-coded
// internal URLs as targets for the structpages-lint [url-attr] rule.
// In gsx the component uses inline params just like the templ version.
//
// NOTE: the [url-attr] rule is implemented on templ's parser and does
// NOT scan .gsx files yet, so nothing in this file is currently flagged
// (see the pinned snapshot in lint_test.go). These targets are kept for
// the future gsxscan port of [url-attr].
//
// NOTE ON gsx AUTO-ESCAPING: gsx escapes URL-context attributes
// (href, hx-get, action, …) by context. That means a dynamically-
// constructed javascript: or data: URL would be neutralised, preventing
// XSS via URL sinks. However, the [url-attr] lint rule catches a
// different problem — routing-correctness: hard-coded path strings that
// bypass structpages.URLFor break when routes are renamed. That class of
// bug is orthogonal to XSS escaping, so gsx's auto-escaping would not
// make such findings go away once [url-attr] learns to scan .gsx.
component BadLinks(id int, name string) {
	<a href="/login">Hard-coded internal</a>
	<a href={"/" + "admin"}>Expression literal</a>
	<a href={"/items/" + strconv.Itoa(id)}>Concat</a>
	<a href={fmt.Sprintf("/users/%s", name)}>Sprintf</a>
	<a hx-get={"/api/items"}>Bad hx-get</a>
	<form action={"/submit"}>Bad action</form>
	<a href="https://example.com/external">External (allowed)</a>
}
