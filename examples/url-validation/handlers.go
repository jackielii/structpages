package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackielii/structpages"
)

// title uppercases the first byte — adequate for the ASCII fixture
// names this demo uses. Avoids strings.Title (deprecated).
func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Convention: each page has a Props method that captures the request
// context (and the *PageNode when group resolution is needed), and a
// Page method that renders using the captured state. structpages calls
// Props first and passes the result to Page.
//
// All URL generation goes through the typed helpers in urls.go — Page
// methods never call structpages.URLFor with []any chains directly.
// This keeps the surface area of refactorable strings minimal and gives
// the integration test in integration_test.go a clean inventory to
// validate.

// --- home page ----------------------------------------------------------

type homePage struct{}

type homeProps struct{ ctx context.Context }

func (homePage) Props(r *http.Request) homeProps { return homeProps{ctx: r.Context()} }

func (homePage) Page(p homeProps) htmlResp {
	links := []struct{ group, label, desc string }{
		{"foundations", "Foundations", "colors, typography, spacing"},
		{"components", "Components", "button, input, dialog"},
		{"patterns", "Patterns", "form-layout, data-table"},
	}
	var b strings.Builder
	b.WriteString("<h1>url-validation demo</h1>")
	b.WriteString("<p>Three shared-type groups. Every URL on this page is generated through a typed helper, so a renamed field or moved route breaks the integration test and the boot validator before it breaks production.</p>")

	// Section 1: chain form — disambiguates shared leaf types via parent.
	b.WriteString("<h2>chain form: <code>[]any{parent, leaf}</code></h2><ul>")
	for _, l := range links {
		url, err := urlForGroupIndex(p.ctx, l.group)
		if err != nil {
			fmt.Fprintf(&b, "<li>error: %v</li>", err)
			continue
		}
		fmt.Fprintf(&b, `<li><a href=%q>%s</a> — %s</li>`, url, l.label, l.desc)
	}
	b.WriteString("</ul>")

	// Section 2: chain + URL fragment composition (e.g. query template).
	b.WriteString("<h2>composition: <code>[]any{parent, leaf, &quot;?tab={tab}&quot;}</code></h2><ul>")
	for _, l := range links {
		url, err := urlForGroupIndexWithTab(p.ctx, l.group, "overview")
		if err != nil {
			fmt.Fprintf(&b, "<li>error: %v</li>", err)
			continue
		}
		fmt.Fprintf(&b, `<li><a href=%q>%s overview</a></li>`, url, l.label)
	}
	b.WriteString("</ul>")

	// Section 3: Ref form — cross-package fallback.
	b.WriteString("<h2>Ref form: <code>structpages.Ref(&quot;Parent.Field&quot;)</code></h2><ul>")
	for _, l := range links {
		qualified := title(l.group) + ".Index"
		url, err := urlForByRef(p.ctx, qualified)
		if err != nil {
			fmt.Fprintf(&b, "<li>error: %v</li>", err)
			continue
		}
		fmt.Fprintf(&b, `<li><a href=%q>%s (via Ref %q)</a></li>`, url, l.label, qualified)
	}
	b.WriteString("</ul>")

	return htmlResp(b.String())
}

// --- group index --------------------------------------------------------

type groupIndex struct{}

type indexProps struct {
	ctx   context.Context
	group string
}

func (groupIndex) Props(r *http.Request, node *structpages.PageNode) indexProps {
	return indexProps{ctx: r.Context(), group: resolveGroup(node)}
}

func (groupIndex) Page(p indexProps) htmlResp {
	homeURL, err := urlForHome(p.ctx)
	if err != nil {
		return htmlf("<h1>error: %v</h1>", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<p><a href=%q>← home</a></p><h1>%s</h1><ul>`, homeURL, title(p.group))
	for _, slug := range entries[p.group] {
		url, err := urlForGroupDetail(p.ctx, p.group, slug)
		if err != nil {
			fmt.Fprintf(&b, "<li>error: %v</li>", err)
			continue
		}
		fmt.Fprintf(&b, `<li><a href=%q>%s</a></li>`, url, slug)
	}
	b.WriteString("</ul>")
	return htmlResp(b.String())
}

// --- entry detail -------------------------------------------------------

type entryPage struct{}

type detailProps struct {
	ctx   context.Context
	group string
	slug  string
}

func (entryPage) Props(r *http.Request, node *structpages.PageNode) detailProps {
	return detailProps{ctx: r.Context(), group: resolveGroup(node), slug: r.PathValue("slug")}
}

func (entryPage) Page(p detailProps) htmlResp {
	indexURL, err := urlForGroupIndex(p.ctx, p.group)
	if err != nil {
		return htmlf("<h1>error: %v</h1>", err)
	}
	return htmlf(
		`<p><a href=%q>← %s</a></p><h1>%s / %s</h1><p>Detail page for entry %q in group %q.</p>`,
		indexURL, title(p.group), title(p.group), p.slug, p.slug, p.group,
	)
}
