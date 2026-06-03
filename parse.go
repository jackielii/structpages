package structpages

import (
	"cmp"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type parseContext struct {
	root           *PageNode
	args           argRegistry
	segmentCache   map[string][]segment
	segmentCacheMu sync.RWMutex
	// urlPrefix, if non-empty, is prepended to every URL produced by URLFor.
	// Set by WithURLPrefix when structpages is deployed behind something that
	// strips a path prefix (e.g., http.StripPrefix or a reverse proxy). It
	// does NOT affect route registration — that is controlled by Mount's
	// route argument.
	urlPrefix string
	// maxIDLen is the character budget for a generated element id before
	// it degrades from the readable full-path form to the compact
	// leaf-only form. Defaults to defaultMaxIDLen; overridable via
	// WithMaxIDLength.
	maxIDLen int
}

func parsePageTree(route string, page any, args ...any) (*parseContext, error) {
	pc := &parseContext{
		args:         make(map[reflect.Type]reflect.Value),
		segmentCache: make(map[string][]segment),
		maxIDLen:     defaultMaxIDLen,
	}
	for _, v := range args {
		if err := pc.args.addArg(v); err != nil {
			return nil, fmt.Errorf("error adding argument to registry: %w", err)
		}
	}
	topNode, err := pc.parsePageTree(route, "", page)
	if err != nil {
		return nil, err
	}
	pc.root = topNode
	pc.assignIDPaths()
	if err := pc.checkIDUniqueness(); err != nil {
		return nil, err
	}
	return pc, nil
}

func (p *parseContext) parsePageTree(route, fieldName string, page any) (*PageNode, error) {
	if page == nil {
		return nil, fmt.Errorf("page cannot be nil")
	}

	st, pt, err := getStructAndPointerTypes(page)
	if err != nil {
		return nil, err
	}

	item := &PageNode{Value: reflect.ValueOf(page), Name: cmp.Or(fieldName, st.Name())}
	item.Method, item.Route, item.Title = parseTag(route)

	// Parse child fields
	if err := p.parseChildFields(st, item); err != nil {
		return nil, err
	}

	// Process methods
	if err := p.processMethods(st, pt, item); err != nil {
		return nil, err
	}

	return item, nil
}

// getStructAndPointerTypes extracts struct and pointer types from a page
func getStructAndPointerTypes(page any) (structType, pointerType reflect.Type, err error) {
	st := reflect.TypeOf(page) // struct type
	pt := reflect.TypeOf(page) // pointer type
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}

	// Ensure we have a struct type
	if st.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("page must be a struct or pointer to struct, got %v", st.Kind())
	}

	return st, pt, nil
}

// parseChildFields parses child fields with route tags
func (p *parseContext) parseChildFields(st reflect.Type, item *PageNode) error {
	for i := range st.NumField() {
		field := st.Field(i)
		route, ok := field.Tag.Lookup("route")
		if !ok {
			continue
		}
		typ := field.Type
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		childPage := reflect.New(typ)
		childItem, err := p.parsePageTree(route, field.Name, childPage.Interface())
		if err != nil {
			return err
		}
		childItem.Parent = item
		item.Children = append(item.Children, childItem)
	}
	return nil
}

// processMethods processes all methods of the page
func (p *parseContext) processMethods(st, pt reflect.Type, item *PageNode) error {
	for _, t := range []reflect.Type{st, pt} {
		for i := range t.NumMethod() {
			method := t.Method(i)
			if isPromotedMethod(&method) {
				continue // skip promoted methods
			}
			if err := p.processMethod(item, &method); err != nil {
				return err
			}
		}
	}
	return nil
}

// processMethod processes a single method
func (p *parseContext) processMethod(item *PageNode, method *reflect.Method) error {
	if isComponent(method) {
		if item.Components == nil {
			item.Components = make(map[string]reflect.Method)
		}
		item.Components[method.Name] = *method
		return nil
	}

	if strings.HasSuffix(method.Name, "Props") {
		if item.Props == nil {
			item.Props = make(map[string]reflect.Method)
		}
		item.Props[method.Name] = *method
		return nil
	}

	switch method.Name {
	case "Middlewares":
		item.Middlewares = method
	case "Init":
		return p.callInitMethod(item, method)
	}
	return nil
}

