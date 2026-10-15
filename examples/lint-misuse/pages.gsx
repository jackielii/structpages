package main

import (
	"fmt"
	"strconv"
)

// BadLinks deliberately hard-codes internal URLs as targets for the
// structpages-lint [url-attr] rule (gsxscan): each URL attribute below
// except the external one is flagged.
//
// NOTE ON gsx AUTO-ESCAPING: gsx escapes URL-context attributes
// (href, hx-get, action, …) by context. That means a dynamically-
// constructed javascript: or data: URL would be neutralised, preventing
// XSS via URL sinks. However, the [url-attr] lint rule catches a
// different problem — routing-correctness: hard-coded path strings that
// bypass structpages.URLFor break when routes are renamed. That class of
// bug is orthogonal to XSS escaping, so gsx's auto-escaping does not
// make these findings go away.
component BadLinks(id int, name string) {
	<a href="/login">Hard-coded internal</a>
	<a href={"/" + "admin"}>Expression literal</a>
	<a href={"/items/" + strconv.Itoa(id)}>Concat</a>
	<a href={fmt.Sprintf("/users/%s", name)}>Sprintf</a>
	<a hx-get={"/api/items"}>Bad hx-get</a>
	<form action={"/submit"}>Bad action</form>
	<a href="https://example.com/external">External (allowed)</a>
}

// ParamLinks exercises the [params] check on the variadic key/value
// form that `|> url(key, value)` lowers to:
// structpages.URLFor(ctx, itemDetail{}, "nope", 1).
component ParamLinks(slug string) {
	<a href={itemDetail{} |> url("slug", slug)}>OK: slug is a placeholder</a>
	<a href={itemDetail{} |> url("nope", 1)}>BAD: nope is not a placeholder</a>
	// OK: a .gsx directive suppresses the analyzer's finding on the
	// generated URLFor call, which //line maps back to this line.
	// structpages:lint:ignore params
	<a href={itemDetail{} |> url("alsoNope", 1)}>suppressed</a>
	<a
		// structpages:lint:ignore params
		href={itemDetail{} |> url("stillNope", 1)}
	>
		suppressed from the attribute list
	</a>
	<a href={itemDetail{} |> url("unsuppressed", 1)}>BAD: no directive</a>
}
