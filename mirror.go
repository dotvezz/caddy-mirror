package mirror

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

var (
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func getBuf() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuf(buf *bytes.Buffer) {
	bufferPool.Put(buf)
}

type slogger interface {
	Error(string, ...any)
	Info(string, ...any)
}

// Handler runs multiple handlers and aggregates their results
type Handler struct {
	ComparisonConfig
	ReportingConfig

	MetricsName string `json:"metrics_name"`
	metrics     metrics

	SecondaryRaw       json.RawMessage `json:"secondary"`
	PrimaryRaw         json.RawMessage `json:"primary"`
	secondary, primary caddyhttp.MiddlewareHandler

	Timeout string `json:"secondary_timeout,omitempty"`
	timeout time.Duration

	MirrorRate float64 `json:"mirror_rate,omitempty"`

	slogger slogger
	now     func() time.Time
}

func (h Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mirror",
		New: func() caddy.Module { return new(Handler) },
	}
}

func cloneRequest(r *http.Request) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(
		ctx,
		caddyhttp.VarsCtxKey,
		maps.Clone( // The vars map isn't concurrency safe, so we'll clone it for the mirrored request
			ctx.Value(caddyhttp.VarsCtxKey).(map[string]any),
		),
	)

	return r.Clone(ctx)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) (err error) {
	if !h.shouldMirror() { // Fractional mirroring. If this returns false, we only call primary
		return h.primary.ServeHTTP(w, r, next)
	}

	var primaryBuf, shadowBuf *bytes.Buffer
	if h.shouldCompare() { // Only prepare buffers if we anticipate needing them for secondary response comparison
		primaryBuf, shadowBuf = getBuf(), getBuf()
		defer putBuf(primaryBuf)
		defer putBuf(shadowBuf)
	}

	sr := cloneRequest(r)

	pRecorder := caddyhttp.NewResponseRecorder(w, primaryBuf, h.shouldBuffer)
	sRecorder := caddyhttp.NewResponseRecorder(&NopResponseWriter{}, shadowBuf, h.shouldBuffer)

	if r.Body != nil { // Body is strictly read-once, can't be cloned. So we multiplex it to secondary
		prbuf, srbuf := getBuf(), getBuf()
		defer putBuf(prbuf)
		defer putBuf(srbuf)
		r.Body, sr.Body = duplex(r.Body, prbuf, srbuf)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { // Handle only the secondary request asynchronously
		defer wg.Done()
		sErr := h.requestProcessor("secondary", h.secondary)(sRecorder, sr, next)
		if sErr != nil { // TODO: Make sure that this error is handled as idiomatically and safely as possible
			h.slogger.Error("secondary_handler_error", slog.String("error", sErr.Error()))
		}
	}()

	err = h.requestProcessor("primary", h.primary)(pRecorder, r, next)
	if err != nil {
		return err
	}

	var pBytes []byte
	if pRecorder.Buffered() {
		// We don't want the mirrored request to block sending a response downstream. So here we send the primary response
		// *without* waiting for the secondary request to complete.
		pBytes = pRecorder.Buffer().Bytes()
		w.WriteHeader(pRecorder.Status())
		_, err = w.Write(pBytes)
	}

	if h.shouldCompare() {
		// If we're doing comparison, let's spin up a new goroutine so we can avoid blocking. This way downstream
		// handlers and clients are able to know we're done with our ResponseWriter here.
		go func() {
			// Wait for the mirrored request to complete before attempting to compare.
			wg.Wait()
			var sBytes []byte
			if sRecorder.Buffered() {
				sBytes = sRecorder.Buffer().Bytes()
				h.compareBody(pBytes, sBytes)
			}
			h.compareHeaders(pRecorder.Header(), sRecorder.Header())
			h.compareStatus(pRecorder.Status(), sRecorder.Status())
		}()
	}

	return err
}

func (h *Handler) requestProcessor(name string, inner caddyhttp.MiddlewareHandler) func(wr http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return func(wr http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
		// Even though there may be a timeout provided by another handler, we really want to make sure we keep our
		// goroutines tidy. We're enforcing a timeout on all request processing as mitigation for the possibility of
		// goroutine leaks and connection leaks.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), h.timeout)
		defer cancel()
		r = r.WithContext(ctx)
		startedAt := h.now()
		if h.MetricsName != "" {
			if r.Body != nil {
				// Since the primary and secondary request bodies are sent through a tee, it's unfair to compare response
				// timing using the original startedAt value. The secondary request body can never be fully transmitted
				// before the primary, introducing unintended skew to the metrics.
				//
				// This FinishReadCloser tries to make metrics fair for primary and secondary handlers by re-setting
				// the startedAt value at the time the request body is fully transmitted.
				r.Body = NewFinishReadCloser(r.Body, func() {
					startedAt = h.now()
				})
			}

			// TimedWriter lets us capture the time when we first start receiving a response body, and the time when we
			// first receive a response status, allowing us to track time to first byte.
			wr = NewTimedWriter(wr, func() {
				h.metrics.ttfb[name].Observe(time.Since(startedAt).Seconds())
			})
		}
		err := inner.ServeHTTP(wr, r, next)
		if h.MetricsName != "" {
			h.metrics.totalTime[name].Observe(time.Since(startedAt).Seconds())
		}
		if err != nil {
			h.slogger.Error(name+"_handler_error", slog.String("error", err.Error()))
		}
		return err
	}
}

func (h *Handler) shouldMirror() bool {
	switch h.MirrorRate {
	case 1:
		return true
	case 0:
		return true
	case -1:
		return false
	default:
		return rand.Float64() < h.MirrorRate
	}
}
