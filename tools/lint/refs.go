package lint

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

// checkRefConversion validates a `structpages.Ref(stringExpr)` call.
// The argument must be a string constant; non-literal arguments are
// skipped silently.
//
// Three shape-driven sub-checks:
//   - "/..." → must match a node.FullRoute
//   - "X.Y.Z" → walk a qualified path from root or top-level
//   - "Name" → must match some node.Name
//
// Method-suffix (ID-aware) validation is handled separately when the
// Ref is the argument to ID/IDTarget; this function only validates
// the page-resolution layer.
func checkRefConversion(ctx *checkCtx, call *ast.CallExpr) {
	if len(call.Args) != 1 {
		return
	}
	s, ok := stringConstantFromPass(ctx, call.Args[0])
	if !ok {
		return
	}
	if ctx.tree == nil || len(ctx.tree.Roots) == 0 {
		return
	}
	_ = resolveRef(ctx, call.Args[0].Pos(), s, false)
}

// resolveRef validates a Ref string against the page tree. It honours
// ctx.silent — callers that have already reported the failure should
// flip ctx.silent before invoking this to suppress duplicate
// diagnostics.
func resolveRef(ctx *checkCtx, pos token.Pos, ref string, isIDContext bool) *PageNode {
	if ref == "" {
		ctx.report(pos, "ref", "Ref(\"\"): empty reference")
		return nil
	}
	switch {
	case strings.HasPrefix(ref, "/"):
		return resolveRefByRoute(ctx, pos, ref)
	case strings.Contains(ref, "."):
		// In ID context the last segment is a method; trim it off
		// before resolving the page path.
		pagePath := ref
		if isIDContext {
			idx := strings.LastIndex(ref, ".")
			pagePath = ref[:idx]
			if pagePath == "" {
				ctx.report(pos, "ref", "Ref %q: missing page name before method", ref)
				return nil
			}
		}
		return resolveRefByQualified(ctx, pos, ref, pagePath)
	default:
		if isIDContext {
			// Simple method name — resolved against every page in the
			// tree by the ID check, not here. We can still verify it
			// matches *some* page's method, but the multi-page rule
			// (must be unambiguous) lives in idfor.go.
			return nil
		}
		return resolveRefByName(ctx, pos, ref)
	}
}

func resolveRefByRoute(ctx *checkCtx, pos token.Pos, ref string) *PageNode {
	var match *PageNode
	var allRoutes []string
	for _, root := range ctx.tree.Roots {
		root.All(func(n *PageNode) bool {
			allRoutes = append(allRoutes, n.FullRoute)
			if n.FullRoute == ref {
				match = n
				return false
			}
			return true
		})
		if match != nil {
			return match
		}
	}
	ctx.report(pos, "ref",
		"Ref %q: no page with this route. Did you rename a route tag? Known routes: %s",
		ref, joinSortedUnique(allRoutes, 8))
	return nil
}

func resolveRefByQualified(ctx *checkCtx, pos token.Pos, ref, pagePath string) *PageNode {
	segments := strings.Split(pagePath, ".")
	if len(segments) == 0 || segments[0] == "" {
		ctx.report(pos, "ref", "Ref %q: empty qualified path", ref)
		return nil
	}
	for _, root := range ctx.tree.Roots {
		var current *PageNode
		if root.Name == segments[0] {
			current = root
		} else {
			for _, c := range root.Children {
				if c.Name == segments[0] {
					current = c
					break
				}
			}
		}
		if current == nil {
			continue
		}
		ok := true
		for i, name := range segments[1:] {
			var next *PageNode
			for _, c := range current.Children {
				if c.Name == name {
					next = c
					break
				}
			}
			if next == nil {
				childNames := make([]string, len(current.Children))
				for j, c := range current.Children {
					childNames[j] = c.Name
				}
				sort.Strings(childNames)
				ctx.report(pos, "ref",
					"Ref %q: segment %d (%q) not found as child of %q; available: %s",
					ref, i+1, name, current.Name, strings.Join(childNames, ", "))
				return nil
			}
			current = next
			_ = ok
		}
		return current
	}
	// Anchor not found in any root.
	var topNames []string
	for _, root := range ctx.tree.Roots {
		topNames = append(topNames, root.Name)
		for _, c := range root.Children {
			topNames = append(topNames, c.Name)
		}
	}
	ctx.report(pos, "ref",
		"Ref %q: anchor %q not found at root or top level; available: %s",
		ref, segments[0], joinSortedUnique(topNames, 16))
	return nil
}

func resolveRefByName(ctx *checkCtx, pos token.Pos, ref string) *PageNode {
	var allNames []string
	for _, root := range ctx.tree.Roots {
		var found *PageNode
		root.All(func(n *PageNode) bool {
			allNames = append(allNames, n.Name)
			if n.Name == ref {
				found = n
				return false
			}
			return true
		})
		if found != nil {
			return found
		}
	}
	ctx.report(pos, "ref",
		"Ref %q: no page with this name; known names include: %s",
		ref, joinSortedUnique(allNames, 16))
	return nil
}

// joinSortedUnique returns a sorted, comma-separated list of up to
// max entries from in. Empty strings are dropped.
func joinSortedUnique(in []string, limit int) string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > limit {
		out = append(out[:limit], "...")
	}
	return strings.Join(out, ", ")
}
