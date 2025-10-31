package structpages

import (
	"cmp"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
)

type parseContext struct {
	root *PageNode
	args argRegistry
}

func parsePageTree(route string, page any, args ...any) (*parseContext, error) {
	pc := &parseContext{args: make(map[reflect.Type]reflect.Value)}
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
	for node := range p.root.All() {
		pt := pointerType(node.Value.Type())
		if ptv == pt {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page node found for type %s", ptv.String())
}

// findPageNodeByRef finds a page node by name or route from a Ref string.
// If the ref starts with "/", it matches by route; otherwise by page name.
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

	// Match by page name
	for node := range p.root.All() {
		if node.Name == ref {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no page found with name %q", ref)
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
