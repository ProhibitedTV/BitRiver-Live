package metrics

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"time"
)

// ResponseRecorder wraps an http.ResponseWriter to capture the final status
// code while preserving optional interfaces like Hijacker and Flusher.
type ResponseRecorder struct {
	http.ResponseWriter
	status int
}

// NewResponseRecorder constructs a ResponseRecorder defaulting the status code
// to 200 OK when WriteHeader is not invoked by the handler.
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{ResponseWriter: w, status: http.StatusOK}
}

// Status exposes the last status code written to the response.
func (rr *ResponseRecorder) Status() int {
	return rr.status
}

// WriteHeader captures the status code before delegating to the underlying
// ResponseWriter.
func (rr *ResponseRecorder) WriteHeader(status int) {
	rr.status = status
	rr.ResponseWriter.WriteHeader(status)
}

// Flush flushes the response when supported by the underlying writer.
func (rr *ResponseRecorder) Flush() {
	if flusher, ok := rr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack preserves HTTP/1.1 connection hijacking when available.
func (rr *ResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rr.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push forwards HTTP/2 server push support to the underlying writer.
func (rr *ResponseRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rr.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// CloseNotify keeps backwards compatibility with deprecated CloseNotifier.
//
//nolint:staticcheck // CloseNotifier remains to support legacy HTTP/1.1 clients.
func (rr *ResponseRecorder) CloseNotify() <-chan bool {
	if notifier, ok := rr.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	return nil
}

// ReadFrom streams data efficiently when supported by the underlying writer.
func (rr *ResponseRecorder) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := rr.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(r)
	}
	return io.Copy(rr.ResponseWriter, r)
}

// HTTPMiddleware records request metrics around the provided handler using the
// supplied recorder (falling back to metrics.Default when nil).
func HTTPMiddleware(recorder *Recorder, next http.Handler) http.Handler {
	rec := recorder
	if rec == nil {
		rec = Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := NewResponseRecorder(w)
		start := time.Now()
		next.ServeHTTP(rr, r)
		rec.ObserveRequest(r.Method, r.URL.Path, rr.Status(), time.Since(start))
	})
}