// callInitMethod calls the Init method and handles errors
func (p *parseContext) callInitMethod(item *PageNode, method *reflect.Method) error {
	res, err := p.callMethod(item, method)
	if err != nil {
		return fmt.Errorf("error calling Init method on %s: %w", item.Name, err)
	}
	res, err = extractError(res)
	if err != nil {
		return fmt.Errorf("error calling Init method on %s: %w", item.Name, err)
	}
	_ = res
	return nil
}

// callMethod calls the method with receiver value v and arguments args.
// It uses type matching to fill method parameters from both provided args and p.args registry.
func (p *parseContext) callMethod(
	pn *PageNode, method *reflect.Method, args ...reflect.Value,
) ([]reflect.Value, error) {
	// Prepare receiver
	v, err := p.prepareReceiver(pn.Value, method)
	if err != nil {
		return nil, err
	}

	// Build available arguments map
	availableArgs := p.buildAvailableArgs(pn, args)

	// Prepare method arguments
	in := make([]reflect.Value, method.Type.NumIn())
	in[0] = v // first argument is the receiver

	// Fill remaining arguments
	if err := p.fillMethodArgs(in, method, availableArgs); err != nil {
		return nil, err
	}

	return method.Func.Call(in), nil
}

// prepareReceiver ensures the receiver matches the method's expectations
func (p *parseContext) prepareReceiver(v reflect.Value, method *reflect.Method) (reflect.Value, error) {
	receiver := method.Type.In(0)

	if receiver.Kind() == reflect.Pointer && v.Kind() != reflect.Pointer {
		if !v.CanAddr() {
			return reflect.Value{}, fmt.Errorf("method %s requires pointer receiver but value of type %s is not addressable",
				formatMethod(method), v.Type())
		}
		v = v.Addr()
	}
	if receiver.Kind() != reflect.Pointer && v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if receiver.Kind() != v.Kind() {
		return reflect.Value{}, fmt.Errorf("method %s receiver type mismatch: expected %s, got %s",
			formatMethod(method), receiver.String(), v.Type().String())
	}
	return v, nil
}

// buildAvailableArgs creates a map of available arguments by type
func (p *parseContext) buildAvailableArgs(pn *PageNode, args []reflect.Value) map[reflect.Type][]reflect.Value {
	availableArgs := make(map[reflect.Type][]reflect.Value)

	// Add provided args to available pool
	for _, arg := range args {
		if arg.IsValid() {
			argType := arg.Type()
			availableArgs[argType] = append(availableArgs[argType], arg)
		}
	}

	// Add PageNode as available argument
	pnv := reflect.ValueOf(pn)
	availableArgs[pnv.Type()] = append(availableArgs[pnv.Type()], pnv)
	availableArgs[pnv.Type().Elem()] = append(availableArgs[pnv.Type().Elem()], pnv.Elem())

	return availableArgs
}

// fillMethodArgs fills the method arguments using type matching
func (p *parseContext) fillMethodArgs(
	in []reflect.Value,
	method *reflect.Method,
	availableArgs map[reflect.Type][]reflect.Value,
) error {
	usedArgs := make(map[reflect.Value]bool)

	for i := 1; i < method.Type.NumIn(); i++ {
		argType := method.Type.In(i)

		// Try to find a matching argument
		arg, found := p.findMatchingArg(argType, availableArgs, usedArgs)
		if found {
			in[i] = arg
			continue
		}

		// If not found in available args, try the registry
		val, ok := p.args.getArg(argType)
		if !ok {
			return fmt.Errorf("method %s requires argument of type %s, but not found",
				formatMethod(method), argType.String())
		}
		in[i] = val
	}
	return nil
}

