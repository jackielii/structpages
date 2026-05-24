// Package main is a deliberately-broken structpages app used as the
// dogfood fixture for tools/lint/cmd/structpages-lint. Every "BAD" comment marks
// a call the linter is expected to flag; lint_test.go snapshots the
// exact diagnostic output so regressions in message wording surface
// in CI.
package main

import (
	"context"
	"net/http"

	"github.com/jackielii/structpages"
)

// Page tree:
//
//   root
//     home (/)
//     items
//       index (/items/)
//       detail (/items/{slug})

type root struct {
	Home  homePage  `route:"/{$} Home"`
	Items itemsRoot `route:"/items Items"`
}

type homePage struct{}

func (homePage) Page() string { return "home" }

type itemsRoot struct {
	Index  itemsIndex `route:"/{$} Items index"`
	Detail itemDetail `route:"/{slug} Item detail"`
}

type itemsIndex struct{}

func (itemsIndex) Page() string { return "items index" }

type itemDetail struct{}

func (itemDetail) Page() string  { return "items detail" }
func (itemDetail) Stats() string { return "stats partial" }

// unmountedPage is intentionally not referenced by any route tag —
// IDTarget(ctx, p.Title) below should be flagged.
type unmountedPage struct{}

func (unmountedPage) Title() string { return "title" }

func main() {
	mux := http.NewServeMux()
	if _, err := structpages.Mount(mux, &root{}, "/", "lint-misuse demo"); err != nil {
		panic(err)
	}
	// Reference urlSamples so the linter has call sites to analyse
	// without producing an "unused function" hint when run with the
	// usual go vet / unusedfunc analyzers.
	_ = urlSamples
}

// urlSamples demonstrates the four families of failure the linter
// catches. Every annotated BAD line should produce one diagnostic
// (the test asserts exact wording).
func urlSamples(ctx context.Context) {
	// BAD: ref/qualified — Items has no child "NoSuch"
	_, _ = structpages.URLFor(ctx, structpages.Ref("Items.NoSuch"))

	// BAD: ref/route — no such full route
	_, _ = structpages.URLFor(ctx, structpages.Ref("/missing"))

	// BAD: ref/name — no page named "Ghost"
	_, _ = structpages.URLFor(ctx, structpages.Ref("Ghost"))

	// OK: bare typed lookup
	_, _ = structpages.URLFor(ctx, homePage{})

	// BAD: urlfor/typed — chain says items->home, but home is not under items
	_, _ = structpages.URLFor(ctx, []any{itemsRoot{}, homePage{}})

	// BAD: urlfor/typed — typed value after string fragment
	_, _ = structpages.URLFor(ctx, []any{itemsRoot{}, "/extra", itemDetail{}})

	// BAD: params — "wrong" is not a placeholder in /items/{slug}
	_, _ = structpages.URLFor(ctx, []any{itemsRoot{}, itemDetail{}},
		map[string]any{"wrong": "x"})

	// BAD: idfor — receiver type unmountedPage is not in the page tree
	var p unmountedPage
	_, _ = structpages.IDTarget(ctx, p.Title)

	// OK: methodexpr on a mounted page
	var d itemDetail
	_, _ = structpages.IDTarget(ctx, d.Stats)

	// OK: would be a [ref] diagnostic but directive on previous line silences it.
	//structpages:lint:ignore ref
	_, _ = structpages.URLFor(ctx, structpages.Ref("Suppressed"))

	// OK: directive without category silences any class.
	_, _ = structpages.URLFor(ctx, structpages.Ref("AlsoSuppressed")) //structpages:lint:ignore
}
