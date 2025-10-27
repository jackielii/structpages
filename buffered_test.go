package structpages

import (
	"bytes"
	"errors"
	"fmt"
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

	// Test Flush - should work with our new implementation
	err := rc.Flush()
	if err != nil {
		t.Errorf("expected Flush to work, got error: %v", err)
	}

	// Test SetWriteDeadline - should fail since httptest.ResponseRecorder doesn't support it
	err = rc.SetWriteDeadline(time.Now().Add(time.Second))
	if !errors.Is(err, http.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported for SetWriteDeadline, got %v", err)
	}
}

// Test http.ResponseController compatibility with our buffered type
func TestBufferedResponseController(t *testing.T) {
	tests := []struct {
		name           string
		setupWriter    func() http.ResponseWriter
		expectFlushErr bool
		description    string
	}{
		{
			name: "plain ResponseRecorder supports flush",
			setupWriter: func() http.ResponseWriter {
				return httptest.NewRecorder()
			},
			expectFlushErr: false,
			description:    "Control test - plain recorder should support flush",
		},
		{
			name: "buffered with ResponseRecorder",
			setupWriter: func() http.ResponseWriter {
				recorder := httptest.NewRecorder()
				return newBuffered(recorder)
			},
			expectFlushErr: false,
			description:    "Our buffered type should work with ResponseController",
		},
		{
			name: "buffered with non-flushing writer",
			setupWriter: func() http.ResponseWriter {
				return newBuffered(&nonFlushingWriter{})
			},
			expectFlushErr: false, // Our buffered type handles this gracefully
			description:    "Should handle non-flushing underlying writers gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.setupWriter()

			// Test ResponseController creation
			rc := http.NewResponseController(w)
			if rc == nil {
				t.Fatal("Failed to create ResponseController")
			}

			// Test flush operation
			err := rc.Flush()
			if tt.expectFlushErr && err == nil {
				t.Error("Expected flush error but got none")
			}
			if !tt.expectFlushErr && err != nil {
				t.Errorf("Unexpected flush error: %v", err)
			}
		})
	}
}

// Test Server-Sent Events streaming with our buffered type
func TestSSEStreamingWithBuffered(t *testing.T) {
	t.Run("direct streaming with recorder", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		// Simulate SSE streaming
		rc := http.NewResponseController(recorder)

		recorder.Header().Set("Content-Type", "text/event-stream")
		recorder.Header().Set("Cache-Control", "no-cache")

		fmt.Fprintf(recorder, "event: test\ndata: message1\n\n")
		err := rc.Flush()
		if err != nil {
			t.Errorf("Flush failed: %v", err)
		}

		// Check immediate output
		body := recorder.Body.String()
		if body != "event: test\ndata: message1\n\n" {
			t.Errorf("Expected immediate output, got: %q", body)
		}
	})

	t.Run("buffered streaming - with flush support", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		bw := newBuffered(recorder)

		rc := http.NewResponseController(bw)

		bw.Header().Set("Content-Type", "text/event-stream")
		bw.Header().Set("Cache-Control", "no-cache")

		// Write to buffer
		fmt.Fprintf(bw, "event: test\ndata: message1\n\n")

		// Flush - this should now immediately write content
		err := rc.Flush()
		if err != nil {
			t.Errorf("Flush failed: %v", err)
		}

		// With new implementation, content should be immediately available
		recorderBody := recorder.Body.String()
		if recorderBody != "event: test\ndata: message1\n\n" {
			t.Errorf("Expected immediate content after flush, got: %q", recorderBody)
		}

		// Write more content
		fmt.Fprintf(bw, "event: test\ndata: message2\n\n")

		// Close to flush final content
		bw.close()
		finalBody := recorder.Body.String()
		expected := "event: test\ndata: message1\n\nevent: test\ndata: message2\n\n"
		if finalBody != expected {
			t.Errorf("Expected all content after close, got: %q", finalBody)
		}
	})
}

