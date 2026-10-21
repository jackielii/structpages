// Command escape passes URLFor values already escaped with net/url, which
// URLFor escapes again. The [escape] check flags each such value.
package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jackielii/structpages"
)

type searchPage struct{}

type pages struct {
	Search searchPage `route:"/search/{id} Search"`
}

func main() {
	mux := http.NewServeMux()
	structpages.Mount(mux, pages{}, "/", "App")
}

func links(ctx context.Context, q string, v url.Values, args []any) {
	structpages.URLFor(ctx, []any{searchPage{}, "?q={q}"}, "id", "1", "q", url.QueryEscape(q))                 // flagged
	structpages.URLFor(ctx, []any{searchPage{}, "?q={q}"}, map[string]any{"id": "1", "q": url.QueryEscape(q)}) // flagged
	structpages.URLFor(ctx, searchPage{}, url.PathEscape(q))                                                   // flagged
	structpages.URLFor(ctx, []any{searchPage{}, "?{q}"}, "1", v.Encode())                                      // flagged
	structpages.URLFor(ctx, []any{searchPage{}, "?q={q}"}, "id", "1", "q", (url.QueryEscape(q)))               // flagged

	var sp *structpages.StructPages
	sp.URLFor([]any{searchPage{}, "?q={q}"}, "id", "1", "q", url.QueryEscape(q)) // flagged

	structpages.URLFor(ctx, []any{searchPage{}, "?q={q}"}, "id", "1", "q", q)
	structpages.URLFor(ctx, []any{searchPage{}, "?" + v.Encode()}, "1")
	structpages.URLFor(ctx, searchPage{}, args...)
	structpages.URLFor(ctx, searchPage{}, url.PathEscape(q)) //structpages:lint:ignore escape
}
