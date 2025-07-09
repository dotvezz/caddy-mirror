package mirror

import (
	"bytes"
	"io"
	"net/http"
)

type FinishReader struct {
	io.ReadCloser

	onFinished func()
}

func NewFinishReadCloser(r io.ReadCloser, onFinished func()) *FinishReader {
	return &FinishReader{
		ReadCloser: r,
		onFinished: onFinished,
	}
}

func (f *FinishReader) Read(p []byte) (n int, err error) {
	n, err = f.ReadCloser.Read(p)

	if err != nil {
		f.onFinished()
		f.onFinished = noop
	}

	return
}

func (f *FinishReader) Close() error {
	f.onFinished()
	f.onFinished = noop

	return f.ReadCloser.Close()
}

func (f *FinishReader) OnFinished(callback func()) {
	f.onFinished = callback
}

type TimedWriter struct {
	http.ResponseWriter

	onStarted func()
}

func NewTimedWriter(w http.ResponseWriter, onStart func()) http.ResponseWriter {
	return &TimedWriter{
		ResponseWriter: w,
		onStarted:      onStart,
	}
}

func (f *TimedWriter) started() {
	// Some branchless programming hackery here. To avoid needing a branch on any kind of "already started" evaluation,
	// just always call f.onStarted and replace it with a noop. On the first write, it'll invoke the provided callback,
	// then on later writes it'll noop
	f.onStarted()
	f.onStarted = noop
}

func (f *TimedWriter) Write(p []byte) (n int, err error) {
	// This is when we want to capture TTFB
	f.started()
	return f.ResponseWriter.Write(p)
}

func (f *TimedWriter) WriteHeader(status int) {
	// We're also calling f.started here just in case f.Write was not used (for default 200 behavior, etc)
	f.started()
	f.ResponseWriter.WriteHeader(status)
}

func noop() {}

type NopResponseWriter struct {
	header http.Header
	status int
}

func (w *NopResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *NopResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *NopResponseWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

type PassthroughCloser struct {
	io.Reader
	io.Closer
}

// duplex duplicates a single io.ReadCloser across two io.ReadClosers
// TODO: Strategy to reduce or eliminate the need for the buffers
func duplex(r io.ReadCloser, pbuf, sbuf *bytes.Buffer) (io.ReadCloser, io.ReadCloser) {
	defer r.Close()
	io.Copy(pbuf, r)
	sbuf.Write(pbuf.Bytes())
	return io.NopCloser(pbuf), io.NopCloser(sbuf)
}
