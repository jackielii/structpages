//lint:file-ignore U1000 Ignore unused code in test file

package structpages

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackielii/ctxkey"
)

// Test context keys for middleware tests
var (
	testCtxKey1 = ctxkey.New("middleware.test.key1", "")
	testCtxKey2 = ctxkey.New("middleware.test.key2", 0)
)

// Test page types
type orderTestPage struct{}

func (orderTestPage) Page() component {
	return testComponent{content: "order test page,"}
}

func (orderTestPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		makeOrderMiddleware("page1"),
		makeOrderMiddleware("page2"),
	}
}

type contextTestPage struct{}

func (contextTestPage) Page() component {
	return testComponent{content: "context test page"}
}

func (contextTestPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	val1 := testCtxKey1.Value(r.Context())
	val2 := testCtxKey2.Value(r.Context())
	_, _ = fmt.Fprintf(w, "val1=%s,val2=%d", val1, val2)
}

type protectedPage struct{}

func (protectedPage) Page() component {
	return testComponent{content: "protected content"}
}

func (protectedPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{authMiddleware, afterAuthMiddleware}
}

type childPageMw struct{}

func (childPageMw) Page() component {
	return testComponent{content: "child page content"}
}

func (childPageMw) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{childMiddleware}
}

type parentPageMw struct {
	childPageMw `route:"/child Child Page"`
}

func (parentPageMw) Page() component {
	return testComponent{content: "parent page content"}
}

func (parentPageMw) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{parentMiddleware}
}

type complexPage struct{}

func (complexPage) Page() component {
	return testComponent{content: "complex page"}
}

func (complexPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		makeComplexMiddleware("page-mw-1", "X-Flow"),
		makeComplexMiddleware("page-mw-2", "X-Flow"),
		makeComplexMiddleware("page-mw-3", "X-Flow"),
	}
}

type errorTestPage struct{}

func (errorTestPage) Page() component {
	return testComponent{content: "error test page"}
}

func (errorTestPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{panicMiddleware}
}

type methodPage struct{}

func (methodPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("method: " + r.Method))
}

func (methodPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{methodMiddleware}
}

type infoPage struct{}

func (infoPage) Page() component {
	return testComponent{content: "info page"}
}

func (infoPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{nodeInfoMiddleware}
}

type integrationPage struct {
	middlewareChildPage `route:"/child Child"`
}

func (integrationPage) Page() component {
	return testComponent{content: "integration page"}
}

func (integrationPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		func(next http.Handler, node *PageNode) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Integration", "test")
				next.ServeHTTP(w, r)
			})
		},
	}
}

// Helper functions to create middlewares
func makeOrderMiddleware(name string) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Write before
			_, _ = w.Write([]byte(name + " before,"))

			// Call the next handler
			next.ServeHTTP(w, r)

			// Write after
			_, _ = w.Write([]byte(name + " after"))
			if name != "global1" {
				_, _ = w.Write([]byte(","))
			}
		})
	}
}

// Create a middleware that logs and modifies headers
func makeComplexMiddleware(name, headerKey string) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add header before
			existing := r.Header.Get(headerKey)
			if existing != "" {
				existing += ","
			}
			r.Header.Set(headerKey, existing+name+"-before")

			// Add response header
			w.Header().Set("X-"+name, "processed")

			next.ServeHTTP(w, r)

			// Add header after
			existing = r.Header.Get(headerKey)
			r.Header.Set(headerKey, existing+","+name+"-after")
		})
	}
}

// Middleware that checks for authorization header
var authMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware that should not be reached when short-circuited
var afterAuthMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-After-Auth", "reached")
		next.ServeHTTP(w, r)
	})
}

// Parent middleware
var parentMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Parent", "parent-middleware")
		next.ServeHTTP(w, r)
	})
}

// Child middleware
var childMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Child", "child-middleware")
		next.ServeHTTP(w, r)
	})
}

// Middleware that panics
var panicMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("panic") == "true" {
			panic("middleware panic")
		}
		next.ServeHTTP(w, r)
	})
}

// Recovery middleware
var recoveryMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "Recovered from: %v", err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Method-specific middleware
var methodMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Method", r.Method)
		if r.Method == http.MethodPost {
			w.Header().Set("X-Post-Only", "true")
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware that uses PageNode information
var nodeInfoMiddleware = func(next http.Handler, node *PageNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Page-Name", node.Name)
		w.Header().Set("X-Page-Title", node.Title)
		w.Header().Set("X-Page-Route", node.Route)
		w.Header().Set("X-Page-Method", node.Method)
		next.ServeHTTP(w, r)
	})
}

