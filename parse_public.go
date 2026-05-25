package structpages

import (
	"context"
)

// Parse builds a page tree from page without registering any HTTP
// routes. The returned *StructPages exposes URLFor, ID, IDTarget,
// and PageContext, but never touches a mux.
//
// Use Parse for tests and tooling that need URL/ID resolution
// against the real page tree but don't want to spin up a server.
// For production use, prefer Mount, which performs the same parse
// step and additionally registers handlers.
//
// Options behave identically to Mount: WithArgs, WithURLPrefix,
// WithErrorHandler, etc. all apply. WithMiddlewares and other
// mux-affecting options are accepted but never observed (no
// handlers are wired).
//
// Example (test):
//
//	sp, err := structpages.Parse(webPages{}, "/", "App")
//	if err != nil { t.Fatal(err) }
//	ctx := sp.PageContext(context.Background())
//	body := renderTo(ctx, List{}.Page(props))
func Parse(page any, route, title string, options ...Option) (*StructPages, error) {
	sp := &StructPages{
		targetSelector: HTMXRenderTarget,
	}
	for _, opt := range options {
		opt(sp)
	}
	pc, err := parsePageTree(route, page, sp.args...)
	if err != nil {
		return nil, err
	}
	pc.root.Title = title
	pc.urlPrefix = sp.urlPrefix
	sp.pc = pc
	return sp, nil
}

// PageContext returns a context derived from ctx with sp's page
// tree attached. URLFor, ID, and IDTarget invoked with the returned
// context resolve against sp instead of failing with "parse context
// not found in context".
//
// Use this in tests that render templ components with
// context.Background(): wrap the bare context once via PageContext
// and the renders behave as if they ran under a real request.
//
//	sp, _ := structpages.Parse(webPages{}, "/", "App")
//	ctx := sp.PageContext(context.Background())
//	html := mustRender(ctx, MyPage{}.Page(props))
func (sp *StructPages) PageContext(ctx context.Context) context.Context {
	return pcCtx.WithValue(ctx, sp.pc)
}
