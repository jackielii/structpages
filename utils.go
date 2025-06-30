package structpages

import (
	"bytes"
	"cmp"
	"net/http"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func releaseBuffer(b *bytes.Buffer) {
	b.Reset()
	bufferPool.Put(b)
}

// buffered wraps an http.ResponseWriter to buffer the response body and status code.
// It implements the Unwrap method to support http.ResponseController.
type buffered struct {
	http.ResponseWriter
	status     int
	buf        *bytes.Buffer
	headerSent bool
	statusSet  bool
}

func newBuffered(w http.ResponseWriter) *buffered {
	return &buffered{ResponseWriter: w, buf: getBuffer(), status: http.StatusOK}
}

func (w *buffered) Write(b []byte) (int, error) { return w.buf.Write(b) }

func (w *buffered) WriteHeader(statusCode int) {
	if w.headerSent || w.statusSet {
		return // Prevent duplicate WriteHeader calls
	}
	w.status = statusCode
	w.statusSet = true
}

func (w *buffered) Status() int { return cmp.Or(w.status, http.StatusOK) }

// Flush immediately writes any buffered content to the underlying ResponseWriter.
// This enables streaming scenarios like Server-Sent Events (SSE).
func (w *buffered) Flush() {
	if !w.headerSent {
		w.ResponseWriter.WriteHeader(w.status)
		w.headerSent = true
	}

	if w.buf.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buf.Bytes())
		w.buf.Reset()
	}

	// Try to flush underlying writer if it supports it
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// FlushError immediately writes any buffered content and returns any error.
// This method is used by http.ResponseController for streaming scenarios.
func (w *buffered) FlushError() error {
	if !w.headerSent {
		w.ResponseWriter.WriteHeader(w.status)
		w.headerSent = true
	}

	var err error
	if w.buf.Len() > 0 {
		_, err = w.ResponseWriter.Write(w.buf.Bytes())
		w.buf.Reset()
	}

	// Try to flush underlying writer if it supports it
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	return err
}

// Unwrap returns the underlying ResponseWriter, allowing http.ResponseController
// to access extended functionality like Hijack, etc.
func (w *buffered) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *buffered) close() error {
	if !w.headerSent {
		w.ResponseWriter.WriteHeader(w.status)
		w.headerSent = true
	}
	_, err := w.ResponseWriter.Write(w.buf.Bytes())
	releaseBuffer(w.buf)
	return err
}
