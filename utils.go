package structpages

import (
	"bytes"
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

type buffered struct {
	http.ResponseWriter
	buf *bytes.Buffer
}

func newBuffered(w http.ResponseWriter) buffered {
	return buffered{ResponseWriter: w, buf: getBuffer()}
}

func (w buffered) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w buffered) close() error {
	_, err := w.ResponseWriter.Write(w.buf.Bytes())
	defer releaseBuffer(w.buf)
	return err
}
