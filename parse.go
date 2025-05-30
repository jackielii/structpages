package srx

import (
	"fmt"
	"reflect"
)

func parsePageTree(route string, page any, initArgs ...any) *parseContext {
	args := make(map[reflect.Type]reflect.Value)
	for _, arg := range initArgs {
		if arg == nil {
			continue
		}
		typ := reflect.TypeOf(arg)
		val := reflect.ValueOf(arg)
		args[typ] = val
	}
	pc := &parseContext{initArgs: args}
	topNode := pc.parsePageTree(route, "", page)
	pc.rootNode = topNode
	return pc
}

type parseContext struct {
	rootNode *PageNode
	initArgs map[reflect.Type]reflect.Value
}

func (p *parseContext) parsePageTree(route string, fieldName string, page any) *PageNode {
	st := reflect.TypeOf(page) // struct type
	pt := reflect.TypeOf(page) // pointer type
	if st.Kind() == reflect.Ptr {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	name := fieldName
	if name == "" {
		name = st.Name()
	}

	item := &PageNode{Value: reflect.ValueOf(page), Route: route, Name: name}

	for i := range st.NumField() {
		field := st.Field(i)
		route := field.Tag.Get("route")
		if route != "" {
			typ := field.Type
			if typ.Kind() == reflect.Ptr {
				typ = typ.Elem()
			}
			childPage := reflect.New(typ)
			childItem := p.parsePageTree(route, field.Name, childPage.Interface())

			title := field.Tag.Get("title")
			if title != "" {
				childItem.Title = title
			}
			childItem.Parent = item

			item.Children = append(item.Children, childItem)
		}
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
			switch method.Name {
			case "Init":
				res := p.callMethod(item.Value, method)
				res, err := extractError(res)
				if err != nil {
					panic(fmt.Sprintf("Error calling Init method on %s: %v", item.Name, err))
				}
				_ = res
			case "Page":
				item.Page = &method
				if !returnsTemplComponent(method) {
					panic("Page Method " + t.String() + " does not return a templ.Component")
				}
			case "Partial":
				item.Partial = &method
				if !returnsTemplComponent(method) {
					panic("Partial Method " + t.String() + " does not return a templ.Component")
				}
			case "Args":
				item.Args = &method
			}
		}
	}

	return item
}

func (p *parseContext) callMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) []reflect.Value {
	receiver := method.Type.In(0)
	// make sure receiver and value match, if method takes a pointer, convert value to pointer
	if receiver.Kind() == reflect.Ptr && v.Kind() != reflect.Ptr {
		// if !v.CanAddr() {
		// 	spew.Dump(v.Interface())
		// }
		v = v.Addr()
	}
	if receiver.Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if receiver.Kind() != v.Kind() {
		panic(fmt.Sprintf("Method %s receiver type mismatch: expected %s, got %s", formatMethod(&method), receiver.String(), v.Type().String()))
	}
	if len(args) > method.Type.NumIn()-1 {
		panic(fmt.Sprintf("Method %s expects at most %d arguments, but got %d", formatMethod(&method), method.Type.NumIn()-1, len(args)))
	}
	in := make([]reflect.Value, method.Type.NumIn())
	in[0] = v // first argument is the receiver
	for i, arg := range args {
		in[i+1] = arg
	}
	lenFilled := len(args) + 1
	if len(in) == lenFilled {
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
		sval, sok := p.initArgs[st]
		pval, pok := p.initArgs[pt]
		if !sok && !pok {
			panic(fmt.Sprintf("Method %s requires argument of type %s, but no initArgs provided", formatMethod(&method), st.String()))
		}
		var val reflect.Value
		if sok && needPtr {
			val = sval.Addr()
		} else if sok && !needPtr {
			val = sval
		} else if pok && needPtr {
			val = pval
		} else {
			val = pval.Elem()
		}
		in[i] = val
	}
	return method.Func.Call(in)
}

func (p *parseContext) callTemplMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) templComponent {
	results := p.callMethod(v, method, args...)
	if len(results) != 1 {
		panic("Method " + method.Name + " must return a single templ.Component")
	}
	comp, ok := results[0].Interface().(templComponent)
	if !ok {
		panic("Method " + method.Name + " does not return a templ.Component")
	}
	return comp
}
