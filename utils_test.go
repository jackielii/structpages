package structpages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_newBuffered(t *testing.T) {
	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bw := newBuffered(w)
		bw.Write([]byte("Hello, World!"))
		// write header after writing to the buffer
		w.Header().Add("X-test", "test")
		bw.close()
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "Hello, World!" {
		t.Errorf("expected body %q, got %q", "Hello, World!", rec.Body.String())
	}
	if rec.Header().Get("X-test") != "test" {
		t.Errorf("expected header X-test to be 'test', got %q", rec.Header().Get("X-test"))
	}
}
