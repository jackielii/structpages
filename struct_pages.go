package structpages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
)

// ErrSkipPageRender is a sentinel error that can be returned from a Props method
// to indicate that the page rendering should be skipped. This is useful for
// implementing conditional rendering or redirects within page logic.
var ErrSkipPageRender = errors.New("skip page render")

// MiddlewareFunc is a function that wraps an http.Handler with additional functionality.
// It receives both the handler to wrap and the PageNode being handled, allowing middleware
// to access page metadata like route, title, and other properties.
type MiddlewareFunc func(http.Handler, *PageNode) http.Handler

// Mux represents any HTTP router that can register handlers using the Handle method.
// This interface is satisfied by http.ServeMux and must follow the same pattern support
// for route registration.
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// StructPages holds the parsed page tree context for URL generation.
// It is returned by Mount and provides URLFor and IDFor methods.
type StructPages struct {
	pc             *parseContext
	onError        func(http.ResponseWriter, *http.Request, error)
	middlewares    []MiddlewareFunc
	targetSelector TargetSelector
	warnEmptyRoute func(*PageNode)
	args           []any
}

// ID generates a raw HTML ID for a component method (without "#" prefix).
// Use this for HTML id attributes.
// It works without context by using the structpages's parseContext directly.
//
// Example:
//
//	sp.ID(p.UserList)
//	// → "team-management-view-user-list"
//
//	sp.ID(UserStatsWidget)
//	// → "user-stats-widget" (no page prefix for standalone functions)
func (sp *StructPages) ID(v any) (string, error) {
	return idFor(sp.pc, v, true)
}

// IDTarget generates a CSS selector (with "#" prefix) for a component method.
// Use this for HTMX hx-target attributes.
// It works without context by using the structpages's parseContext directly.
//
// Example:
//
//	sp.IDTarget(p.UserList)
//	// → "#team-management-view-user-list"
//
//	sp.IDTarget(UserStatsWidget)
//	// → "#user-stats-widget" (no page prefix for standalone functions)
func (sp *StructPages) IDTarget(v any) (string, error) {
	return idFor(sp.pc, v, false)
}

// URLFor returns the URL for a given page type. If args is provided, it'll replace
// the path segments. Supported format is similar to http.ServeMux.
//
// Unlike the context-based URLFor function, this method doesn't have access to
// pre-extracted URL parameters from the current request, so all required parameters
// must be provided as args.
//
// If multiple page type matches are found, the first one is returned.
// In such situation, use a func(*PageNode) bool as page argument to match a specific page.
//
// Additionally, you can pass []any to page to join multiple path segments together.
// Strings will be joined as is. Example:
//
//	sp.URLFor([]any{Page{}, "?foo={bar}"}, "bar", "baz")
//
// It also supports a func(*PageNode) bool as the Page argument to match a specific page.
// It can be useful when you have multiple pages with the same type but different routes.
func (sp *StructPages) URLFor(page any, args ...any) (string, error) {
	// Create a context with parseContext and call the context-based URLFor
	ctx := pcCtx.WithValue(context.Background(), sp.pc)
	return URLFor(ctx, page, args...)
}

// Option represents a configuration option for StructPages.
type Option func(*StructPages)

// Mount parses the page tree and registers all routes onto the provided mux.
// If mux is nil, routes are registered on http.DefaultServeMux.
// Returns a StructPages that provides URLFor and IDFor methods.
//
// Parameters:
//   - mux: Any router satisfying the Mux interface (e.g., http.ServeMux). If nil, uses http.DefaultServeMux.
//   - page: A struct instance with route-tagged fields
//   - route: The base route path for this page tree (e.g., "/" or "/admin")
//   - title: The title for the root page
//   - options: Optional configuration (WithErrorHandler, WithMiddlewares, etc.) and dependency injection args
//
// Example with custom mux:
//
//	mux := http.NewServeMux()
//	sp, err := structpages.Mount(mux, index{}, "/", "My App",
//	    structpages.WithErrorHandler(customHandler))
//	sp.URLFor(index.Page)
//	http.ListenAndServe(":8080", mux)
//
// Example with DefaultServeMux:
//
//	sp, err := structpages.Mount(nil, index{}, "/", "My App")
//	http.ListenAndServe(":8080", nil)
func Mount(mux Mux, page any, route, title string, options ...Option) (*StructPages, error) {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	sp := &StructPages{
		onError: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		},
		targetSelector: HTMXRenderTarget,
	}

	for _, opt := range options {
		opt(sp)
	}

	// Parse page tree
	pc, err := parsePageTree(route, page, sp.args...)
	if err != nil {
		return nil, err
	}
	pc.root.Title = title
	sp.pc = pc

	// Register all pages
	middlewares := append([]MiddlewareFunc{withPcCtx(pc), extractURLParams}, sp.middlewares...)
	if err := sp.registerPageItem(mux, pc.root, middlewares); err != nil {
		return nil, err
	}

	return sp, nil
}