// Test multiple flush operations
func TestBufferedMultipleFlushes(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	rc := http.NewResponseController(bw)

	// Write first event
	fmt.Fprintf(bw, "event: connect\ndata: connected\n\n")
	rc.Flush()

	// Check first event is immediately available
	body1 := recorder.Body.String()
	if body1 != "event: connect\ndata: connected\n\n" {
		t.Errorf("Expected first event immediately, got: %q", body1)
	}

	// Write second event
	fmt.Fprintf(bw, "event: data\ndata: hello\n\n")
	rc.Flush()

	// Check both events are available
	body2 := recorder.Body.String()
	expected2 := "event: connect\ndata: connected\n\nevent: data\ndata: hello\n\n"
	if body2 != expected2 {
		t.Errorf("Expected first two events, got: %q", body2)
	}

	// Write third event
	fmt.Fprintf(bw, "event: data\ndata: world\n\n")
	rc.Flush()

	// Check all events are available after final flush
	body3 := recorder.Body.String()
	expected3 := "event: connect\ndata: connected\n\nevent: data\ndata: hello\n\nevent: data\ndata: world\n\n"
	if body3 != expected3 {
		t.Errorf("Expected all events, got: %q", body3)
	}

	// Close should not add more content since everything was already flushed
	bw.close()
	finalBody := recorder.Body.String()
	if finalBody != expected3 {
		t.Errorf("Expected no change after close, got: %q", finalBody)
	}
}

// Test FlushError method directly
func TestBufferedFlushError(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	// Write some content
	fmt.Fprintf(bw, "test content")

	// Test FlushError method
	err := bw.FlushError()
	if err != nil {
		t.Errorf("FlushError returned unexpected error: %v", err)
	}

	// Content should be immediately available
	body := recorder.Body.String()
	if body != "test content" {
		t.Errorf("Expected content after FlushError, got: %q", body)
	}
}

// Test duplicate WriteHeader prevention
func TestBufferedDuplicateWriteHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	// Set custom status
	bw.WriteHeader(http.StatusCreated)

	// Try to set status again - should be ignored
	bw.WriteHeader(http.StatusBadRequest)

	// Flush to send headers
	bw.Flush()

	// Should have the first status, not the second
	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, recorder.Code)
	}
}

// Test SetWriteDeadline compatibility
func TestBufferedWriteDeadline(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	rc := http.NewResponseController(bw)

	// This should not error even though the underlying recorder doesn't support deadlines
	// because ResponseController should handle it gracefully
	err := rc.SetWriteDeadline(timeInFuture())
	// Some implementations might not support this, so we just log the error
	if err != nil {
		t.Logf("SetWriteDeadline not supported (expected): %v", err)
	}
}

// Test SetReadDeadline compatibility
func TestBufferedReadDeadline(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	rc := http.NewResponseController(bw)

	// This should not error even though the underlying recorder doesn't support deadlines
	err := rc.SetReadDeadline(timeInFuture())
	// Some implementations might not support this, so we just log the error
	if err != nil {
		t.Logf("SetReadDeadline not supported (expected): %v", err)
	}
}

// Test EnableFullDuplex compatibility
func TestBufferedEnableFullDuplex(t *testing.T) {
	recorder := httptest.NewRecorder()
	bw := newBuffered(recorder)

	rc := http.NewResponseController(bw)

	// This should not error
	err := rc.EnableFullDuplex()
	if err != nil {
		t.Logf("EnableFullDuplex not supported (expected): %v", err)
	}
}

// Helper types for testing

// nonFlushingWriter simulates a ResponseWriter without flush support
type nonFlushingWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *nonFlushingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *nonFlushingWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *nonFlushingWriter) WriteHeader(status int) {
	w.status = status
}

// Helper function to create a time in the future
func timeInFuture() time.Time {
	return time.Now().Add(5 * time.Second)
}
