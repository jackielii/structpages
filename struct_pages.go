package srx

import (
	"fmt"
	"net/http"
	"reflect"
)

type middlewareFunc = func(http.Handler, *PageNode) http.Handler

type StructPages struct {
	onError     func(http.ResponseWriter, *http.Request, error)
	middlewares []middlewareFunc
}

func NewStructPages(options ...func(*StructPages)) *StructPages {
	reg := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
	}
	for _, opt := range options {
		opt(reg)
	}
	return reg
}

func WithErrorHandler(onError func(http.ResponseWriter, *http.Request, error)) func(*StructPages) {
	return func(pr *StructPages) {
		pr.onError = onError
	}
}

func WithMiddlewares(middlewares ...func(http.Handler, *PageNode) http.Handler) func(*StructPages) {
	return func(pr *StructPages) {
		pr.middlewares = append(pr.middlewares, middlewares...)
	}
}

func (sp *StructPages) MountPages(router Router, route string, page any, initArgs ...any) {
	pc := parsePageTree(route, page, initArgs...)
	sp.registerPageItem(router, pc, pc.rootNode)
}

func (pr *StructPages) registerPageItem(router Router, pc *parseContext, page *PageNode) {
	if page.Route == "" {
		panic("Page item route is empty: " + page.Name)
	}
	if page.Children != nil {
		// nested pages has to be registered first to avoid conflicts with the parent route
		// defer func() {
		// println("Registering route group", "name", page.Name, "route", page.FullRoute())
		router.Route(page.Route, func(router Router) {
			for _, child := range page.Children {
				pr.registerPageItem(router, pc, child)
			}
		})
		// }()
	}
	// println("Registering page item", "name:", page.Name, page.Method, page.FullRoute(), "title:", page.Title)
	handler := pr.buildHandler(page, pc)
	if handler == nil {
		return
	}
	// apply page middlewares
	if page.Middlewares != nil {
		// TODO: should apply parent middlewares first, probably passed down from the page node
		res := pc.callMethod(page.Value, *page.Middlewares, reflect.ValueOf(page))
		if len(res) != 1 {
			panic(fmt.Sprintf("Middlewares method on %s did not return single result", page.Name))
		}
		middlewares, ok := res[0].Interface().([]middlewareFunc)
		if !ok {
			panic(fmt.Sprintf("Middlewares method on %s did not return []func(http.Handler, *PageNode) http.Handler", page.Name))
		}
		for _, mw := range middlewares {
			handler = mw(handler, page)
		}
	}
	// apply global middlewares
	for _, middleware := range pr.middlewares {
		handler = middleware(handler, page)
	}
	router.HandleMethod(page.Method, page.Route, handler)
}

func (sp *StructPages) buildHandler(page *PageNode, pc *parseContext) http.Handler {
	if h := sp.getHttpHandler(page.Value); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}
	pageComp := page.Components["Page"]
	partialComp := page.Components["Partial"]
	if pageComp == nil {
		panic(fmt.Sprintf("Page item %s does not have a Page component", page.Name))
	}
	// if pageComp == nil {
	// 	panic("Page item " + page.Name + " does not have a Page method")
	// }
	// if pageComp.Type.NumIn() > 1 && page.Args == nil {
	// 	panic("Page method on " + page.Name + " requires args, but Args method not declared")
	// }
	// if partialComp != nil && partialComp.Type.NumIn() > 1 && page.Args == nil {
	// 	panic("Partial method on " + page.Name + " requires args, but Args method not declared")
	// }

	// TODO: move handler creation to configuration
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []reflect.Value
		if page.Args != nil {
			args = pc.callMethod(page.Value, *page.Args, reflect.ValueOf(r))
			var err error
			args, err = extractError(args)
			if err != nil {
				sp.onError(w, r, fmt.Errorf("error calling Args method on %s: %w", page.Name, err))
				return
			}
		}

		if isHTMX(r) {
			if partialComp != nil {
				comp := pc.callTemplMethod(page.Value, *partialComp, args...)
				if err := comp.Render(r.Context(), w); err != nil {
					sp.onError(w, r, err)
				}
			} else {
				comp := pc.callTemplMethod(page.Value, *pageComp, args...)
				w.Header().Set("HX-Retarget", "body")
				if err := comp.Render(r.Context(), w); err != nil {
					sp.onError(w, r, err)
				}
			}
		} else {
			comp := pc.callTemplMethod(page.Value, *pageComp, args...)
			if err := comp.Render(r.Context(), w); err != nil {
				sp.onError(w, r, err)
			}
		}
	})
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

func formatMethod(method *reflect.Method) string {
	if method == nil || method.Func == (reflect.Value{}) {
		return "<nil>"
	}
	return fmt.Sprintf("%s.%s", method.Type.In(0).String(), method.Name)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (pr *StructPages) getHttpHandler(v reflect.Value) http.Handler {
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
			if err := h.ServeHTTP(w, r); err != nil {
				pr.onError(w, r, err)
			}
		})
	}
	return nil
}
