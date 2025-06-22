package structpages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterWithNil(t *testing.T) {
	router := NewRouter(nil)
	if router == nil {
		t.Fatal("expected router to be non-nil")
	}
	if router.router != http.DefaultServeMux {
		t.Error("expected router to use DefaultServeMux when nil is passed")
	}
}

func TestStdRouter(t *testing.T) {
	router := NewRouter(http.NewServeMux())

	{
		// Test HandleMethod
		router.HandleMethod(http.MethodGet, "/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("StdRouter HandleMethod"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/handle", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "StdRouter HandleMethod" {
			t.Errorf("expected body %q, got %q", "StdRouter HandleMethod", rec.Body.String())
		}
	}

	{
		// Test Route method
		router.Route("/test", func(r Router) {
			r.HandleMethod(http.MethodGet, "/sub", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("StdRouter Subroute"))
			}))
			r.HandleMethod(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("StdRouter root route"))
			}))
		})

		req := httptest.NewRequest(http.MethodGet, "/test/sub?a&b&c", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "StdRouter Subroute" {
			t.Errorf("expected body %q, got %q", "StdRouter Subroute", rec.Body.String())
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "StdRouter root route" {
			t.Errorf("expected body %q, got %q", "Subrouter root route", rec.Body.String())
		}
	}

	{
		// test route with path value
		router.Route("/withid/{id}", func(r Router) {
			r.HandleMethod(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := r.PathValue("id")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("StdRouter with ID: " + id))
			}))
			r.HandleMethod(http.MethodGet, "/end", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := r.PathValue("id")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("StdRouter with ID: " + id))
			}))
		})
		req := httptest.NewRequest(http.MethodGet, "/withid/123/end", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "StdRouter with ID: 123" {
			t.Errorf("expected body %q, got %q", "StdRouter with ID: 123", rec.Body.String())
		}
	}
}