// WithArgs adds global dependency injection arguments that will be
// available to all page methods (Props, Middlewares, ServeHTTP etc.).
func WithArgs(args ...any) func(*StructPages) {
	return func(r *StructPages) {
		r.args = append(r.args, args...)
	}
}

// WithTargetSelector sets a custom TargetSelector function that determines
// which component to render based on the request. The default is HTMXRenderTarget,
// which handles HTMX partial requests automatically.
//
// The selector function receives the request and page node, and returns a RenderTarget
// that will be passed to Props. This allows custom logic for component selection.
//
// Example - Custom selector with A/B testing:
//
//	sp := Mount(http.NewServeMux(), index{}, "/", "My App",
//	    WithTargetSelector(func(r *http.Request, pn *PageNode) (RenderTarget, error) {
//	        if getABTestVariant(r) == "B" {
//	            // Use different component for variant B
//	            method := pn.Components["ContentB"]
//	            return newMethodRenderTarget("ContentB", method), nil
//	        }
//	        // Fallback to default HTMX behavior
//	        return HTMXRenderTarget(r, pn)
//	    }))
func WithTargetSelector(selector TargetSelector) func(*StructPages) {
	return func(r *StructPages) {
		r.targetSelector = selector
	}
}

// WithErrorHandler sets a custom error handler function that will be called when
// an error occurs during page rendering or request handling. If not set, a default
// handler returns a generic "Internal Server Error" response.
func WithErrorHandler(onError func(http.ResponseWriter, *http.Request, error)) func(*StructPages) {
	return func(r *StructPages) {
		r.onError = onError
	}
}

// WithMiddlewares adds global middleware functions that will be applied to all routes.
// Middleware is executed in the order provided, with the first middleware being the
// outermost handler. These global middlewares run before any page-specific middlewares.
func WithMiddlewares(middlewares ...MiddlewareFunc) func(*StructPages) {
	return func(r *StructPages) {
		r.middlewares = append(r.middlewares, middlewares...)
	}
}

