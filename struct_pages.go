package structpages

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
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
	requestArgs    []any
	urlPrefix      string
	maxIDLen       int
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
	return idFor(sp.pc, nil, v, true)
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
	return idFor(sp.pc, nil, v, false)
}

// Key generates the raw HTML id of one keyed instance of a component (see
// the package-level [Key]). It works without context by using the
// structpages's parseContext directly.
//
// Example:
//
//	sp.Key(p.Row, "s3")
//	// → "team-management-view-row--s3"
func (sp *StructPages) Key(v any, key string) (string, error) {
	if err := checkKeyArgs(v, key); err != nil {
		return "", err
	}
	id, err := sp.ID(v)
	if err != nil {
		return "", err
	}
	return id + keySep + escapeKey(key), nil
}

// KeyTarget is [StructPages.Key] as a CSS selector (with "#" prefix), for
// hx-target.
func (sp *StructPages) KeyTarget(v any, key string) (string, error) {
	id, err := sp.Key(v, key)
	if err != nil {
		return "", err
	}
	return "#" + id, nil
}

// URLFor returns the URL for a given page type. If args is provided, it'll replace
// the path segments. Supported format is similar to http.ServeMux.
//
// Unlike the context-based URLFor function, this method doesn't have access to
// pre-extracted URL parameters from the current request, so all required parameters
// must be provided as args.
//
// Type-based lookup is strict: if the same page type is mounted under
// multiple parents, URLFor returns an error listing every match
// instead of silently choosing one. Disambiguate with the []any chain
// form (recommended; type-safe), Ref("Parent.Field") for cross-package
// callers, or a func(*PageNode) bool predicate.
//
// Path and query parameters are passed via a map[string]any (preferred
// — explicit and resilient to route changes):
//
//	sp.URLFor(Page{}, map[string]any{"id": 42})
//	sp.URLFor([]any{Page{}, "?foo={bar}"}, map[string]any{"bar": "baz"})
//
// Positional and key/value-pairs forms also work (see formatPathSegments
// in url_for.go for the full detection order) but require the call site
// to track parameter position or name conventions.
//
// You can pass []any as the page to join multiple path segments
// together — strings are concatenated as-is. You can also pass a
// func(*PageNode) bool predicate to match a specific page when
// type-based lookup isn't enough.
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
		targetSelector: pageTarget,
	}

	for _, opt := range options {
		opt(sp)
	}

	if err := sp.parse(page, route, title); err != nil {
		return nil, err
	}
	if err := sp.resolveRequestArgs(); err != nil {
		return nil, err
	}
	if err := sp.checkInjection(); err != nil {
		return nil, err
	}

	// Register all pages
	middlewares := append([]MiddlewareFunc{withPcCtx(sp.pc), extractURLParams}, sp.middlewares...)
	if err := sp.registerPageItem(mux, sp.pc.root, middlewares); err != nil {
		return nil, err
	}

	return sp, nil
}

// parse builds sp's page tree from page with the options already applied
// to sp. Mount and Parse share it, so the tree and its ids are the same
// whether or not routes are registered.
func (sp *StructPages) parse(page any, route, title string) error {
	pc, err := parsePageTree(route, page, sp.args...)
	if err != nil {
		return err
	}
	pc.root.Title = title
	pc.urlPrefix = sp.urlPrefix
	if sp.maxIDLen > 0 && sp.maxIDLen != pc.maxIDLen {
		// Re-resolve ids against the configured budget. idPath/suffix are
		// length-independent, so only the uniqueness check must re-run.
		pc.maxIDLen = sp.maxIDLen
		if err := pc.checkIDUniqueness(); err != nil {
			return err
		}
	}
	sp.pc = pc
	return nil
}

// WithArgs adds global dependency injection arguments that will be
// available to all page methods (Props, Middlewares, ServeHTTP etc.).
func WithArgs(args ...any) func(*StructPages) {
	return func(r *StructPages) {
		r.args = append(r.args, args...)
	}
}

// WithRequestArgs registers per-request values for the whole mount. Each
// provider is a func(*http.Request) (T, error) or
// func(*http.Request, *PageNode) (T, error); its result fills parameters of
// type T in Props, extended ServeHTTP and component methods, matched by the
// rules WithArgs uses. A provider runs at most once per request, the first
// time a parameter needs it, and its error stops the request like an error
// from Props. Mount fails if a provider has another shape, or if two
// providers return the same type.
//
// A page's RequestArgs method registers providers for that page and the
// pages below it; they shadow mount-wide providers of the same type.
//
//	sp, err := structpages.Mount(mux, pages{}, "/", "App",
//	    structpages.WithRequestArgs(currentUser), // func(*http.Request) (*User, error)
//	)
//
//	func (p accountPage) Props(u *User) (accountProps, error) { … }
func WithRequestArgs(providers ...any) func(*StructPages) {
	return func(r *StructPages) {
		r.requestArgs = append(r.requestArgs, providers...)
	}
}

