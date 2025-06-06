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

func parsePageTree(route string, page any, initArgs ...any) *parseContext {
	pc := &parseContext{args: make(map[reflect.Type]reflect.Value)}
	for _, v := range initArgs {
		pc.args.addArg(v)
	}
	topNode := pc.parsePageTree(route, "", page)
	pc.root = topNode
	return pc
}

func (p *parseContext) parsePageTree(route string, fieldName string, page any) *PageNode {
	st := reflect.TypeOf(page) // struct type
	pt := reflect.TypeOf(page) // pointer type
	if st.Kind() == reflect.Ptr {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	item := &PageNode{Value: reflect.ValueOf(page), Name: cmp.Or(fieldName, st.Name())}
	item.Method, item.Route, item.Title = parseTag(route)

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
		childItem := p.parsePageTree(route, field.Name, childPage.Interface())
		childItem.Parent = item

		item.Children = append(item.Children, childItem)
	}

	// log.Printf("Parsing page item: %s, route: %s, NumMethod: %d", item.name, item.route, st.NumMethod())
	for _, t := range []reflect.Type{st, pt} {
		for i := range t.NumMethod() {
			method := t.Method(i)
			if isPromotedMethod(method) {
				continue // skip promoted methods
			}
			// log.Printf("  Method: %s, NumIn: %d, NumOut: %d", method.Name, method.Type.NumIn(), method.Type.NumOut())
			// for j := range method.Type.NumIn() {
			// 	log.Printf("    In[%d]: %s", j, method.Type.In(j).String())
			// }
			if isComponent(method) {
				if item.Components == nil {
					item.Components = make(map[string]reflect.Method)
				}
				item.Components[method.Name] = method
				continue
			}
			if strings.HasSuffix(method.Name, "Props") {
				if item.Props == nil {
					item.Props = make(map[string]reflect.Method)
				}
				item.Props[method.Name] = method
				continue
			}

			switch method.Name {
			case "PageConfig":
				item.Config = &method
			case "Middlewares":
				item.Middlewares = &method
			case "Init":
				res := p.callMethod(item.Value, method)
				res, err := extractError(res)
				if err != nil {
					panic(fmt.Sprintf("Error calling Init method on %s: %v", item.Name, err))
				}
				_ = res
			}
		}
	}

	return item
}

// callMethod calls the emthod with receiver value v and arguments args.
// it uses types from p.args to fill in missing arguments.
func (p *parseContext) callMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) []reflect.Value {
	receiver := method.Type.In(0)
	// make sure receiver and value match, if method takes a pointer, convert value to pointer
	if receiver.Kind() == reflect.Ptr && v.Kind() != reflect.Ptr {
		v = v.Addr()
	}
	if receiver.Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if receiver.Kind() != v.Kind() {
		panic(fmt.Sprintf("Method %s receiver type mismatch: expected %s, got %s", formatMethod(&method), receiver.String(), v.Type().String()))
	}
	// we allow calling methods with fewer arguments than defined
	// if len(args) > method.Type.NumIn()-1 {
	// 	panic(fmt.Sprintf("Method %s expects at most %d arguments, but got %d", formatMethod(&method), method.Type.NumIn()-1, len(args)))
	// }
	in := make([]reflect.Value, method.Type.NumIn())
	in[0] = v // first argument is the receiver
	lenFilled := 1
	for i := range min(len(in)-1, len(args)) {
		in[i+1] = args[i]
		lenFilled++
	}
	if len(in) <= lenFilled {
		return method.Func.Call(in)
	}
	// convention: if a method has more arguments than provided, we try to fill them with initArgs
	for i := lenFilled; i < len(in); i++ {
		argType := method.Type.In(i)
		val, ok := p.args.getArg(argType)
		if !ok {
			panic(fmt.Sprintf("Method %s requires argument of type %s, but not found", formatMethod(&method), argType.String()))
		}
		in[i] = val
	}
	return method.Func.Call(in)
}

func (p *parseContext) callComponentMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) component {
	results := p.callMethod(v, method, args...)
	if len(results) != 1 {
		panic("Method " + method.Name + " must return a single result")
	}
	comp, ok := results[0].Interface().(component)
	if !ok {
		panic("Method " + method.Name + " does not return value of type component")
	}
	return comp
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

func parseTag(route string) (method string, path string, title string) {
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

func isComponent(t reflect.Method) bool {
	typ := reflect.TypeOf((*component)(nil)).Elem()
	if t.Type.NumOut() != 1 {
		return false
	}
	return t.Type.Out(0).Implements(typ)
}

func isPromotedMethod(method reflect.Method) bool {
	// Check if the method is promoted from an embedded type
	// https://github.com/golang/go/issues/73883
	wPC := method.Func.Pointer()
	wFunc := runtime.FuncForPC(wPC)
	wFile, wLine := wFunc.FileLine(wPC)
	return wFile == "<autogenerated>" && wLine == 1
}

type argRegistry map[reflect.Type]reflect.Value

func (args argRegistry) addArg(v any) {
	if v == nil {
		return
	}
	typ := reflect.TypeOf(v)
	pv := reflect.ValueOf(v)
	// TODO: what do we do if types conflict?
	args[typ] = pv
}

// note that p.args are always pointers
func (args argRegistry) getArg(pt reflect.Type) (reflect.Value, bool) {
	st := pt
	needsElem, needsPtr := false, false
	if pt.Kind() != reflect.Ptr {
		needsElem = true
		pt = reflect.PointerTo(pt)
	}
	if st.Kind() == reflect.Ptr {
		needsPtr = true
		st = st.Elem()
	}

	if v, ok := args[pt]; ok {
		if needsElem {
			return v.Elem(), true
		}
		return v, true
	}

	if v, ok := args[st]; ok {
		if needsPtr {
			if !v.CanAddr() {
				// TODO: some values are not addressable
			}
			return v.Addr(), true
		}
		return v, true
	}

	for t, v := range args {
		if pt.AssignableTo(t) {
			if needsPtr {
				return v.Addr(), true
			}
			return v, true
		}
		if st.AssignableTo(t) {
			if needsElem {
				return v.Elem(), true
			}
			return v, true
		}
	}

	return reflect.Value{}, false
}
