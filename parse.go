package structpages

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
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
	if st.Kind() == reflect.Ptr {
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
		if typ.Kind() == reflect.Ptr {
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
	case "PageConfig":
		item.Config = method
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

// callMethod calls the emthod with receiver value v and arguments args.
// it uses types from p.args to fill in missing arguments.
func (p *parseContext) callMethod(pn *PageNode, method *reflect.Method,
	args ...reflect.Value,
) ([]reflect.Value, error) {
	v := pn.Value
	receiver := method.Type.In(0)
	// make sure receiver and value match, if method takes a pointer, convert value to pointer
	if receiver.Kind() == reflect.Ptr && v.Kind() != reflect.Ptr {
		if !v.CanAddr() {
			return nil, fmt.Errorf("method %s requires pointer receiver but value of type %s is not addressable",
				formatMethod(method), v.Type())
		}
		v = v.Addr()
	}
	if receiver.Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if receiver.Kind() != v.Kind() {
		return nil, fmt.Errorf("method %s receiver type mismatch: expected %s, got %s",
			formatMethod(method), receiver.String(), v.Type().String())
	}
	// we allow calling methods with fewer arguments than defined
	// if len(args) > method.Type.NumIn()-1 {
	// 	panic(fmt.Sprintf("Method %s expects at most %d arguments, but got %d",
	// 		formatMethod(&method), method.Type.NumIn()-1, len(args)))
	// }
	in := make([]reflect.Value, method.Type.NumIn())
	in[0] = v // first argument is the receiver
	lenFilled := 1
	for i := range min(len(in)-1, len(args)) {
		expectedType := method.Type.In(i + 1)
		argValue := args[i]
		// Check if the argument type is compatible
		if argValue.IsValid() && !argValue.Type().AssignableTo(expectedType) {
			return nil, fmt.Errorf("argument %d: cannot use %v as %v", i+1, argValue.Type(), expectedType)
		}
		in[i+1] = argValue
		lenFilled++
	}
	if len(in) <= lenFilled {
		return method.Func.Call(in), nil
	}
	pnv := reflect.ValueOf(pn)
	// if we still have arguments to fill, we'll provide the following:
	// - if the argument is of type *PageNode or PageNode, use the current node
	// TODO: explore the use case where a struct page is required
	for i := lenFilled; i < len(in); i++ {
		argType := method.Type.In(i)
		switch {
		case argType == pnv.Type():
			in[i] = pnv // if the argument is of type *PageNode, use the current node
		case argType == pnv.Type().Elem():
			in[i] = pnv.Elem()
		default:
			val, ok := p.args.getArg(argType)
			if !ok {
				return nil, fmt.Errorf("method %s requires argument of type %s, but not found",
					formatMethod(method), argType.String())
			}
			in[i] = val
		}
		lenFilled++
	}
	return method.Func.Call(in), nil
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
	if f, ok := v.(func(*PageNode) bool); ok {
		for node := range p.root.All() {
			if f(node) {
				return node.FullRoute(), nil
			}
		}
	}
	ptv := pointerType(reflect.TypeOf(v))
	for node := range p.root.All() {
		pt := pointerType(node.Value.Type())
		if ptv == pt {
			return node.FullRoute(), nil
		}
	}
	return "", fmt.Errorf("urlfor: no page node found for %s", ptv.String())
}

func pointerType(v reflect.Type) reflect.Type {
	if v.Kind() == reflect.Ptr {
		return v
	}
	return reflect.PointerTo(v)
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

type component interface {
	Render(context.Context, io.Writer) error
}

func isComponent(t *reflect.Method) bool {
	typ := reflect.TypeOf((*component)(nil)).Elem()
	if t.Type.NumOut() != 1 {
		return false
	}
	return t.Type.Out(0).Implements(typ)
}

func isPromotedMethod(method *reflect.Method) bool {
	// Check if the method is promoted from an embedded type
	// https://github.com/golang/go/issues/73883
	wPC := method.Func.Pointer()
	wFunc := runtime.FuncForPC(wPC)
	wFile, wLine := wFunc.FileLine(wPC)
	return wFile == "<autogenerated>" && wLine == 1
}
