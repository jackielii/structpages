package srx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStdRouter(t *testing.T) {
	mux := http.NewServeMux()
	stdRouter := NewStdRouter(mux)

	{
		// Test HandleMethod
		stdRouter.HandleMethod(http.MethodGet, "/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("StdRouter HandleMethod"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/handle", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "StdRouter HandleMethod" {
			t.Errorf("expected body %q, got %q", "StdRouter HandleMethod", rec.Body.String())
		}
	}
	{
		// Test Route method
		stdRouter.Route("/test", func(r Router) {
			r.HandleMethod(http.MethodGet, "/sub", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("StdRouter Subroute"))
			}))
		})

		req := httptest.NewRequest(http.MethodGet, "/test/sub?a&b&c", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "StdRouter Subroute" {
			t.Errorf("expected body %q, got %q", "StdRouter Subroute", rec.Body.String())
		}
	}
}
