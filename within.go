package structpages

import (
	"context"
	"errors"
	"fmt"
)

// Within reports whether the page being rendered is page or one of its
// descendants. It answers the active-navigation question "is the current
// page under this nav target?" without matching URL prefixes, so a target
// that is a group (a struct whose fields are routes) matches every page
// below it, and a target whose route has path parameters needs no args.
//
// page takes the forms URLFor takes for its page argument: a page value,
// a [Ref], a top-level string as Ref sugar, a []any chain of page steps,
// or a func(*PageNode) bool predicate. A chain may not carry string
// fragments, since Within compares pages, not URLs.
//
// Pages are compared by node, so an index leaf (`route:"/{$}"`) and the
// group it sits in stay distinct: Within(ctx, Dashboard{}) holds only on
// the dashboard, while Within(ctx, Manage{}) holds on every page under
// Manage.
//
// Outside a page render — no [CurrentPage] in ctx — it returns false and
// no error. A page that does not resolve is an error, so a renamed nav
// target fails the render instead of quietly never matching.
//
//	active, err := structpages.Within(ctx, structpages.Ref("Feedback.Manage"))
func Within(ctx context.Context, page any) (bool, error) {
	current := CurrentPage(ctx)
	if current == nil {
		return false, nil
	}
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return false, errors.New("parse context not found in context")
	}
	if s, ok := page.(string); ok {
		page = Ref(s)
	}
	parts, ok := page.([]any)
	if !ok {
		parts = []any{page}
	}
	if len(parts) == 0 {
		return false, errors.New("within: empty []any chain")
	}
	for i, part := range parts {
		if _, ok := part.(string); ok {
			return false, fmt.Errorf("within: chain element %d is a string fragment; "+
				"Within takes page steps only", i)
		}
	}
	target, err := pc.resolveChain(parts)
	if err != nil {
		return false, fmt.Errorf("within: %w", err)
	}
	for n := current; n != nil; n = n.Parent {
		if n == target {
			return true, nil
		}
	}
	return false, nil
}
