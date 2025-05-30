package chirouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackielii/srx"
)

func TestChiRouter(t *testing.T) {
	r := NewChiRouter(chi.NewRouter())
	{
		// Test HandleMethod
		r.HandleMethod(http.MethodGet, "/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ChiRouter HandleMethod"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/handle", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "ChiRouter HandleMethod" {
			t.Errorf("expected body %q, got %q", "ChiRouter HandleMethod", rec.Body.String())
		}
	}
	{
		// Test Route method
		r.Route("/test", func(r srx.Router) {
			r.HandleMethod(http.MethodGet, "/sub", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ChiRouter Subroute"))
			}))
			r.HandleMethod(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ChiRouter root route"))
			}))
		})
		req := httptest.NewRequest(http.MethodGet, "/test/sub?a&b&c", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "ChiRouter Subroute" {
			t.Errorf("expected body %q, got %q", "ChiRouter Subroute", rec.Body.String())
		}
	}
	{
		// test route with path value
		r.Route("/withid/{id}", func(r srx.Router) {
			r.HandleMethod(http.MethodGet, "/end", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ChiRouter with ID: " + id))
			}))
		})
		req := httptest.NewRequest(http.MethodGet, "/withid/123/end", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "ChiRouter with ID: 123" {
			t.Errorf("expected body %q, got %q", "ChiRouter with ID: 123", rec.Body.String())
		}
	}
}
