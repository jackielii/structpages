package structpages

import (
	"fmt"
	"net/http"
	"reflect"
	"slices"
)

type MiddlewareFunc func(http.Handler, *PageNode) http.Handler

type StructPages struct {
	onError           func(http.ResponseWriter, *http.Request, error)
	middlewares       []MiddlewareFunc
	defaultPageConfig func(r *http.Request) (string, error)
}

func New(options ...func(*StructPages)) *StructPages {
	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
	}
	for _, opt := range options {
		opt(sp)
	}
	return sp
}

func WithDefaultPageConfig(configFunc func(r *http.Request) (string, error)) func(*StructPages) {
	return func(sp *StructPages) {
		sp.defaultPageConfig = configFunc
	}
}

func WithErrorHandler(onError func(http.ResponseWriter, *http.Request, error)) func(*StructPages) {
	return func(sp *StructPages) {
		sp.onError = onError
	}
}

func WithMiddlewares(middlewares ...MiddlewareFunc) func(*StructPages) {
	return func(sp *StructPages) {
		sp.middlewares = append(sp.middlewares, middlewares...)
	}
}

func (sp *StructPages) MountPages(router Router, page any, route, title string, args ...any) {
	pc := parsePageTree(route, page, args...)
	pc.root.Title = title
	middlewares := append([]MiddlewareFunc{withPcCtx(pc)}, sp.middlewares...)
	sp.registerPageItem(router, pc, pc.root, middlewares)
}

func (sp *StructPages) registerPageItem(router Router, pc *parseContext, page *PageNode, middlewares []MiddlewareFunc) {
	if page.Route == "" {
		panic("Page item route is empty: " + page.Name)
	}
	if page.Middlewares != nil {
		res, err := pc.callMethod(page, *page.Middlewares)
		if err != nil {
			panic(fmt.Errorf("error calling Middlewares method on %s: %w", page.Name, err))
		}
		if len(res) != 1 {
			panic(fmt.Errorf("middlewares method on %s did not return single result", page.Name))
		}
		mws, ok := res[0].Interface().([]MiddlewareFunc)
		if !ok {
			panic(fmt.Errorf("middlewares method on %s did not return []func(http.Handler, *PageNode) http.Handler", page.Name))
		}
		middlewares = append(middlewares, mws...)
	}
	if page.Children != nil {
		// nested pages has to be registered first to avoid conflicts with the parent route
		// defer func() {
		// println("Registering route group", "name", page.Name, "route", page.FullRoute())
		router.Route(page.Route, func(router Router) {
			for _, child := range page.Children {
				sp.registerPageItem(router, pc, child, middlewares)
			}
		})
		// }()
	}
	// println("Registering page item", "name:", page.Name, page.Method, page.FullRoute(), "title:", page.Title)
	handler := sp.buildHandler(page, pc)
	if handler == nil {
		if len(page.Children) == 0 {
			// when handdler is nil and no children, it means this page is not a valid endpoint
			panic(fmt.Errorf("page item %s does not have a valid handler or children", page.Name))
		}
		return
	}
	for _, middleware := range slices.Backward(middlewares) {
		handler = middleware(handler, page)
	}
	router.HandleMethod(page.Method, page.Route, handler)
}

func (sp *StructPages) buildHandler(page *PageNode, pc *parseContext) http.Handler {
	if h := sp.asHandler(pc, page); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compMethod, err := sp.findComponent(pc, page, r)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error calling PageConfig method on %s: %w", page.Name, err))
			return
		}

		props, err := sp.getProps(pc, page, compMethod, r)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error calling props component %s.%s: %w", page.Name, compMethod.Name, err))
			return
		}

		if !compMethod.Func.IsValid() {
			sp.onError(w, r, fmt.Errorf("page %s does not have a Page or PageConfig method", page.Name))
			return
		}

		comp, err := pc.callComponentMethod(page, compMethod, props...)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, compMethod.Name, err))
			return
		}
		sp.render(w, r, comp)
	})
}

