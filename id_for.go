package structpages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// ID generates a raw HTML ID for a component method (without "#" prefix).
// Use this for HTML id attributes.
//
// Parameters:
//   - ctx: Context containing parseContext (required for method expressions and Ref)
//   - v: One of:
//   - Method expression (p.UserList) - generates ID from page and method name
//   - Ref type (structpages.Ref("PageName.MethodName")) - looks up page/method dynamically
//   - Plain string ("my-custom-id") - returned as-is
//
// Example:
//
//	<div id={ structpages.ID(ctx, p.UserList) }>
//	// → <div id="team-management-view-user-list">
//
//	<div id={ structpages.ID(ctx, UserStatsWidget) }>
//	// → <div id="user-stats-widget"> (no page prefix for standalone functions)
//
//	<div id={ structpages.ID(ctx, "my-custom-id") }>
//	// → <div id="my-custom-id">
//
// Returns an error if parseContext is not found in the provided context.
func ID(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - ID must be called within a page handler or template")
	}

	return idFor(pc, currentPageCtx.Value(ctx), v, true)
}

// IDTarget generates a CSS selector (with "#" prefix) for a component method.
// Use this for HTMX hx-target attributes.
//
// Parameters:
//
//   - ctx: Context containing parseContext (required for method expressions and Ref)
//   - v: One of:
//   - Method expression (p.UserList) - generates selector from page and method name
//   - Ref type (structpages.Ref("PageName.MethodName")) - looks up page/method dynamically
//   - string ("body" or "#my-custom-id") - returned as-is
//
// Example:
//
//	<button hx-target={ structpages.IDTarget(ctx, p.UserList) }>
//	// → <button hx-target="#team-management-view-user-list">
//
//	<button hx-target={ structpages.IDTarget(ctx, UserStatsWidget) }>
//	// → <button hx-target="#user-stats-widget"> (no page prefix for standalone functions)
//
//	<button hx-target={ structpages.IDTarget(ctx, "body") }>
//	// → <button hx-target="body">
//
// Returns an error if parseContext is not found in the provided context.
func IDTarget(ctx context.Context, v any) (string, error) {
	// Extract parseContext - REQUIRED
	pc := pcCtx.Value(ctx)
	if pc == nil {
		return "", errors.New("parseContext not found in context - IDTarget must be called within a page handler or template")
	}

	return idFor(pc, currentPageCtx.Value(ctx), v, false)
}

// idFor generates the ID string based on the provided value
// (method expression, Ref, plain string, or standalone function).
//
// When currentPage is non-nil and the method expression's receiver
// type matches the current page's type, the id is derived from the
// current page's field Name. This guarantees self-render produces
// the right id when the same struct type is mounted under multiple
// parents with different field names (topologies C and D from the
// design discussion). When currentPage is nil or the receiver type
// doesn't match, the resolver falls back to a global tree lookup —
// matches the existing behavior used by sp.ID / sp.IDTarget and
// cross-page renders.
func idFor(pc *parseContext, currentPage *PageNode, v any, rawID bool) (string, error) {
	methodExpr := v

	// Handle Ref type for dynamic method references
	if ref, ok := methodExpr.(Ref); ok {
		return idForRef(pc, string(ref), rawID)
	}

	// Handle []any chain form: typed chain steps + trailing method spec.
	// Parallels URLFor's []any{Parent{}, Leaf{}, "?frag"} composition,
	// but the trailing element is a method name string or a method
	// expression whose receiver type is the chain leaf.
	if parts, ok := methodExpr.([]any); ok {
		return idForChain(pc, currentPage, parts, rawID)
	}

	// Handle plain string as literal ID - return as-is
	if str, ok := methodExpr.(string); ok {
		return str, nil
	}

	// Validate that we have a method expression (function) before extracting method info
	rv := reflect.ValueOf(methodExpr)
	if rv.Kind() != reflect.Func {
		return "", fmt.Errorf("unsupported type %T: expected method expression, Ref, or string", methodExpr)
	}

	// Extract all method info (handles methods and standalone functions)
	info, err := extractMethodInfo(methodExpr)
	if err != nil {
		return "", err
	}

	// Standalone functions are shared components with no tree position;
	// they are prefixed by their package name instead of a page path.
	if info.isFunction {
		return pc.functionID(info, rawID), nil
	}

	pn, err := resolvePageForMethod(pc, currentPage, info)
	if err != nil {
		return "", fmt.Errorf("cannot find page for method expression: %w", err)
	}
	return pc.componentID(pn, info.methodName, rawID), nil
}