// WithURLPrefix tells structpages that it is being served behind a path
// prefix that has been stripped before requests reach the registered routes
// (for example by http.StripPrefix or an upstream reverse proxy). The prefix
// is prepended to every URL returned by URLFor — both the *StructPages.URLFor
// method and the request-context URLFor function — so generated URLs point
// at the externally visible path.
//
// This option only affects URL generation. Route registration is still
// controlled by the route argument to Mount.
//
// Example: app registered at "/" internally, served at "/admin" externally:
//
//	inner := http.NewServeMux()
//	sp, _ := structpages.Mount(inner, pages{}, "/", "App",
//	    structpages.WithURLPrefix("/admin"))
//	outer := http.NewServeMux()
//	outer.Handle("/admin/", http.StripPrefix("/admin", inner))
//	// sp.URLFor(home{}) now returns "/admin"
func WithURLPrefix(prefix string) func(*StructPages) {
	return func(r *StructPages) {
		r.urlPrefix = prefix
	}
}

// WithMaxIDLength sets the character budget for generated element ids
// (ID/IDTarget) before they degrade from the readable full-path form
// (every ancestor field name joined, e.g. "admin-users-user-list") to
// the compact leaf-only form (e.g. "user-list", plus a short stable hash
// suffix when the leaf name is not unique in the tree). The default is 40.
//
// A larger value keeps deeper ids fully readable at the cost of length;
// a smaller value favors shorter ids. The value only affects id
// generation, never routing.
func WithMaxIDLength(n int) func(*StructPages) {
	return func(r *StructPages) {
		r.maxIDLen = n
	}
}

// WithTargetSelector sets the TargetSelector that reads what each request
// wants. Its RenderTarget is passed to Props, and when Props returns no
// error the page's component the target Is renders, or Page if none.
//
// Without this option every request wants Page. For htmx, choose the
// selector for your htmx version:
//
//	structpages.WithTargetSelector(structpages.HTMXRenderTarget)   // htmx 1.x and 2.x
//	structpages.WithTargetSelector(structpages.HTMXv4RenderTarget) // htmx 4
//
// A custom selector returns any RenderTarget; the framework only calls its
// Is method. For example, a selector reading a query parameter:
//
//	structpages.WithTargetSelector(func(r *http.Request, pn *structpages.PageNode) (structpages.RenderTarget, error) {
//	    return partialParam(r.URL.Query().Get("partial")), nil
//	})
func WithTargetSelector(selector TargetSelector) func(*StructPages) {
	return func(r *StructPages) {
		if selector != nil {
			r.targetSelector = selector
		}
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
	// Pre-parse route segments for performance (done once at Mount time)
	fullRoute := page.FullRoute()
	if page.routeSegments == nil {
		if segments, err := sp.pc.cacheRouteSegments(fullRoute); err == nil {
			// Shared with the cache: both only read it; segmentsFor and
			// getRouteSegments hand out copies
			page.routeSegments = segments
		}
	}
	// If method is "ALL", register without method prefix (matches all methods)
	// Otherwise, register with "METHOD /path" format
	pattern := fullRoute
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
	if len(page.Components) == 0 && len(page.Props) == 0 {
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Inject current page into context for IDFor to use with standalone functions
		ctx := currentPageCtx.WithValue(r.Context(), page)
		r = r.WithContext(ctx)
		rs := newRequestScope(r, page)

		// 1. What the request wants.
		target, err := sp.selectTarget(r, page)
		if err != nil {
			sp.onError(w, r, fmt.Errorf("error selecting target for %s: %w", page.Name, err))
			return
		}

		// 2. Props is the normal flow; an error breaks it.
		props, err := sp.execProps(rs, page, r, w, target)
		if err != nil {
			if sp.handleRenderComponentError(w, r, err) {
				return
			}
			if errors.Is(err, ErrSkipPageRender) {
				return
			}
			sp.onError(w, r, fmt.Errorf("error running props for %s: %w", page.Name, err))
			return
		}

		// 3. Render what the request wants: the component the target Is,
		// else Page.
		method, err := selectComponent(page, target)
		if err != nil {
			sp.onError(w, r, err)
			return
		}
		comp, err := sp.pc.callComponentMethod(rs, page, method, props, declaredRequestValues(w, r, target))
		if err != nil {
			// A request arg the component needed can break the flow
			// like Props.
			if sp.handleRenderComponentError(w, r, err) || errors.Is(err, ErrSkipPageRender) {
				return
			}
			sp.onError(w, r, fmt.Errorf("error calling component %s.%s: %w", page.Name, method.Name, err))
			return
		}
		sp.render(w, r, comp)
	})
}

