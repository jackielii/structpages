package lint

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

// checkParamMap validates `URLFor(ctx, page, map[string]any{...})`
// calls. It checks the literal map's keys against the route pattern's
// `{name}` and `{name...}` segments. Missing keys are emitted at low
// confidence because the runtime can pull params from request context
// — false positives there are easy to silence with the directive.
//
// fragment is the concatenated trailing string fragments of an
// []any chain (e.g., "?preset={preset}"). Its placeholders are
// added to the known set so keys filled by the fragment don't
// trigger false-positive diagnostics.
func checkParamMap(ctx *checkCtx, node *PageNode, fragment string, args []ast.Expr) {
	if len(args) == 0 {
		return
	}
	mapLit, ok := args[0].(*ast.CompositeLit)
	if !ok {
		return
	}
	if !isStringAnyMap(ctx.pass.TypesInfo, mapLit) {
		return
	}

	pattern := node.FullRoute + fragment
	segments := parsePatternSegments(pattern)
	if len(segments) == 0 && len(mapLit.Elts) == 0 {
		return
	}
	want := map[string]bool{}
	for _, s := range segments {
		want[s] = true
	}
	got := map[string]ast.Expr{}
	for _, e := range mapLit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := stringConstantFromPass(ctx, kv.Key)
		if !ok {
			continue
		}
		got[key] = kv.Key
	}

	for k, keyExpr := range got {
		if !want[k] {
			knownSorted := mapKeys(want)
			ctx.report(keyExpr.Pos(), "params",
				"URLFor: param %q does not appear in pattern %q (known: %s)",
				k, pattern, strings.Join(knownSorted, ", "))
		}
	}
}

// isStringAnyMap reports whether expr's type is map[string]any (or an
// alias of it).
func isStringAnyMap(info *types.Info, expr ast.Expr) bool {
	t := info.TypeOf(expr)
	if t == nil {
		return false
	}
	m, isMap := t.Underlying().(*types.Map)
	if !isMap {
		return false
	}
	k, isBasic := m.Key().Underlying().(*types.Basic)
	if !isBasic || k.Kind() != types.String {
		return false
	}
	v, isIface := m.Elem().Underlying().(*types.Interface)
	if !isIface {
		return false
	}
	return v.Empty()
}

// parsePatternSegments walks a route pattern and returns the names of
// every `{name}` and `{name...}` placeholder. `{$}` is ignored.
//
// Mirrors structpages.parseSegments without producing the segment
// metadata the runtime needs.
func parsePatternSegments(pattern string) []string {
	var names []string
	rest := pattern
	for {
		i := strings.Index(rest, "{")
		if i == -1 {
			return names
		}
		rest = rest[i+1:]
		j := strings.Index(rest, "}")
		if j == -1 {
			return names
		}
		name := rest[:j]
		rest = rest[j+1:]
		if name == "$" {
			continue
		}
		name = strings.TrimSuffix(name, "...")
		names = append(names, name)
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
