package lint

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

// checkIDOrIDTarget validates `structpages.ID(ctx, v)` and
// `structpages.IDTarget(ctx, v)`. Three cases:
//
//   - Method expression (`p.UserList`): the receiver type must be
//     mounted as a page.
//   - Ref string: must resolve as a page in the tree; if the ref is
//     qualified (Page.Method), the trailing segment must be a method
//     on that page; if it's simple, it must be a method on exactly
//     one page.
//   - Plain string literal: no check (literal id passes through).
//   - Standalone function: no check (allowed by runtime).
func checkIDOrIDTarget(ctx *checkCtx, call *ast.CallExpr, fnName string) {
	if len(call.Args) != 2 {
		return
	}
	if ctx.tree == nil || len(ctx.tree.Roots) == 0 {
		return
	}
	arg := unparen(call.Args[1])

	// String literal? No check.
	if _, ok := stringConstantFromPass(ctx, arg); ok {
		// But only if the argument's static type is plain string. If
		// it's a Ref (also a string under the hood), fall through.
		if t := ctx.pass.TypesInfo.TypeOf(arg); t != nil {
			if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String && !isRefType(t) {
				return
			}
		}
	}

	// Ref(...) — validate as a method reference.
	if c, ok := arg.(*ast.CallExpr); ok && isRefConversion(ctx.pass.TypesInfo, c) {
		checkIDRef(ctx, c, fnName)
		return
	}

	// Method expression or standalone function.
	checkIDFuncArg(ctx, arg, fnName)
}

// checkIDRef validates a Ref passed to ID/IDTarget. Supported shapes:
//   - "Page.Method" — page must exist and method must be on it
//   - "Method" — must be on exactly one mounted page
func checkIDRef(ctx *checkCtx, call *ast.CallExpr, fnName string) {
	if len(call.Args) != 1 {
		return
	}
	ref, ok := stringConstantFromPass(ctx, call.Args[0])
	if !ok {
		return
	}
	pos := call.Args[0].Pos()
	if ref == "" {
		ctx.report(pos, "idfor", "%s: empty Ref", fnName)
		return
	}
	if strings.Contains(ref, ".") {
		idx := strings.LastIndex(ref, ".")
		pagePath := ref[:idx]
		method := ref[idx+1:]
		if pagePath == "" || method == "" {
			ctx.report(pos, "idfor", "%s: malformed qualified Ref %q", fnName, ref)
			return
		}
		node := resolveRef(ctx, pos, ref, true)
		if node == nil {
			return
		}
		if _, ok := node.Methods[method]; !ok {
			methods := mapMethodNames(node.Methods)
			ctx.report(pos, "idfor",
				"%s: Ref %q: method %q not found on page %q; available: %s",
				fnName, ref, method, node.Name, strings.Join(methods, ", "))
		}
		return
	}
	// Simple method name — must be on exactly one mounted page.
	var matches []*PageNode
	for _, root := range ctx.tree.Roots {
		root.All(func(n *PageNode) bool {
			if _, ok := n.Methods[ref]; ok {
				matches = append(matches, n)
			}
			return true
		})
	}
	switch len(matches) {
	case 1:
		return
	case 0:
		ctx.report(pos, "idfor",
			"%s: Ref %q: method not found on any mounted page", fnName, ref)
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		sort.Strings(names)
		ctx.report(pos, "idfor",
			"%s: Ref %q: method found on multiple pages: %s; use qualified form like %q",
			fnName, ref, strings.Join(names, ", "), names[0]+"."+ref)
	}
}

// checkIDFuncArg validates a method-expression (or function) argument
// to ID/IDTarget. For a method expression p.Method, the receiver type
// must be mounted as a page. Standalone functions are accepted
// without check.
func checkIDFuncArg(ctx *checkCtx, expr ast.Expr, fnName string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return // Not a method expression: standalone function or variable.
	}
	use, ok := ctx.pass.TypesInfo.Uses[sel.Sel]
	if !ok {
		return
	}
	fn, ok := use.(*types.Func)
	if !ok {
		return
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return
	}
	recv := sig.Recv().Type()
	named, ok := resolveNamedStruct(recv)
	if !ok {
		return
	}
	if !ctx.isMounted(named) {
		ctx.report(expr.Pos(), "idfor",
			"%s: method expression %s.%s: receiver type %s is not mounted as a page",
			fnName, named.Obj().Name(), fn.Name(), named.Obj().Name())
	}
}

// isMounted reports whether named appears as the Type of any node in
// the tree.
func (ctx *checkCtx) isMounted(named *types.Named) bool {
	key := typeKey(named)
	for _, root := range ctx.tree.Roots {
		found := false
		root.All(func(n *PageNode) bool {
			if typeKey(n.Type) == key {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// isRefType reports whether t is structpages.Ref (a defined string
// type), as opposed to a plain string.
func isRefType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == structpagesPkg && obj.Name() == "Ref"
}

func mapMethodNames(m map[string]*types.Func) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