// idForChain resolves the []any composition form for ID/IDTarget.
// The trailing element is the method spec:
//
//   - string — used as the method name on the chain leaf's type.
//   - method expression / method value — its receiver type IS the
//     chain leaf, and its name supplies the method. If the prior
//     chain step's type matches the receiver type, the method
//     expression's implicit leaf is collapsed (no duplicate descend).
//
// Leading typed values form the chain steps, resolved by the same
// rules URLFor's chain form uses (first step by type lookup,
// subsequent steps by child-type descent).
func idForChain(pc *parseContext, currentPage *PageNode, parts []any, rawID bool) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("ID: empty []any chain")
	}
	last := parts[len(parts)-1]
	chainSteps := parts[:len(parts)-1]

	var methodName string
	var fromMethodExpr *methodInfo

	switch t := last.(type) {
	case string:
		if t == "" {
			return "", errors.New("ID: empty method name in []any chain")
		}
		methodName = t
	case nil:
		return "", errors.New("ID: nil trailing element in []any chain")
	default:
		rv := reflect.ValueOf(last)
		if rv.Kind() != reflect.Func {
			return "", fmt.Errorf(
				"ID: trailing element of []any chain must be a method name string or method expression, got %T",
				last)
		}
		info, err := extractMethodInfo(last)
		if err != nil {
			return "", fmt.Errorf("ID: invalid method expression in chain: %w", err)
		}
		if info.isFunction {
			return "", errors.New("ID: trailing element is a standalone function; " +
				"chain form expects a method expression bound to a page type")
		}
		fromMethodExpr = info
		methodName = info.methodName
	}

	// When the trailing element is a method expression, its receiver
	// type is an implicit final chain step. Append it unless the
	// caller already wrote the type explicitly (e.g.,
	// []any{Parent{}, Leaf{}, leafPage.Method} where Leaf{} and
	// leafPage agree — collapse to a single descend rather than
	// double-stepping).
	if fromMethodExpr != nil {
		recv := fromMethodExpr.receiverType
		recvElem := recv
		if recvElem.Kind() == reflect.Pointer {
			recvElem = recvElem.Elem()
		}
		duplicate := false
		if n := len(chainSteps); n > 0 {
			lastStepType := reflect.TypeOf(chainSteps[n-1])
			if pointerType(lastStepType) == pointerType(recv) {
				duplicate = true
			}
		}
		if !duplicate {
			chainSteps = append(chainSteps, reflect.New(recvElem).Elem().Interface())
		}
	}

	if len(chainSteps) == 0 {
		return "", errors.New("ID: []any chain has no page context; " +
			"provide at least one typed chain step or use a method expression")
	}

	leaf, err := pc.resolveChain(chainSteps)
	if err != nil {
		return "", fmt.Errorf("ID: %w", err)
	}

	// Self-render override: if the resolved leaf type matches the
	// current page's type, prefer the current page's field name.
	// This mirrors the bare method-expression behavior from idFor.
	if currentPage != nil && pointerType(currentPage.Value.Type()) == pointerType(leaf.Value.Type()) {
		leaf = currentPage
	}

	leafType := pointerType(leaf.Value.Type())
	if _, ok := leafType.MethodByName(methodName); !ok {
		return "", fmt.Errorf("ID: method %q not found on chain leaf type %s",
			methodName, leafType.Elem().Name())
	}

	return pc.componentID(leaf, methodName, rawID), nil
}