// selectComponent returns the component of page that target Is, else
// Page. The framework asks the target itself, so the choice never depends
// on which Is calls Props made. A target that Is more than one component
// is an error rather than a guess.
func selectComponent(page *PageNode, target RenderTarget) (*reflect.Method, error) {
	var selected []string
	for _, name := range page.componentNames {
		if target.Is(page.componentExprs[name]) {
			selected = append(selected, name)
		}
	}
	switch len(selected) {
	case 1:
		m := page.Components[selected[0]]
		return &m, nil
	case 0:
		if m, ok := page.Components["Page"]; ok {
			return &m, nil
		}
		if _, hasProps := page.Props["Props"]; hasProps {
			return nil, fmt.Errorf("page %s has no Page component, and Props returned no error "+
				"while the request wants none of its components; return RenderComponent from Props", page.Name)
		}
		return nil, fmt.Errorf("page %s has no Page component", page.Name)
	}
	return nil, fmt.Errorf("page %s: the render target Is more than one component (%s)",
		page.Name, strings.Join(selected, ", "))
}

// selectTarget runs the configured TargetSelector, or pageTarget when
// none is set.
func (sp *StructPages) selectTarget(r *http.Request, pn *PageNode) (RenderTarget, error) {
	if sp.targetSelector == nil {
		return pageTarget(r, pn)
	}
	return sp.targetSelector(r, pn)
}

// declaredRequestValues are the per-request values a component can take,
// typed as requestArgTypes declares. Components get them only for a
// parameter of exactly that type, so an io.Writer or other interface
// parameter still comes from Props' results or the registry.
func declaredRequestValues(w http.ResponseWriter, r *http.Request, target RenderTarget) []reflect.Value {
	vs := []reflect.Value{reflect.ValueOf(&w).Elem(), reflect.ValueOf(r)}
	if target != nil {
		vs = append(vs, reflect.ValueOf(&target).Elem())
	}
	return vs
}

// pageTarget is the TargetSelector used without WithTargetSelector: every
// request wants Page.
func pageTarget(_ *http.Request, pn *PageNode) (RenderTarget, error) {
	m := pn.Components["Page"]
	return newMethodRenderTarget("Page", &m), nil
}

// requestValues are the per-request values Props and extended ServeHTTP
// take, besides the *PageNode every injected method gets. They are passed
// as their dynamic types, so a parameter of an interface the writer
// implements (io.Writer) gets it.
func requestValues(w http.ResponseWriter, r *http.Request, target RenderTarget) []reflect.Value {
	vs := []reflect.Value{reflect.ValueOf(w), reflect.ValueOf(r)}
	if target != nil {
		vs = append(vs, reflect.ValueOf(target))
	}
	return vs
}

// render renders comps in order into one buffered response, so a failure
// in any of them leaves the response to the error handler.
func (sp *StructPages) render(w http.ResponseWriter, r *http.Request, comps ...component) {
	buf := getBuffer()
	defer releaseBuffer(buf)
	for i, comp := range comps {
		if comp == nil {
			sp.onError(w, r, fmt.Errorf("render: component %d is nil", i+1))
			return
		}
		if err := comp.Render(r.Context(), buf); err != nil {
			sp.onError(w, r, err)
			return
		}
	}
	// Props may have set its own Content-Type (say, for JSON); keep it.
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(buf.Bytes())
}

type httpErrHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

var (
	handlerType    = reflect.TypeOf((*http.Handler)(nil)).Elem()
	errHandlerType = reflect.TypeOf((*httpErrHandler)(nil)).Elem()
)

// requestArgTypes are the declared types of the values every Props and
// extended ServeHTTP call receives per request, besides the *PageNode every
// injected method gets. A parameter one of them can fill never reaches the
// registry.
var requestArgTypes = []reflect.Type{
	reflect.TypeFor[http.ResponseWriter](),
	reflect.TypeFor[*http.Request](),
	reflect.TypeFor[RenderTarget](),
}

