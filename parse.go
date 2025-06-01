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

	// args hold type vs values
	// type is always a pointer type
	// value is always a pointer to the value
	args map[reflect.Type]reflect.Value
}

func parsePageTree(route string, page any, initArgs ...any) *parseContext {
	pc := &parseContext{args: make(map[reflect.Type]reflect.Value)}
	for _, v := range initArgs {
		pc.addValue(v)
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
					item.Components = make(map[string]*reflect.Method)
				}
				item.Components[method.Name] = &method
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
			case "Props":
				item.Props = &method
			}
		}
	}

	return item
}

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
		st := argType
		pt := argType
		needPtr := false
		if argType.Kind() == reflect.Ptr {
			needPtr = true
			st = st.Elem()
		} else {
			pt = reflect.PointerTo(st)
		}
		pval, pok := p.args[pt]
		if !pok {
			panic(fmt.Sprintf("Method %s requires argument of type %s, but no initArgs provided", formatMethod(&method), st.String()))
		}
		var val reflect.Value
		if !needPtr {
			val = pval.Elem()
		} else {
			val = pval
		}
		in[i] = val
	}
	return method.Func.Call(in)
}

func (p *parseContext) callComponentMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) component {
	results := p.callMethod(v, method, args...)
	if len(results) != 1 {
		panic("Method " + method.Name + " must return a single templ.Component")
	}
	comp, ok := results[0].Interface().(component)
	if !ok {
		panic("Method " + method.Name + " does not return a templ.Component")
	}
	return comp
}

func (p *parseContext) addValue(v any) {
	if v == nil {
		return
	}
	typ := reflect.TypeOf(v)
	pv := reflect.ValueOf(v)
	if typ.Kind() != reflect.Ptr {
		typ = reflect.PointerTo(typ)
		pv = pv.Addr()
	}
	if _, ok := p.args[typ]; !ok {
		p.args[typ] = pv
	}
}

func (p *parseContext) urlFor(v any) (string, error) {
	if f, ok := v.(func(*PageNode) bool); ok {
		for node := range p.root.All() {
			if f(node) {
				return node.FullRoute(), nil
			}
		}
	}
	pt := reflect.TypeOf(v)
	if pt.Kind() != reflect.Ptr {
		pt = reflect.PointerTo(pt)
	}
	for node := range p.root.All() {
		if node.Value.Type() == pt {
			return node.FullRoute(), nil
		}
	}
	return "", fmt.Errorf("urlfor: no page node found for %s", pt.String())
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
	templComponent := reflect.TypeOf((*component)(nil)).Elem()
	if t.Type.NumOut() != 1 {
		return false
	}
	return t.Type.Out(0).Implements(templComponent)
}

func isPromotedMethod(method reflect.Method) bool {
	// Check if the method is promoted from an embedded type
	// https://github.com/golang/go/issues/73883
	wPC := method.Func.Pointer()
	wFunc := runtime.FuncForPC(wPC)
	wFile, wLine := wFunc.FileLine(wPC)
	return wFile == "<autogenerated>" && wLine == 1
}