// WithWarnEmptyRoute sets a custom warning function for pages that have neither
// a handler method nor children. These pages are automatically skipped during
// route registration. If warnFunc is nil, a default warning message is printed
// to stdout. Set warnFunc to a no-op function to suppress warnings entirely.
//
// Example usage:
//
//	// Use default warning (prints to stdout)
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(nil),
//	)
//
//	// Custom warning function
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(func(pn *PageNode) {
//			log.Printf("Skipping empty page: %s", pn.Name)
//		}),
//	)
//
//	// Suppress warnings entirely
//	sp := structpages.Mount(
//		http.NewServeMux(), index{}, "/", "App",
//		structpages.WithWarnEmptyRoute(func(*PageNode) {}),
//	)
func WithWarnEmptyRoute(warnFunc func(*PageNode)) func(*StructPages) {
	if warnFunc == nil {
		warnFunc = func(pn *PageNode) {
			fmt.Printf("⚠️  Warning: page route has no children and no handler, skipping route registration: %s\n", pn.Name)
		}
	}
	return func(r *StructPages) {
		r.warnEmptyRoute = warnFunc
	}
}

func (sp *StructPages) registerPageItem(mux Mux, page *PageNode, mw []MiddlewareFunc) error {
	if page.Route == "" {
		return fmt.Errorf("page item route is empty: %s", page.Name)
	}

	if page.Middlewares != nil {
		res, err := sp.pc.callMethod(page, page.Middlewares)
		if err != nil {
			return fmt.Errorf("error calling Middlewares method on %s: %w", page.Name, err)
		}
		if len(res) != 1 {
			return fmt.Errorf("middlewares method on %s did not return single result", page.Name)
		}
		mws, ok := res[0].Interface().([]MiddlewareFunc)
		if !ok {
			return fmt.Errorf("middlewares method on %s did not return []func(http.Handler, *PageNode) http.Handler", page.Name)
		}
		mw = append(mw, mws...)
	}
	if page.Children != nil {
		// nested pages has to be registered first to avoid conflicts with the parent route
		for _, child := range page.Children {
			if err := sp.registerPageItem(mux, child, mw); err != nil {
				return err
			}
		}
	}
	handler := sp.buildHandler(page)
	if handler == nil && len(page.Children) == 0 {
		if sp.warnEmptyRoute != nil {
			sp.warnEmptyRoute(page)
		}
		return nil
	} else if handler == nil {
		return nil
	}
	for _, middleware := range slices.Backward(mw) {
		handler = middleware(handler, page)
	}
	// If method is "ALL", register without method prefix (matches all methods)
	// Otherwise, register with "METHOD /path" format
	pattern := page.FullRoute()
	if page.Method != methodAll {
		pattern = page.Method + " " + pattern
	}
	mux.Handle(pattern, handler)
	return nil
}

func (sp *StructPages) buildHandler(page *PageNode) http.Handler {
	if h := sp.asHandler(page); h != nil {
		return h
	}
	if len(page.Components) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject current page into context for IDFor to use with standalone functions
		ctx := currentPageCtx.WithValue(r.Context(), page)
		r = r.WithContext(ctx)

		// 1. Select which component to render using TargetSelector
		target, err := sp.targetSelector(r, page)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error selecting target for %s: %w", page.Name, err))
			return
		}

		// 2. Call Props with RenderTarget available for injection
		props, err := sp.execProps(page, r, w, target)
		if err != nil {
			// Check if it's a render component error
			if sp.handleRenderComponentError(w, r, err, page) {
				return
			}

			if errors.Is(err, ErrSkipPageRender) {
				return
			}
			sp.onError(w, r, fmt.Errorf("error running props for %s: %w", page.Name, err))
			return
		}

		// 3. Extract method from target and render with props
		// Type-assert to get the method
		if mrt, ok := target.(*methodRenderTarget); ok {
			// Validate method before calling
			if !mrt.method.Func.IsValid() {
				// Check if Props method exists - if so, this is a Props-only page
				if _, hasProps := page.Props["Props"]; hasProps {
					sp.onError(w, r, fmt.Errorf("page %s: no component found and Props did not use RenderComponent", page.Name))
				} else {
					sp.onError(w, r, fmt.Errorf("page %s does not have a Page component method", page.Name))
				}
				return
			}
			comp, err := sp.pc.callComponentMethod(page, &mrt.method, props...)
			if err != nil {
				sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, mrt.method.Name, err))
				return
			}
			sp.render(w, r, comp)
			return
		}

		// If we get here, target is not a method (could be function or custom)
		// Props should have called RenderComponent

		// Provide better error message for function targets
		if frt, ok := target.(*functionRenderTarget); ok {
			// Convert kebab-case ID to PascalCase function name
			funcName := kebabToPascal(frt.hxTarget)
			sp.onError(w, r, fmt.Errorf(
				"page %s: Component function '%s' is targeted but not handled. "+
					"check target.Is(%s) and call RenderComponent(target, args...). "+
					"Or build the component directly and call RenderComponent(component)",
				page.Name, funcName, funcName))
			return
		}

		// Generic error for other custom target types
		sp.onError(w, r, fmt.Errorf("page %s: target is not a method and Props did not use RenderComponent", page.Name))
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
	handlerType    = reflect.TypeOf((*http.Handler)(nil)).Elem()
	errHandlerType = reflect.TypeOf((*httpErrHandler)(nil)).Elem()
)