func (sp *StructPages) render(w http.ResponseWriter, r *http.Request, comp component) {
	buf := getBuffer()
	defer releaseBuffer(buf)
	if err := comp.Render(r.Context(), buf); err != nil {
		sp.onError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

type httpErrHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

var (
	errorType      = reflect.TypeOf((*error)(nil)).Elem()
	handlerType    = reflect.TypeOf((*http.Handler)(nil)).Elem()
	errHandlerType = reflect.TypeOf((*httpErrHandler)(nil)).Elem()
)

func extractError(args []reflect.Value) ([]reflect.Value, error) {
	if len(args) >= 1 && args[len(args)-1].Type().AssignableTo(errorType) {
		i := args[len(args)-1].Interface()
		args = args[:len(args)-1]
		if i == nil {
			return args, nil
		}
		return args, i.(error)
	}
	return args, nil
}

func formatMethod(method *reflect.Method) string {
	if method == nil || !method.Func.IsValid() {
		return "<nil>"
	}
	receiver := method.Type.In(0)
	if receiver.Kind() == reflect.Ptr {
		receiver = receiver.Elem()
	}
	return fmt.Sprintf("%s.%s", receiver.String(), method.Name)
}

func (sp *StructPages) asHandler(pc *parseContext, pn *PageNode) http.Handler {
	v := pn.Value
	st, pt := v.Type(), v.Type()
	if st.Kind() == reflect.Ptr {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	method, ok := st.MethodByName("ServeHTTP")
	if !ok || isPromotedMethod(method) {
		method, ok = pt.MethodByName("ServeHTTP")
		if !ok || isPromotedMethod(method) {
			return nil
		}
	}

	if v.Type().Implements(handlerType) {
		// println(v.Type().String(), "implements ServeHTTP:", ok, "returning handler")
		return v.Interface().(http.Handler)
	}
	if v.Type().Implements(errHandlerType) {
		h := v.Interface().(httpErrHandler)
		// println(v.Type().String(), "implements ServeHTTP:", ok, "returning err handler")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// because we have to handle errors, and error handler could write header
			// potentially we want to clear the buffer writer
			bw := newBuffered(w)
			defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
			if err := h.ServeHTTP(bw, r); err != nil {
				sp.onError(w, r, err)
			}
		})
	}
	// extended ServeHTTP method with extra arguments
	if method.Type.NumIn() > 3 { // receiver, http.ResponseWriter, *http.Request
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var wv reflect.Value // ResponseWriter, will be buffered if handler returns error
			if method.Type.NumOut() > 1 {
				// same logic: if it returns value, we need to buffer it
				bw := newBuffered(w)
				defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
				wv = reflect.ValueOf(bw)
			} else {
				wv = reflect.ValueOf(w)
			}
			results, err := pc.callMethod(pn, method, wv, reflect.ValueOf(r))
			if err != nil {
				sp.onError(w, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				return
			}
			_, err = extractError(results)
			if err != nil {
				sp.onError(w, r, err)
				return
			}
		})
	}

	return nil
}

func (sp *StructPages) findComponent(pc *parseContext, pn *PageNode, r *http.Request) (reflect.Method, error) {
	if pn.Config != nil {
		res, err := pc.callMethod(pn, *pn.Config, reflect.ValueOf(r))
		if err != nil {
			return reflect.Method{}, fmt.Errorf("error calling PageConfig method for %s: %w", pn.Name, err)
		}
		res, err = extractError(res)
		if err != nil {
			return reflect.Method{}, fmt.Errorf("error calling PageConfig method for %s: %w", pn.Name, err)
		}
		if len(res) >= 1 && res[0].Type().Kind() == reflect.String {
			name := res[0].String()
			if comp, ok := pn.Components[name]; ok {
				return comp, nil
			}
			return reflect.Method{}, fmt.Errorf("PageConfig method for %s returned unknown component name: %s", pn.Name, name)
		}
	}
	if sp.defaultPageConfig != nil {
		name, err := sp.defaultPageConfig(r)
		if err != nil {
			return reflect.Method{}, fmt.Errorf("error calling default page config for %s: %w", pn.Name, err)
		}
		page, ok := pn.Components[name]
		if !ok {
			return reflect.Method{}, fmt.Errorf("default PageConfig for %s returned unknown component name: %s", pn.Name, name)
		}
		return page, nil
	}
	page, ok := pn.Components["Page"]
	if !ok {
		return reflect.Method{}, fmt.Errorf("no Page component or PageConfig method found for %s", pn.Name)
	}
	return page, nil
}

func (sp *StructPages) getProps(pc *parseContext, pn *PageNode, compMethod reflect.Method,
	r *http.Request) ([]reflect.Value, error) {
	pageName := compMethod.Name
	var propMethod reflect.Method
	for _, name := range []string{pageName + "Props", "Props"} {
		if pm, ok := pn.Props[name]; ok {
			propMethod = pm
			break
		}
	}
	if propMethod.Func.IsValid() {
		props, err := pc.callMethod(pn, propMethod, reflect.ValueOf(r))
		if err != nil {
			return nil, fmt.Errorf("error calling props method %s.%s: %w", pn.Name, propMethod.Name, err)
		}
		return extractError(props)
	}
	return nil, nil
}