// resolvePageForMethod picks the PageNode whose Name supplies the
// id prefix. It prefers the currentPage when its receiver type
// matches the method expression — that way a page rendering its
// own template gets the id for *its* mount, not whichever match
// tree-walk encounters first.
//
// For cross-page calls (no current page, or its type doesn't
// match), the resolver collects every mount of the receiver type.
// Identical mount field names produce identical ids and are
// silently collapsed (e.g. an entryPage mounted under three
// section roots all named "EntryDetail" — the user explicitly
// chose this shape). Divergent field names produce different ids;
// the resolver refuses to silently pick one and surfaces a
// disambiguation error instead.
func resolvePageForMethod(pc *parseContext, currentPage *PageNode, info *methodInfo) (*PageNode, error) {
	if currentPage != nil && pageNodeMatchesMethod(currentPage, info) {
		return currentPage, nil
	}
	matches := pc.collectPageNodesForMethod(info)
	switch len(matches) {
	case 0:
		if info.isBound {
			return nil, fmt.Errorf("no page node found with type name %q", info.receiverTypeName)
		}
		return nil, fmt.Errorf("no page node found for type %s", pointerType(info.receiverType).String())
	case 1:
		return matches[0], nil
	}
	first := matches[0]
	firstID := pc.componentID(first, info.methodName, true)
	allSameID := true
	for _, m := range matches[1:] {
		if pc.componentID(m, info.methodName, true) != firstID {
			allSameID = false
			break
		}
	}
	if allSameID {
		return first, nil
	}
	// Build a useful error: list each distinct (mount, id) pair.
	type opt struct {
		name, route, id string
	}
	seen := map[string]bool{}
	var opts []opt
	for _, m := range matches {
		id := pc.componentID(m, info.methodName, true)
		if seen[id] {
			continue
		}
		seen[id] = true
		opts = append(opts, opt{
			name:  m.Name,
			route: m.FullRoute(),
			id:    id,
		})
	}
	descs := make([]string, len(opts))
	for i, o := range opts {
		descs[i] = fmt.Sprintf("%s at %s → %q", o.name, o.route, o.id)
	}
	return nil, fmt.Errorf(
		"ID: type %s is mounted under multiple fields producing different ids: %s; "+
			"disambiguate with the []any chain form, a Ref, or move the slot to a "+
			"standalone function",
		first.Value.Type().String(), strings.Join(descs, "; "))
}

// collectPageNodesForMethod returns every PageNode whose value type
// matches info's receiver. For bound method values (isBound), it
// additionally verifies the method exists on the type — matching
// the historical behavior of findPageNodeByTypeName. For unbound
// method expressions, the caller has already vouched for the
// method by writing the expression, so we trust it.
func (p *parseContext) collectPageNodesForMethod(info *methodInfo) []*PageNode {
	var out []*PageNode
	for node := range p.root.All() {
		nodeType := node.Value.Type()
		if info.isBound {
			nodeTypeName := nodeType.Name()
			if nodeType.Kind() == reflect.Pointer {
				nodeTypeName = nodeType.Elem().Name()
			}
			if nodeTypeName != info.receiverTypeName {
				continue
			}
			if _, found := nodeType.MethodByName(info.methodName); !found {
				continue
			}
		} else if pointerType(nodeType) != pointerType(info.receiverType) {
			continue
		}
		out = append(out, node)
	}
	return out
}

// pageNodeMatchesMethod reports whether pn's value type is the
// receiver type for info (either by reflect.Type or by type name
// for bound method values).
func pageNodeMatchesMethod(pn *PageNode, info *methodInfo) bool {
	if pn == nil {
		return false
	}
	nodeType := pn.Value.Type()
	if info.isBound {
		nodeTypeName := nodeType.Name()
		if nodeType.Kind() == reflect.Pointer {
			nodeTypeName = nodeType.Elem().Name()
		}
		if nodeTypeName != info.receiverTypeName {
			return false
		}
	} else if pointerType(nodeType) != pointerType(info.receiverType) {
		return false
	}
	// Confirm the method exists on this type — guards against
	// matching a same-named type that doesn't have the method.
	_, found := pointerType(nodeType).MethodByName(info.methodName)
	return found
}