func (sp *StructPages) asHandler(pn *PageNode) http.Handler {
	v := pn.Value
	st, pt := v.Type(), v.Type()
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	method, ok := st.MethodByName("ServeHTTP")
	if !ok || isPromotedMethod(&method) {
		method, ok = pt.MethodByName("ServeHTTP")
		if !ok || isPromotedMethod(&method) {
			return nil
		}
	}

	if v.Type().Implements(handlerType) {
		return v.Interface().(http.Handler)
	}
	if v.Type().Implements(errHandlerType) {
		h := v.Interface().(httpErrHandler)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// because we have to handle errors, and error handler could write header
			// potentially we want to clear the buffer writer
			bw := newBuffered(w)
			defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
			if err := h.ServeHTTP(bw, r); err != nil {
				// Clear the buffer since we have an error
				bw.buf.Reset()
				// Check if it's a render component error
				if sp.handleRenderComponentError(bw, r, err, pn) {
					return
				}
				// Write error directly to the buffered writer
				sp.onError(bw, r, err)
			}
		})
	}
	// extended ServeHTTP method with extra arguments
	if method.Type.NumIn() > 3 { // receiver, http.ResponseWriter, *http.Request
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var wv reflect.Value // ResponseWriter, will be buffered if handler returns error
			var bw *buffered
			if method.Type.NumOut() > 0 {
				// If the method returns any values (including just an error), we need to buffer
				bw = newBuffered(w)
				defer func() { _ = bw.close() }() // ignore error, no way to recover from it. maybe log it?
				wv = reflect.ValueOf(bw)
			} else {
				wv = reflect.ValueOf(w)
			}

			// Create RenderTarget for dependency injection using targetSelector
			var renderTarget RenderTarget
			if sp.targetSelector != nil {
				renderTarget, _ = sp.targetSelector(r, pn)
			}

			// Make RenderTarget available for dependency injection
			additionalArgs := []reflect.Value{wv, reflect.ValueOf(r), reflect.ValueOf(renderTarget)}

			results, err := sp.pc.callMethod(pn, &method, additionalArgs...)
			if err != nil {
				if bw != nil {
					bw.buf.Reset()
					sp.onError(bw, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				} else {
					sp.onError(w, r, fmt.Errorf("error calling ServeHTTP method on %s: %w", pn.Name, err))
				}
				return
			}
			_, err = extractError(results)
			if err != nil {
				// bw is guaranteed to be non-nil here because:
				// - bw is nil only when method.Type.NumOut() == 0
				// - when NumOut() == 0, extractError always returns nil error
				// - therefore this branch is only reachable when bw != nil
				bw.buf.Reset()
				// Check if it's a render component error
				if sp.handleRenderComponentError(bw, r, err, pn) {
					return
				}
				sp.onError(bw, r, err)
				return
			}
		})
	}

	// unlikely case: ServeHTTP exists but does not match any known signature
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sp.onError(w, r, fmt.Errorf("page %s has ServeHTTP method with unsupported signature", pn.Name))
	})
}

func (sp *StructPages) execProps(pn *PageNode,
	r *http.Request, w http.ResponseWriter, renderTarget RenderTarget,
) ([]reflect.Value, error) {
	// Look for Props method
	propMethod, ok := pn.Props["Props"]
	if !ok {
		return nil, nil
	}

	if !propMethod.Func.IsValid() {
		return nil, fmt.Errorf("Props method for page %s has invalid Func", pn.Name)
	}

	// Make RenderTarget available for injection along with r and w
	// Note: only pass valid values to avoid zero reflect.Value issues
	args := []reflect.Value{reflect.ValueOf(r), reflect.ValueOf(w)}
	if renderTarget != nil {
		args = append(args, reflect.ValueOf(renderTarget))
	}
	props, err := sp.pc.callMethod(pn, &propMethod, args...)
	if err != nil {
		return nil, fmt.Errorf("error calling Props method %s.Props: %w", pn.Name, err)
	}
	return extractError(props)
}
