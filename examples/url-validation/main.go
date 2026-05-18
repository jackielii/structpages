// Showcases every URLFor form against the his-project design-system
// preview skeleton: one render type per role (groupIndex, entryPage)
// mounted under three parents (foundations, components, patterns).
//
// Forms demonstrated (see urls.go for the helpers, url_test.go for the
// per-form unit assertions):
//
//  1. bare typed page:    URLFor(ctx, page{})
//  2. chain disambig.:    URLFor(ctx, []any{parent, leaf})
//  3. chain + params:     URLFor(ctx, []any{parent, leaf}, map[string]any{...})
//  4. chain + fragment:   URLFor(ctx, []any{parent, leaf, "?tab={t}"}, map[string]any{...})
//  5. Ref single name:    URLFor(ctx, Ref("PageName"))
//  6. Ref qualified path: URLFor(ctx, Ref("Parent.Field"))
//  7. Ref by route:       URLFor(ctx, Ref("/components/{slug}"))
//
// And the two validation guards that turn refactorable strings into
// production-safe references:
//
//   - validate.go runs at boot (main calls it after Mount); a dangling
//     URL kills the process before it accepts requests.
//   - integration_test.go renders every page and asserts URLs in the
//     body; runs in CI on every commit.
//
// Visit / and follow links to confirm each nav lands on the right page.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/jackielii/structpages"
)

func main() {
	mux := http.NewServeMux()
	sp, err := structpages.Mount(mux, &root{}, "/", "url-validation demo")
	if err != nil {
		log.Fatalf("mount: %v", err)
	}
	// Fail fast on any dangling URL — a renamed field, moved route,
	// or broken Ref kills the boot, not the first request that hits
	// it. See validate.go for the inventory + rationale.
	if err := validateURLs(sp); err != nil {
		log.Fatalf("URL validation failed:\n%v", err)
	}
	log.Println("listening on :8080  →  http://localhost:8080/")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

// htmlResp is a minimal renderable component: structpages accepts any
// type with `Render(context.Context, io.Writer) error`, so we skip
// templ entirely and write HTML strings directly.
type htmlResp string

func (h htmlResp) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, string(h))
	return err
}

func htmlf(format string, a ...any) htmlResp {
	return htmlResp(fmt.Sprintf(format, a...))
}
