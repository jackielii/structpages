package structpages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
	sp := NewStructPages()
	sp.MountPages(r, "/", &topPage{})

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

type middlewarePages struct{}

func (middlewarePages) Middlewares() []middlewareFunc {
	return []middlewareFunc{
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
	println(PrintRoutes(&topPage{}))
	r := NewRouter(http.NewServeMux())
	sp := NewStructPages()
	sp.MountPages(r, "/", &topPage{})
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