// checkInjection reports every Props, extended ServeHTTP and component
// parameter in the tree that more than one request arg or registration
// could fill, so the ambiguity fails Mount instead of the first request to
// that page. A component's parameters are filled from Props' results first,
// so those count as provided. A parameter nothing fills is left to request
// time: a page may be mounted but unused in a configuration that doesn't
// register its dependencies.
func (sp *StructPages) checkInjection() error {
	var errs []error
	for pn := range sp.pc.root.All() {
		var componentArgs []reflect.Type
		if m, ok := pn.Props["Props"]; ok && m.Func.IsValid() {
			errs = append(errs, sp.pc.checkMethodArgs(pn, &m, requestArgTypes, nil))
			componentArgs = propsResultTypes(&m)
		}
		for _, name := range slices.Sorted(maps.Keys(pn.Components)) {
			m := pn.Components[name]
			errs = append(errs, sp.pc.checkMethodArgs(pn, &m, componentArgs, requestArgTypes))
		}
		if m, ok := serveHTTPMethod(pn); ok && isExtendedServeHTTP(&m) {
			errs = append(errs, sp.pc.checkMethodArgs(pn, &m, requestArgTypes, nil))
		}
	}
	return errors.Join(errs...)
}

// propsResultTypes are the declared types of the values Props returns,
// without a trailing error: the values a component's parameters are filled
// from first.
func propsResultTypes(m *reflect.Method) []reflect.Type {
	var ts []reflect.Type
	for i := range m.Type.NumOut() {
		ts = append(ts, m.Type.Out(i))
	}
	if n := len(ts); n > 0 && ts[n-1].AssignableTo(errorType) {
		ts = ts[:n-1]
	}
	return ts
}

// serveHTTPMethod returns pn's own (not promoted) ServeHTTP method, on
// the value or the pointer receiver.
func serveHTTPMethod(pn *PageNode) (reflect.Method, bool) {
	st, pt := pn.Value.Type(), pn.Value.Type()
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	} else {
		pt = reflect.PointerTo(st)
	}
	method, ok := st.MethodByName("ServeHTTP")
	if !ok || isPromotedMethod(&method) {
		method, ok = pt.MethodByName("ServeHTTP")
		if !ok || isPromotedMethod(&method) {
			return reflect.Method{}, false
		}
	}
	return method, true
}

// isExtendedServeHTTP reports whether method is the ServeHTTP form that
// takes parameters beyond the writer and request, filled by injection.
func isExtendedServeHTTP(method *reflect.Method) bool {
	return method.Type.NumIn() > 3 // receiver, http.ResponseWriter, *http.Request
}

func (sp *StructPages) asHandler(pn *PageNode) http.Handler {
	v := pn.Value
	method, ok := serveHTTPMethod(pn)
	if !ok {
		return nil
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
				if sp.handleRenderComponentError(bw, r, err) {
					return
				}
				// Write error directly to the buffered writer
				sp.onError(bw, r, err)
			}
		})
	}
	// extended ServeHTTP method with extra arguments
	if isExtendedServeHTTP(&method) {
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

			// The RenderTarget is injectable here too.
			renderTarget, err := sp.selectTarget(r, pn)
			if err != nil {
				sp.onError(w, r, fmt.Errorf("error selecting target for %s: %w", pn.Name, err))
				return
			}
			additionalArgs := []reflect.Value{wv, reflect.ValueOf(r)}
			if renderTarget != nil {
				additionalArgs = append(additionalArgs, reflect.ValueOf(renderTarget))
			}

			results, err := sp.pc.callMethodIn(newRequestScope(r, pn), pn, &method, additionalArgs...)
			if err != nil {
				// The call failed before the handler ran, so nothing has
				// been written: a request arg's RenderComponent or
				// ErrSkipPageRender breaks the flow as from Props.
				out := w
				if bw != nil {
					bw.buf.Reset()
					out = bw
				}
				if sp.handleRenderComponentError(out, r, err) || errors.Is(err, ErrSkipPageRender) {
					return
				}
				if bw != nil {
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
				if sp.handleRenderComponentError(bw, r, err) {
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

func (sp *StructPages) execProps(rs *requestScope, pn *PageNode,
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

	props, err := sp.pc.callMethodIn(rs, pn, &propMethod, requestValues(w, r, renderTarget)...)
	if err != nil {
		return nil, fmt.Errorf("error calling Props method %s.Props: %w", pn.Name, err)
	}
	return extractError(props)
}