// findMatchingArg tries to find a matching argument from available args
func (p *parseContext) findMatchingArg(
	argType reflect.Type,
	availableArgs map[reflect.Type][]reflect.Value,
	usedArgs map[reflect.Value]bool,
) (reflect.Value, bool) {
	// First try exact type match
	if candidates, ok := availableArgs[argType]; ok {
		for _, candidate := range candidates {
			if !usedArgs[candidate] {
				usedArgs[candidate] = true
				return candidate, true
			}
		}
	}

	// Try assignable types
	for availType, candidates := range availableArgs {
		if availType.AssignableTo(argType) {
			for _, candidate := range candidates {
				if !usedArgs[candidate] {
					usedArgs[candidate] = true
					return candidate, true
				}
			}
		}
	}

	return reflect.Value{}, false
}

func (p *parseContext) callComponentMethod(pn *PageNode, method *reflect.Method,
	args ...reflect.Value,
) (component, error) {
	results, err := p.callMethod(pn, method, args...)
	if err != nil {
		return nil, fmt.Errorf("error calling component method %s: %w", formatMethod(method), err)
	}
	if len(results) != 1 {
		return nil, fmt.Errorf("method %s must return a single result, got %d", formatMethod(method), len(results))
	}
	comp, ok := results[0].Interface().(component)
	if !ok {
		return nil, fmt.Errorf("method %s does not return value of type component", formatMethod(method))
	}
	return comp, nil
}

func (p *parseContext) urlFor(v any) (string, error) {
	node, err := p.findPageNode(v)
	if err != nil {
		return "", err
	}
	return node.FullRoute(), nil
}

