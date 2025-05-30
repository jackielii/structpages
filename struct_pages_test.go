package srx

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
