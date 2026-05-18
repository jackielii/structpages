package main

import (
	"context"
	"fmt"

	"github.com/jackielii/structpages"
)

// Typed URL helpers — one per generated URL family. The recommended
// URLFor shape is two-arg: URLFor(ctx, page, params).
//
//   - page is one of: a typed page value, a []any composition slice
//     (typed chain + literal URL fragments), or a Ref string (fallback
//     for cross-package references).
//   - params is a map[string]any with placeholder values.
//
// Wrapping URL generation in helpers like these is what gives you "no
// dangling URLs in production": refactor blast radius is localised,
// the integration test exercises each helper end-to-end, and the
// init-time validator (validate.go) sanity-checks them on boot.

// urlForHome — bare typed page, no params, no composition.
func urlForHome(ctx context.Context) (string, error) {
	return structpages.URLFor(ctx, homePage{})
}

// urlForGroupIndex — chain through the unique parent type to the
// shared groupIndex leaf. The []any{parent, leaf} form is the
// recommended disambiguation for "same leaf type, different parents".
func urlForGroupIndex(ctx context.Context, group string) (string, error) {
	parent, ok := groupParent(group)
	if !ok {
		return "", fmt.Errorf("urlForGroupIndex: unknown group %q", group)
	}
	return structpages.URLFor(ctx, []any{parent, groupIndex{}})
}

// urlForGroupDetail — chain + slug param. Showcases the common two-arg
// shape: URLFor(ctx, []any{...chain}, map[string]any{...params}).
func urlForGroupDetail(ctx context.Context, group, slug string) (string, error) {
	parent, ok := groupParent(group)
	if !ok {
		return "", fmt.Errorf("urlForGroupDetail: unknown group %q", group)
	}
	return structpages.URLFor(ctx,
		[]any{parent, entryPage{}},
		map[string]any{"slug": slug})
}

// urlForGroupIndexWithTab — composition: chain + literal URL fragment
// for a query template, plus params that fill both the chain
// placeholders (none here) and the fragment ones.
func urlForGroupIndexWithTab(ctx context.Context, group, tab string) (string, error) {
	parent, ok := groupParent(group)
	if !ok {
		return "", fmt.Errorf("urlForGroupIndexWithTab: unknown group %q", group)
	}
	return structpages.URLFor(ctx,
		[]any{parent, groupIndex{}, "?tab={tab}"},
		map[string]any{"tab": tab})
}

// urlForByRef — cross-package fallback. In this single-package demo
// we don't actually need Ref (we have the types), but the call shape
// is what you'd use when an unrelated package can't import this one.
// Qualified Ref ("Components.Index") walks the page tree by Name.
func urlForByRef(ctx context.Context, qualified string) (string, error) {
	return structpages.URLFor(ctx, structpages.Ref(qualified))
}