func (p *parseContext) findPageNode(v any) (*PageNode, error) {
	if v == nil {
		return nil, fmt.Errorf("URLFor: page argument is nil")
	}

	// Handle Ref type for dynamic page references
	if ref, ok := v.(Ref); ok {
		return p.findPageNodeByRef(string(ref))
	}

	// Handle predicate function for custom matching
	if f, ok := v.(func(*PageNode) bool); ok {
		for node := range p.root.All() {
			if f(node) {
				return node, nil
			}
		}
		return nil, fmt.Errorf("no page matched the provided predicate function")
	}

	// Handle static type reference
	ptv := pointerType(reflect.TypeOf(v))
	var matches []*PageNode
	for node := range p.root.All() {
		pt := pointerType(node.Value.Type())
		if ptv == pt {
			matches = append(matches, node)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no page node found for type %s", ptv.String())
	case 1:
		return matches[0], nil
	default:
		routes := make([]string, len(matches))
		for i, m := range matches {
			routes[i] = m.FullRoute()
		}
		return nil, fmt.Errorf(
			"ambiguous: type %s matches %d nodes: %s; "+
				"disambiguate with []any{ParentType{}, %s{}} chain (recommended), "+
				"Ref(\"Parent.Field\") for cross-package, or a func(*PageNode) bool predicate",
			ptv.String(), len(matches), strings.Join(routes, ", "), ptv.Elem().Name())
	}
}

// resolveParts walks a parsed []any (or a single-element list synthesized
// from a bare page argument) and returns the final URL pattern.
//
// The slice is split into two phases by the first string element:
//
//  1. Chain prefix — every leading non-string element is a chain step.
//     The first step is resolved via findPageNode (accepts any
//     page-identifier form: typed value, Ref, predicate). Each
//     subsequent step must be a typed value, descended from the
//     previous node by child type. The pattern is the last node's
//     FullRoute.
//
//  2. String suffix — once the first string appears, everything from
//     that point on is concatenated to the pattern as a literal URL
//     fragment ("?q={q}", "&extra=1", "/sub/{x}", etc.). Typed values
//     are NOT allowed after a string fragment; this would be ambiguous
//     (is the typed value a chain step continuation or a new lookup?)
//     and we reject it explicitly.
//
// This is the only entry point that knows about the slice form's
// internal grammar. URLFor itself just dispatches here.
func (p *parseContext) resolveParts(parts []any) (string, error) {
	if len(parts) == 0 {
		return "", nil
	}

	// Phase 1: collect chain-step prefix.
	chainEnd := len(parts)
	for i, part := range parts {
		if _, isString := part.(string); isString {
			chainEnd = i
			break
		}
	}
	chain := parts[:chainEnd]
	fragments := parts[chainEnd:]

	var pattern string
	if len(chain) > 0 {
		node, err := p.resolveChain(chain)
		if err != nil {
			return "", err
		}
		// When the chain is the whole specification, resolve a subtree
		// container to its index child so the URL carries the canonical
		// trailing slash. If string fragments follow, the caller is
		// building the path explicitly (e.g. appending "/{$}"), so leave
		// the container's own route untouched to avoid doubling it.
		if len(fragments) == 0 {
			node = node.urlTarget()
		}
		pattern = node.FullRoute()
	}

	// Phase 2: append string fragments. Reject typed values mixed in.
	for i, part := range fragments {
		s, isString := part.(string)
		if !isString {
			return "", fmt.Errorf(
				"URLFor: typed value at slice position %d follows a string fragment; "+
					"chain steps must all come before any string fragment in []any composition",
				chainEnd+i)
		}
		pattern += s
	}
	return pattern, nil
}

// resolveChain resolves a sequence of page identifiers to a single
// PageNode by walking down the page tree. The first step is resolved
// against the whole tree; subsequent steps must be typed values and
// are matched against the previous node's children by type.
func (p *parseContext) resolveChain(steps []any) (*PageNode, error) {
	node, err := p.findPageNode(steps[0])
	if err != nil {
		return nil, err
	}
	for i, step := range steps[1:] {
		// Subsequent chain steps must be plain typed values — Refs
		// and predicates only make sense at the top-level lookup.
		if step == nil {
			return nil, fmt.Errorf("URLFor: chain step %d is nil", i+1)
		}
		if _, isRef := step.(Ref); isRef {
			return nil, fmt.Errorf(
				"URLFor: chain step %d is a Ref; Ref is only valid as the first chain step "+
					"(use Ref(\"Parent.Field\") for qualified lookup instead)",
				i+1)
		}
		if _, isPred := step.(func(*PageNode) bool); isPred {
			return nil, fmt.Errorf(
				"URLFor: chain step %d is a predicate; predicates are only valid as the first chain step",
				i+1)
		}
		next, err := p.descendByType(node, step)
		if err != nil {
			return nil, err
		}
		node = next
	}
	return node, nil
}

// descendByType finds the unique child of parent whose value type
// matches step's pointer-normalised type. Errors with the available
// children listed if zero or more than one match.
func (p *parseContext) descendByType(parent *PageNode, step any) (*PageNode, error) {
	want := pointerType(reflect.TypeOf(step))
	var matches []*PageNode
	for _, c := range parent.Children {
		if pointerType(c.Value.Type()) == want {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		names := make([]string, len(parent.Children))
		for i, c := range parent.Children {
			names[i] = fmt.Sprintf("%s (%s)", c.Name, c.Value.Type().String())
		}
		return nil, fmt.Errorf(
			"URLFor chain: parent %s has no child of type %s; available children: %s",
			parent.Name, want.String(), strings.Join(names, ", "))
	default:
		fields := make([]string, len(matches))
		for i, m := range matches {
			fields[i] = m.Name
		}
		return nil, fmt.Errorf(
			"URLFor chain: parent %s has multiple children of type %s: %s; "+
				"use Ref(%q) with the specific field name to pick one",
			parent.Name, want.String(), strings.Join(fields, ", "),
			parent.Name+"."+fields[0])
	}
}

// findPageNodeByRef finds a page node by name, qualified path, or
// route from a Ref string.
//
// Forms recognised, in order:
//   - "/route" — full route match (e.g. "/components/{slug}")
//   - "Parent.Field" or "Grand.Parent.Field" — qualified path; walks
//     down by PageNode.Name from an anchor segment found at the top
//     level. Use this when the same page name appears under multiple
//     parents and you need to disambiguate, or when the parent's type
//     isn't importable from the caller's package.
//   - "Name" — single name; matches the first node whose Name equals
//     this string (top-down walk order).
func (p *parseContext) findPageNodeByRef(ref string) (*PageNode, error) {
	if strings.HasPrefix(ref, "/") {
		// Match by route
		for node := range p.root.All() {
			if node.FullRoute() == ref {
				return node, nil
			}
		}
		return nil, fmt.Errorf("no page found with route %q", ref)
	}

	if strings.Contains(ref, ".") {
		return p.findPageNodeByQualifiedRef(ref)
	}

	// Match by page name
	for node := range p.root.All() {
		if node.Name == ref {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page found with name %q", ref)
}

// findPageNodeByQualifiedRef resolves a dotted Ref like
// "Parent.Field" or "Grand.Parent.Field" by anchoring on the first
// segment (matched against the root or any top-level child by Name)
// and walking down through subsequent segments by child Name.
//
// Errors list available siblings at the level the walk failed, so
// renaming a field surfaces "available: Index, Detail" instead of a
// silent 404.
func (p *parseContext) findPageNodeByQualifiedRef(ref string) (*PageNode, error) {
	segments := strings.Split(ref, ".")
	if len(segments) == 0 || segments[0] == "" {
		return nil, fmt.Errorf("Ref: empty qualified path %q", ref)
	}

	// Anchor: match the first segment by Name. Prefer a top-level match
	// (the root or its direct children) so existing "Parent.Field" refs
	// resolve exactly as before and "Root.Foo" works explicitly.
	var current *PageNode
	if p.root.Name == segments[0] {
		current = p.root
	} else {
		for _, c := range p.root.Children {
			if c.Name == segments[0] {
				current = c
				break
			}
		}
	}
	// Otherwise accept a uniquely-named anchor anywhere in the tree, so a
	// Ref needn't name the structural wrappers above its target (e.g.
	// "Receptionist.Patients" resolves without the authed-subtree segment).
	// More than one match is ambiguous — error rather than silently pick.
	if current == nil {
		var matches []*PageNode
		for node := range p.root.All() {
			if node.Name == segments[0] {
				matches = append(matches, node)
			}
		}
		switch len(matches) {
		case 1:
			current = matches[0]
		case 0:
			return nil, fmt.Errorf(
				"Ref %q: anchor %q not found in the page tree", ref, segments[0])
		default:
			routes := make([]string, len(matches))
			for i, m := range matches {
				routes[i] = m.FullRoute()
			}
			return nil, fmt.Errorf(
				"Ref %q: anchor %q is ambiguous — it names %d nodes (%s); "+
					"qualify it with a parent segment",
				ref, segments[0], len(matches), strings.Join(routes, ", "))
		}
	}

	// Walk down: each subsequent segment must match a child Name.
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
			return nil, fmt.Errorf(
				"Ref %q: segment %d (%q) not found as child of %q; available children: %s",
				ref, i+1, name, current.Name, strings.Join(childNames, ", "))
		}
		current = next
	}
	return current, nil
}

// getSegmentsCached returns cached segments for a pattern, parsing and caching if not already cached
// Returns a copy of the cached segments to avoid mutation issues
func (p *parseContext) getSegmentsCached(pattern string) ([]segment, error) {
	// Try read lock first for cache hit
	p.segmentCacheMu.RLock()
	if cached, ok := p.segmentCache[pattern]; ok {
		p.segmentCacheMu.RUnlock()
		// Return a copy to avoid mutations affecting the cache
		result := make([]segment, len(cached))
		copy(result, cached)
		return result, nil
	}
	p.segmentCacheMu.RUnlock()

	// Cache miss - parse and store
	segments, err := parseSegments(pattern)
	if err != nil {
		return nil, err
	}

	p.segmentCacheMu.Lock()
	p.segmentCache[pattern] = segments
	p.segmentCacheMu.Unlock()

	// Return a copy to avoid mutations affecting the cache
	result := make([]segment, len(segments))
	copy(result, segments)
	return result, nil
}

func parseTag(route string) (method, path, title string) {
	method = methodAll
	parts := strings.Fields(route)
	if len(parts) == 0 {
		path = "/"
		return
	}
	if len(parts) == 1 {
		path = parts[0]
		return
	}
	method = strings.ToUpper(parts[0])
	if slices.Contains(validMethod, strings.ToUpper(method)) {
		path = parts[1]
		title = strings.Join(parts[2:], " ")
	} else {
		method = methodAll
		path = parts[0]
		title = strings.Join(parts[1:], " ")
	}
	return
}

const methodAll = "ALL"

var validMethod = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	http.MethodOptions,
	http.MethodTrace,
	methodAll,
}