// Test middleware execution order with global and page-specific middlewares
func TestMiddlewareExecutionOrder(t *testing.T) {
	// Create Router with global middlewares
	type topPage struct {
		orderTestPage `route:"/order Order Test"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page",
		WithMiddlewares(
			makeOrderMiddleware("global1"),
			makeOrderMiddleware("global2"),
		),
	)
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	req := httptest.NewRequest(http.MethodGet, "/order", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// The expected order should show the "onion" pattern:
	// global1 -> global2 -> page1 -> page2 -> handler -> page2 -> page1 -> global2 -> global1
	expectedBody := "global1 before,global2 before,page1 before,page2 before," +
		"order test page,page2 after,page1 after,global2 after,global1 after"
	body := rec.Body.String()

	if body != expectedBody {
		t.Errorf("unexpected middleware execution order\nwant: %s\ngot:  %s", expectedBody, body)
	}
}

// Test that middlewares can modify request context
func TestMiddlewareContextModification(t *testing.T) {
	// Middleware that adds values to context
	contextMiddleware1 := func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := testCtxKey1.WithValue(r.Context(), "middleware1-value")
			ctx = testCtxKey2.WithValue(ctx, 42)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Middleware that modifies existing context values
	contextMiddleware2 := func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and modify existing values
			val1 := testCtxKey1.Value(r.Context())
			val2 := testCtxKey2.Value(r.Context())

			ctx := testCtxKey1.WithValue(r.Context(), val1+"-modified")
			ctx = testCtxKey2.WithValue(ctx, val2+10)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	type topPage struct {
		contextTestPage `route:"/context Context Test"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page",
		WithMiddlewares(contextMiddleware1, contextMiddleware2))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	req := httptest.NewRequest(http.MethodGet, "/context", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expectedBody := "val1=middleware1-value-modified,val2=52"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

// Test that middlewares can short-circuit the request
func TestMiddlewareShortCircuit(t *testing.T) {
	type topPage struct {
		protectedPage `route:"/protected Protected Page"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	// Test without authorization header
	{
		req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
		if rec.Body.String() != "Unauthorized" {
			t.Errorf("expected body %q, got %q", "Unauthorized", rec.Body.String())
		}
		if rec.Header().Get("X-After-Auth") != "" {
			t.Error("afterAuthMiddleware should not have been reached")
		}
	}

	// Test with authorization header
	{
		req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "protected content" {
			t.Errorf("expected body %q, got %q", "protected content", rec.Body.String())
		}
		if rec.Header().Get("X-After-Auth") != "reached" {
			t.Error("afterAuthMiddleware should have been reached")
		}
	}
}

// Test nested pages with middlewares
func TestNestedPagesMiddleware(t *testing.T) {
	type topPage struct {
		parentPageMw `route:"/parent Parent Page"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	// Test parent page - should only have parent middleware
	{
		req := httptest.NewRequest(http.MethodGet, "/parent", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Parent") != "parent-middleware" {
			t.Error("parent middleware should have been applied")
		}
		if rec.Header().Get("X-Child") != "" {
			t.Error("child middleware should not have been applied to parent")
		}
		if rec.Body.String() != "parent page content" {
			t.Errorf("expected body %q, got %q", "parent page content", rec.Body.String())
		}
	}

	// Test child page - should have both parent and child middlewares
	{
		req := httptest.NewRequest(http.MethodGet, "/parent/child", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Parent") != "parent-middleware" {
			t.Error("parent middleware should have been applied to child")
		}
		if rec.Header().Get("X-Child") != "child-middleware" {
			t.Error("child middleware should have been applied")
		}
		if rec.Body.String() != "child page content" {
			t.Errorf("expected body %q, got %q", "child page content", rec.Body.String())
		}
	}
}

// Test multiple global middlewares with multiple page middlewares
func TestComplexMiddlewareChain(t *testing.T) {
	type topPage struct {
		complexPage `route:"/complex Complex Page"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page",
		WithMiddlewares(
			makeComplexMiddleware("global-mw-1", "X-Flow"),
			makeComplexMiddleware("global-mw-2", "X-Flow"),
		),
	)
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	req := httptest.NewRequest(http.MethodGet, "/complex", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Check that all middlewares set their headers
	expectedHeaders := map[string]string{
		"X-global-mw-1": "processed",
		"X-global-mw-2": "processed",
		"X-page-mw-1":   "processed",
		"X-page-mw-2":   "processed",
		"X-page-mw-3":   "processed",
	}

	for key, value := range expectedHeaders {
		if rec.Header().Get(key) != value {
			t.Errorf("expected header %s to be %q, got %q", key, value, rec.Header().Get(key))
		}
	}
}

// Test error handling in middlewares
func TestMiddlewareErrorHandling(t *testing.T) {
	type topPage struct {
		errorTestPage `route:"/error Error Test"`
	}
	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page", WithMiddlewares(recoveryMiddleware))
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
		_ = sp
	}

	// Test normal request
	{
		req := httptest.NewRequest(http.MethodGet, "/error", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "error test page" {
			t.Errorf("expected body %q, got %q", "error test page", rec.Body.String())
		}
	}

	// Test panic request
	{
		req := httptest.NewRequest(http.MethodGet, "/error?panic=true", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
		expectedBody := "Recovered from: middleware panic"
		if rec.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
		}
	}
}

// Test middleware with different HTTP methods
func TestMiddlewareWithHTTPMethods(t *testing.T) {
	type topPage struct {
		getPage  methodPage `route:"GET /method Method GET"`
		postPage methodPage `route:"POST /method Method POST"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Test GET request
	{
		req := httptest.NewRequest(http.MethodGet, "/method", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Method") != "GET" {
			t.Errorf("expected X-Method header to be GET, got %s", rec.Header().Get("X-Method"))
		}
		if rec.Header().Get("X-Post-Only") != "" {
			t.Error("X-Post-Only header should not be set for GET request")
		}
		if rec.Body.String() != "method: GET" {
			t.Errorf("expected body %q, got %q", "method: GET", rec.Body.String())
		}
	}

	// Test POST request
	{
		req := httptest.NewRequest(http.MethodPost, "/method", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Method") != "POST" {
			t.Errorf("expected X-Method header to be POST, got %s", rec.Header().Get("X-Method"))
		}
		if rec.Header().Get("X-Post-Only") != "true" {
			t.Error("X-Post-Only header should be set for POST request")
		}
		if rec.Body.String() != "method: POST" {
			t.Errorf("expected body %q, got %q", "method: POST", rec.Body.String())
		}
	}
}

// Test middleware access to PageNode information
func TestMiddlewarePageNodeAccess(t *testing.T) {
	type topPage struct {
		infoPage `route:"POST /info Info Page Title"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	req := httptest.NewRequest(http.MethodPost, "/info", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expectedHeaders := map[string]string{
		"X-Page-Name":   "infoPage",
		"X-Page-Title":  "Info Page Title",
		"X-Page-Route":  "/info",
		"X-Page-Method": "POST",
	}

	for key, expected := range expectedHeaders {
		if got := rec.Header().Get(key); got != expected {
			t.Errorf("expected header %s to be %q, got %q", key, expected, got)
		}
	}
}

// Test middleware combination with existing middleware tests from struct_pages_test.go
func TestMiddlewareIntegration(t *testing.T) {
	// This tests the integration with the existing middleware pattern
	// to ensure our new tests don't break existing functionality

	type topPage struct {
		integrationPage `route:"/integration Integration Test"`
	}

	mux := http.NewServeMux()
	sp, err := Mount(mux, &topPage{}, "/", "top page")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}
	_ = sp

	// Test parent page
	{
		req := httptest.NewRequest(http.MethodGet, "/integration", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Integration") != "test" {
			t.Error("integration middleware should have been applied")
		}
		if rec.Body.String() != "integration page" {
			t.Errorf("expected body %q, got %q", "integration page", rec.Body.String())
		}
	}

	// Test child page inherits parent middleware
	{
		req := httptest.NewRequest(http.MethodGet, "/integration/child", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Integration") != "test" {
			t.Error("parent middleware should have been applied to child")
		}
		if rec.Body.String() != "Test middleware child page" {
			t.Errorf("expected body %q, got %q", "Test middleware child page", rec.Body.String())
		}
	}
}
