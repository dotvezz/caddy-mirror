package mirror

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// benchHandlerFunc implements caddyhttp.MiddlewareHandler for benchmarks
// without depending on other test files.
type benchHandlerFunc func(http.ResponseWriter, *http.Request, caddyhttp.Handler) error

func (f benchHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return f(w, r, next)
}

// nullLogger is a no-op slogger to avoid benchmark noise.
type nullLogger struct{}

func (nullLogger) Error(string, ...any) {}
func (nullLogger) Info(string, ...any)  {}

func makeHandler(mirrorRate float64, compareBody bool) *Handler {
	h := &Handler{
		ComparisonConfig: ComparisonConfig{
			CompareBody: compareBody,
		},
		MirrorRate: mirrorRate,
		slogger:    nullLogger{},
		now:        time.Now,
		// Avoid zero timeout; we don't call Provision() in benchmarks
		timeout: 30 * time.Second,
	}

	// Keep primary and secondary fast and deterministic.
	primaryBody := []byte("OK")
	h.primary = benchHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(primaryBody)
		return nil
	})
	h.secondary = benchHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
		w.WriteHeader(http.StatusOK)
		// For CompareBody benchmarks, keep secondary identical to primary to avoid logging work.
		_, _ = w.Write(primaryBody)
		return nil
	})

	return h
}

func makeRequest(withBody bool) *http.Request {
	var body io.ReadCloser
	var r *http.Request
	if withBody {
		body = io.NopCloser(strings.NewReader(strings.Repeat("a", 1024*1024*20)))
		r, _ = http.NewRequest("POST", "http://example.com", body)
	} else {
		r, _ = http.NewRequest("GET", "http://example.com", body)
	}
	r = r.WithContext(context.WithValue(r.Context(), caddyhttp.VarsCtxKey, make(map[string]any)))
	return r
}

func BenchmarkServeHTTP_PrimaryOnly_NoBody(b *testing.B) {
	h := makeHandler(-1, false) // disable mirroring
	w := &NopResponseWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := makeRequest(false)
		_ = h.ServeHTTP(w, r, nil)
	}
}

func BenchmarkServeHTTP_Mirror_NoCompare_NoBody(b *testing.B) {
	h := makeHandler(1, false)
	w := &NopResponseWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := makeRequest(false)
		_ = h.ServeHTTP(w, r, nil)
	}
}

func BenchmarkServeHTTP_Mirror_CompareBody_NoBody(b *testing.B) {
	h := makeHandler(1, true)
	w := &NopResponseWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := makeRequest(false)
		_ = h.ServeHTTP(w, r, nil)
	}
}

func BenchmarkServeHTTP_Mirror_NoCompare_WithBody(b *testing.B) {
	h := makeHandler(1, false)
	w := &NopResponseWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := makeRequest(true)
		_ = h.ServeHTTP(w, r, nil)
	}
}

func BenchmarkServeHTTP_Mirror_CompareBody_WithBody(b *testing.B) {
	h := makeHandler(1, true)
	w := &NopResponseWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := makeRequest(true)
		_ = h.ServeHTTP(w, r, nil)
	}
}
