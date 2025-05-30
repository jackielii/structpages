package srx

import (
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"reflect"
	"runtime"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
)

type StructPages struct {
	onError     func(http.ResponseWriter, *http.Request, error)
	middlewares []func(http.HandlerFunc, *PageNode) http.HandlerFunc
	// preRenderHooks  []func(http.ResponseWriter, *http.Request, pageNode)
	// postRenderHooks []func(http.ResponseWriter, *http.Request, pageNode)
}

func WithErrorHandler(onError func(http.ResponseWriter, *http.Request, error)) func(*StructPages) {
	return func(pr *StructPages) {
		pr.onError = onError
	}
}

func WithMiddlewares(middlewares ...func(http.HandlerFunc, *PageNode) http.HandlerFunc) func(*StructPages) {
	return func(pr *StructPages) {
		pr.middlewares = append(pr.middlewares, middlewares...)
	}
}

// func WithPreRenderHook(hook func(http.ResponseWriter, *http.Request, pageNode)) func(*StructPages) {
// 	return func(pr *StructPages) {
// 		pr.preRenderHooks = append(pr.preRenderHooks, hook)
// 	}
// }
//
// func WithPostRenderHook(hook func(http.ResponseWriter, *http.Request, pageNode)) func(*StructPages) {
// 	return func(pr *StructPages) {
// 		pr.postRenderHooks = append(pr.postRenderHooks, hook)
// 	}
// }

func NewStructPages(options ...func(*StructPages)) *StructPages {
	reg := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("Error rendering page", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
	}
	for _, opt := range options {
		opt(reg)
	}
	return reg
}

type PageNode struct {
	Name     string
	Title    string
	Route    string
	Value    reflect.Value
	Partial  *reflect.Method
	Page     *reflect.Method
	Args     *reflect.Method
	Parent   *PageNode
	Children []*PageNode
}

func (pn *PageNode) FullRoute() string {
	if pn.Parent == nil {
		return pn.Route
	}
	return path.Join(pn.Parent.FullRoute(), pn.Route)
}

func (sp *StructPages) MountPages(router Router, route string, page any, initArgs ...any) {
	pc := parsePageTree(route, page, initArgs...)
	sp.registerPageItem(router, pc, pc.rootNode, route)
}

func sprintMethod(pageMethod *reflect.Method) string {
	if pageMethod == nil {
		return "<nil>"
	}
	return pageMethod.Func.String()
}

func (p PageNode) String() string {
	var sb strings.Builder
	sb.WriteString("PageItem{")
	sb.WriteString("name: " + p.Name)
	sb.WriteString(", title: " + p.Title)
	sb.WriteString(", route: " + p.Route)
	sb.WriteString(", page: " + sprintMethod(p.Page))
	sb.WriteString(", partial: " + sprintMethod(p.Partial))
	sb.WriteString(", args: " + sprintMethod(p.Args))
	sb.WriteString("}")
	return sb.String()
}

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

func (pr *StructPages) registerPageItem(router Router, pc *parseContext, page *PageNode, parentRoute string) {
	if page.Route == "" {
		panic("Page item route is empty: " + page.Name)
	}
	if page.Page == nil && page.Partial == nil {
		router.Route(page.Route, func(router Router) {
			for _, child := range page.Children {
				pr.registerPageItem(router, pc, child, page.Route)
			}
		})
		return
	}
	if page.Page == nil {
		panic("Page item " + page.Name + " does not have a Page method")
	}
	if page.Page.Type.NumIn() > 1 && page.Args == nil {
		panic("Page method on " + page.Name + " requires args, but Args method not declared")
	}
	if page.Partial != nil && page.Partial.Type.NumIn() > 1 && page.Args == nil {
		panic("Partial method on " + page.Name + " requires args, but Args method not declared")
	}
	// TODO: this is problematic as the function itself may panic
	// dry run Args method to make sure the required arguments are available
	// if page.Args != nil {
	// 	emptyRequest, _ := http.NewRequest("GET", "/", nil)
	// 	pc.callMethod(page.Value, *page.Args, reflect.ValueOf(emptyRequest))
	// }

	slog.Info("Registering page item", "name", page.Name, "route", path.Join(parentRoute, page.Route), "title", page.Title)
	handler := func(w http.ResponseWriter, r *http.Request) {
		var args []reflect.Value
		if page.Args != nil {
			args = pc.callMethod(page.Value, *page.Args, reflect.ValueOf(r))
			var err error
			args, err = extractError(args)
			if err != nil {
				pr.onError(w, r, fmt.Errorf("error calling Args method on %s: %w", page.Name, err))
				return
			}
		}

		if htmx.IsHTMX(r) {
			if page.Partial != nil {
				comp := pc.callTemplMethod(page.Value, *page.Partial, args...)
				if err := comp.Render(r.Context(), w); err != nil {
					pr.onError(w, r, err)
				}
			} else {
				comp := pc.callTemplMethod(page.Value, *page.Page, args...)
				if err := htmx.NewResponse().
					Retarget("body").
					RenderTempl(r.Context(), w, comp); err != nil {
					pr.onError(w, r, err)
				}
			}
		} else {
			comp := pc.callTemplMethod(page.Value, *page.Page, args...)
			if err := comp.Render(r.Context(), w); err != nil {
				pr.onError(w, r, err)
			}
		}
	}
	for _, middleware := range pr.middlewares {
		handler = middleware(handler, page)
	}
	router.Get(page.Route, handler)
}

func returnsTemplComponent(t reflect.Method) bool {
	templComponent := reflect.TypeOf((*templ.Component)(nil)).Elem()
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
		panic(fmt.Sprintf("Method %s receiver type mismatch: expected %s, got %s", formatMethod(method), receiver.String(), v.Type().String()))
	}
	if len(args) > method.Type.NumIn()-1 {
		panic(fmt.Sprintf("Method %s expects at most %d arguments, but got %d", formatMethod(method), method.Type.NumIn()-1, len(args)))
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
			panic(fmt.Sprintf("Method %s requires argument of type %s, but no initArgs provided", formatMethod(method), st.String()))
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

func (p *parseContext) callTemplMethod(v reflect.Value, method reflect.Method, args ...reflect.Value) templ.Component {
	results := p.callMethod(v, method, args...)
	if len(results) != 1 {
		panic("Method " + method.Name + " must return a single templ.Component")
	}
	comp, ok := results[0].Interface().(templ.Component)
	if !ok {
		panic("Method " + method.Name + " does not return a templ.Component")
	}
	return comp
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func extractError(args []reflect.Value) ([]reflect.Value, error) {
	var err error
	if len(args) >= 1 && args[len(args)-1].Type().AssignableTo(errorType) {
		i := args[len(args)-1].Interface()
		args = args[:len(args)-1]
		if i == nil {
			return args, nil
		}
		err = i.(error)
	}
	return args, err
}

func formatMethod(method reflect.Method) string {
	if method.Func == (reflect.Value{}) {
		return "<nil>"
	}
	return fmt.Sprintf("%s.%s", method.Type.In(0).String(), method.Name)
}