// findPageNodeForMethod finds a page node using the method info.
// Only call this for methods, not for standalone functions.
func (p *parseContext) findPageNodeForMethod(info *methodInfo) (*PageNode, error) {
	if info.isFunction {
		panic("findPageNodeForMethod called with standalone function")
	}

	if info.isBound {
		// Find by type name string
		return p.findPageNodeByTypeName(info.receiverTypeName, info.methodName)
	}
	// Find by reflect.Type
	return p.findPageNodeByType(info.receiverType)
}

// findPageNodeByTypeName finds a PageNode by matching its type name.
// Also verifies that the method exists on the page.
func (p *parseContext) findPageNodeByTypeName(typeName, methodName string) (*PageNode, error) {
	for node := range p.root.All() {
		nodeType := node.Value.Type()
		nodeTypeName := nodeType.Name()
		if nodeType.Kind() == reflect.Pointer {
			nodeTypeName = nodeType.Elem().Name()
		}
		if nodeTypeName == typeName {
			// Verify the method exists
			if _, found := nodeType.MethodByName(methodName); !found {
				return nil, fmt.Errorf("method %q not found on page %q", methodName, node.Name)
			}
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page node found with type name %q", typeName)
}

// findPageNodeByType finds a PageNode by its receiver type.
func (p *parseContext) findPageNodeByType(receiverType reflect.Type) (*PageNode, error) {
	// Normalize to pointer type for comparison
	targetType := pointerType(receiverType)

	for node := range p.root.All() {
		nodeType := pointerType(node.Value.Type())
		if targetType == nodeType {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page node found for type %s", targetType.String())
}

// idForRef handles dynamic method references using the Ref type.
// It supports both qualified references (PageName.MethodName) and simple method names.
func idForRef(pc *parseContext, ref string, rawID bool) (string, error) {
	var pageName, methodName string

	// Check if qualified reference (PageName.MethodName)
	if idx := strings.Index(ref, "."); idx != -1 {
		pageName = ref[:idx]
		methodName = ref[idx+1:]
	} else {
		// Simple method name - find which page(s) have it
		methodName = ref
		matches := findPagesWithMethod(pc, methodName)

		if len(matches) == 0 {
			return "", fmt.Errorf("method %q not found on any page", methodName)
		}
		if len(matches) > 1 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return "", fmt.Errorf("method %q found on multiple pages: %s. Use qualified name like %q",
				methodName, strings.Join(names, ", "), names[0]+"."+methodName)
		}
		pageName = matches[0].Name
	}

	// Find the page(s) carrying this name. Names are no longer globally
	// unique (e.g. two embedded types with the same name under different
	// parents), so collect every match and refuse to guess when more than
	// one carries the method.
	var named, withMethod []*PageNode
	for node := range pc.root.All() {
		if node.Name != pageName {
			continue
		}
		named = append(named, node)
		if _, found := node.Value.Type().MethodByName(methodName); found {
			withMethod = append(withMethod, node)
		}
	}
	if len(named) == 0 {
		return "", fmt.Errorf("no page found with name %q", pageName)
	}
	if len(withMethod) == 0 {
		return "", fmt.Errorf("method %q not found on page %q", methodName, pageName)
	}
	if len(withMethod) > 1 {
		descs := make([]string, len(withMethod))
		for i, m := range withMethod {
			descs[i] = fmt.Sprintf("%s → %q", m.FullRoute(), pc.componentID(m, methodName, true))
		}
		return "", fmt.Errorf(
			"Ref %q is ambiguous: name %q is mounted at multiple routes: %s; "+
				"disambiguate with the []any chain form or move the slot to a standalone function",
			ref, pageName, strings.Join(descs, ", "))
	}

	return pc.componentID(withMethod[0], methodName, rawID), nil
}

// findPagesWithMethod finds all pages that have a method with the given name.
func findPagesWithMethod(pc *parseContext, methodName string) []*PageNode {
	var matches []*PageNode
	for node := range pc.root.All() {
		if _, found := node.Value.Type().MethodByName(methodName); found {
			matches = append(matches, node)
		}
	}
	return matches
}

// defaultMaxIDLen is the character budget for a generated element id
// before it degrades from the readable full-path form to the compact
// leaf-only form. Overridable via WithMaxIDLength.
const defaultMaxIDLen = 40

// assignIDPaths populates idPath and idCompactSuffix on every node in
// the tree. idPath is the kebab-cased field-name path from the root
// (root excluded) down to the node; for the root itself it is the
// root's own kebab name. A node whose leaf name is shared by another
// node gets a stable "-<hash>" compact suffix so the leaf-only id form
// stays unique.
func (p *parseContext) assignIDPaths() {
	leafCount := make(map[string]int)
	for node := range p.root.All() {
		node.idPath = idPathFor(node)
		leafCount[node.idPath[len(node.idPath)-1]]++
	}
	for node := range p.root.All() {
		if leafCount[node.idPath[len(node.idPath)-1]] > 1 {
			node.idCompactSuffix = "-" + shortHash(strings.Join(node.idPath, "/"))
		} else {
			node.idCompactSuffix = ""
		}
	}
}

// idPathFor builds the kebab-cased field-name path from the root
// (exclusive) down to node. The root node itself has no ancestors, so
// it falls back to its own kebab name.
func idPathFor(node *PageNode) []string {
	var segs []string
	for n := node; n != nil && n.Parent != nil; n = n.Parent {
		segs = append(segs, camelToKebab(n.Name))
	}
	if len(segs) == 0 {
		return []string{camelToKebab(node.Name)}
	}
	slices.Reverse(segs)
	return segs
}

// shortHash returns the first 4 hex characters (16 bits) of the SHA-256
// of s — a stable, deterministic disambiguator for colliding leaf names.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:4]
}

// checkIDUniqueness verifies that no two distinct (node, component
// method) pairs resolve to the same element id under the current
// maxIDLen. The path-based scheme guarantees this except for an
// astronomically unlikely hash collision, which we surface here rather
// than ship silently.
func (p *parseContext) checkIDUniqueness() error {
	type owner struct{ route, method string }
	seen := make(map[string]owner)
	for node := range p.root.All() {
		for method := range node.Components {
			id := p.componentID(node, method, true)
			cur := owner{route: node.FullRoute(), method: method}
			if prev, ok := seen[id]; ok && prev != cur {
				return fmt.Errorf(
					"element id %q is produced by both %s.%s and %s.%s; "+
						"rename a mount field to disambiguate",
					id, prev.route, prev.method, cur.route, cur.method)
			}
			seen[id] = cur
		}
	}
	return nil
}

// componentID constructs the HTML id string for a method on node.
//
// It prefers the readable full-path form (every ancestor field name
// joined); if that exceeds maxIDLen it degrades to the compact
// leaf-only form, with a stable hash suffix appended when the leaf name
// is not unique in the tree. A nil node yields the bare method name.
func (p *parseContext) componentID(node *PageNode, methodName string, rawID bool) string {
	if node == nil {
		return p.idFromPrefix(nil, "", methodName, rawID)
	}
	return p.idFromPrefix(node.idPath, node.idCompactSuffix, methodName, rawID)
}

// functionID constructs the HTML id for a standalone function component.
// Standalone functions are shared across pages and have no tree position,
// so they are prefixed by their (short) package name to keep ids from two
// same-named functions in different packages distinct.
func (p *parseContext) functionID(info *methodInfo, rawID bool) string {
	var prefix []string
	if info.packageName != "" {
		prefix = []string{camelToKebab(info.packageName)}
	}
	return p.idFromPrefix(prefix, "", info.methodName, rawID)
}

// idFromPrefix joins a pre-kebabed prefix path with the kebab method
// name. When the full form exceeds maxIDLen it degrades to the last
// prefix segment plus the method, appending compactSuffix for
// disambiguation. An empty prefix yields the bare method name.
func (p *parseContext) idFromPrefix(prefix []string, compactSuffix, methodName string, rawID bool) string {
	method := camelToKebab(methodName)
	var id string
	switch {
	case len(prefix) == 0:
		id = method
	default:
		full := strings.Join(prefix, "-") + "-" + method
		if len(full) <= p.maxIDLen {
			id = full
		} else {
			id = prefix[len(prefix)-1] + "-" + method + compactSuffix
		}
	}
	if !rawID {
		id = "#" + id
	}
	return id
}
