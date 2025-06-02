package structpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testComponent struct {
	content string
}

func (t testComponent) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte(t.content))
	return err
}

type TestHandlerPage struct{}

func (TestHandlerPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("TestHttpHandler"))
}

func TestHttpHandler(t *testing.T) {
	type topPage struct {
		s TestHandlerPage  `route:"/struct Test struct handler"`
		p *TestHandlerPage `route:"POST /pointer Test pointer handler"`
	}

	// println(PrintRoutes(&topPage{}))
	mux := http.NewServeMux()
	r := NewRouter(mux)
	sp := New()
	sp.MountPages(r, &topPage{}, "/", "")

	{
		req := httptest.NewRequest(http.MethodGet, "/struct", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "TestHttpHandler" {
			t.Errorf("expected body %q, got %q", "TestHttpHandler", rec.Body.String())
		}
	}

	{
		req := httptest.NewRequest(http.MethodPost, "/pointer", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "TestHttpHandler" {
			t.Errorf("expected body %q, got %q", "TestHttpHandler", rec.Body.String())
		}
	}
}

type middlewarePages struct {
	middlewareChildPage `route:"/child Child"`
}

type middlewareChildPage struct{}

func (middlewareChildPage) Page() component {
	return testComponent{content: "Test middleware child page"}
}

func (middlewarePages) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		func(next http.Handler, node *PageNode) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Middleware", "foobar")
				next.ServeHTTP(w, r)
			})
		},
	}
}

func (middlewarePages) Page() component {
	return testComponent{content: "Test middleware handler"}
}

func TestMiddlewares(t *testing.T) {
	type topPage struct {
		middlewarePages `route:"/middleware Test middleware handler"`
	}
	println(PrintRoutes("/", &topPage{}))
	r := NewRouter(http.NewServeMux())
	sp := New()
	sp.MountPages(r, &topPage{}, "/", "top page")
	{
		req := httptest.NewRequest(http.MethodGet, "/middleware", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Test-Middleware") != "foobar" {
			t.Errorf("expected header X-Test-Middleware to be 'foobar', got %s", rec.Header().Get("X-Test-Middleware"))
		}
		if rec.Body.String() != "Test middleware handler" {
			t.Errorf("expected body %q, got %q", "Test middleware handler", rec.Body.String())
		}
	}
	{
		// test child page also has the middleware applied
		req := httptest.NewRequest(http.MethodGet, "/middleware/child", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Header().Get("X-Test-Middleware") != "foobar" {
			t.Errorf("expected header X-Test-Middleware to be 'foobar', got %s", rec.Header().Get("X-Test-Middleware"))
		}
		if rec.Body.String() != "Test middleware child page" {
			t.Errorf("expected body %q, got %q", "Test middleware child page", rec.Body.String())
		}
	}
}

type DefaultConfigPage struct{}

func (DefaultConfigPage) Page() component {
	return testComponent{content: "Default config page"}
}

func (DefaultConfigPage) HxTarget() component {
	return testComponent{content: "hx target defaultConfigPage"}
}

// type ConfiTestPage struct{}
//
// func (ConfiTestPage) PageConfig(r *http.Request) (string, error) {
// 	return "DefaultConfigPage", nil
// }

func TestPageConfig(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		DefaultConfigPage `route:"/default Default config page"`
	}
	sp.MountPages(r, &topPage{}, "/", "top page")
	{
		req := httptest.NewRequest(http.MethodGet, "/default", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "Default config page" {
			t.Errorf("expected body %q, got %q", "Default config page", rec.Body.String())
		}
	}
}

func TestHTMXPageConfig(t *testing.T) {
	sp := New(WithDefaultPageConfig(HTMXPageConfig))
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		DefaultConfigPage `route:"/default Default config page"`
	}
	sp.MountPages(r, &topPage{}, "/", "top page")

	req := httptest.NewRequest(http.MethodGet, "/default", nil)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-Target", "hx-target")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expectedBody := "hx target defaultConfigPage"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

type CustomConfigPage struct{}

func (CustomConfigPage) Custom() component {
	return testComponent{content: "Custom config page"}
}

func (CustomConfigPage) PageConfig(r *http.Request) (string, error) {
	return "Custom", nil
}

func TestCustomPageConfig(t *testing.T) {
	sp := New()
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		CustomConfigPage `route:"/custom Custom config page"`
	}
	sp.MountPages(r, &topPage{}, "/", "top page")

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expectedBody := "Custom config page"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

type middlewareOrderPage struct{}

func (middlewareOrderPage) Page() component {
	return testComponent{content: "Middleware Order Page\n"}
}

func (middlewareOrderPage) Middlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		makeMiddleware("page mw 1"),
		makeMiddleware("page mw 2"),
		makeMiddleware("page mw 3"),
	}
}

func makeMiddleware(name string) MiddlewareFunc {
	return func(next http.Handler, node *PageNode) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Middleware before: " + name + "\n"))
			next.ServeHTTP(w, r)
			w.Write([]byte("Middleware after: " + name + "\n"))
		})
	}
}

func TestMiddlewareOrder(t *testing.T) {
	sp := New(
		WithMiddlewares(
			makeMiddleware("global mw 1"),
			makeMiddleware("global mw 2"),
			makeMiddleware("global mw 3"),
		),
	)
	r := NewRouter(http.NewServeMux())
	type topPage struct {
		middlewareOrderPage `route:"/"`
	}
	sp.MountPages(r, &topPage{}, "/", "top page")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expectedBody := `Middleware before: global mw 1
Middleware before: global mw 2
Middleware before: global mw 3
Middleware before: page mw 1
Middleware before: page mw 2
Middleware before: page mw 3
Middleware Order Page
Middleware after: page mw 3
Middleware after: page mw 2
Middleware after: page mw 1
Middleware after: global mw 3
Middleware after: global mw 2
Middleware after: global mw 1
`
	if diff := cmp.Diff(expectedBody, rec.Body.String()); diff != "" {
		t.Errorf("unexpected body (-want +got):\n%s", diff)
	}
}
