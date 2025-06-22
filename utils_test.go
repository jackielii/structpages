package structpages

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_newBuffered(t *testing.T) {
	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		bw := newBuffered(w)
		_, _ = bw.Write([]byte("Hello, World!"))
		// write header after writing to the buffer
		w.Header().Add("X-test", "test")
		_ = bw.close()
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
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

func TestBufferedWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	bw := newBuffered(rec)

	// Test WriteHeader
	bw.WriteHeader(http.StatusCreated)

	// The status should be buffered, not written immediately
	if rec.Code != http.StatusOK {
		t.Errorf("expected recorder to still have default status %d, got %d", http.StatusOK, rec.Code)
	}

	// Write some data
	_, _ = bw.Write([]byte("test"))

	// Close should write the buffered status
	_ = bw.close()

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d after close, got %d", http.StatusCreated, rec.Code)
	}
}

func TestBufferedUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	bw := newBuffered(rec)

	// Test Unwrap
	unwrapped := bw.Unwrap()

	if unwrapped != rec {
		t.Errorf("expected Unwrap to return original ResponseWriter, got %v", unwrapped)
	}

	// Test that http.ResponseController can work with our buffered writer
	rc := http.NewResponseController(bw)

	// Test Flush - httptest.ResponseRecorder implements Flush()
	err := rc.Flush()
	if err != nil {
		t.Errorf("expected Flush to work through Unwrap, got error: %v", err)
	}

	// Test SetWriteDeadline - should fail since httptest.ResponseRecorder doesn't support it
	err = rc.SetWriteDeadline(time.Now().Add(time.Second))
	if !errors.Is(err, http.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported for SetWriteDeadline, got %v", err)
	}
}
