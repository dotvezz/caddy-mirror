package shadow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
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

var (
	bufferPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
)

// Handler runs multiple handlers and aggregates their results
type Handler struct {
	ComparisonConfig
	ReportingConfig

	MetricsName string `json:"metrics_name"`
	metrics     metrics

	ShadowRaw       json.RawMessage `json:"shadow"`
	PrimaryRaw      json.RawMessage `json:"primary"`
	shadow, primary caddyhttp.MiddlewareHandler

	Timeout string `json:"timeout,omitempty"`
	timeout time.Duration

	slogger *slog.Logger
	now     func() time.Time
}

func (h Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.shadow",
		New: func() caddy.Module { return new(Handler) },
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) (err error) {
	primaryCtx := r.Context()

	// The vars map isn't concurrency safe, so we'll clone it for the shadowed request
	shadowCtx := context.WithValue(
		primaryCtx,
		caddyhttp.VarsCtxKey,
		maps.Clone(primaryCtx.Value(caddyhttp.VarsCtxKey).(map[string]any)),
	)

	{ // Make sure that both request contexts get timeouts and cancel when we're done with them
		var cancelShadow, cancelPrimary context.CancelFunc
		shadowCtx, cancelShadow = context.WithTimeout(shadowCtx, h.timeout)
		primaryCtx, cancelPrimary = context.WithTimeout(primaryCtx, h.timeout)
		defer cancelShadow()
		defer cancelPrimary()
	}

	var primaryBuf, shadowBuf *bytes.Buffer
	if h.shouldCompare() { // Only prepare buffers if we anticipate needing them for response comparison
		primaryBuf = bufferPool.Get().(*bytes.Buffer)
		shadowBuf = bufferPool.Get().(*bytes.Buffer)
		defer bufferPool.Put(primaryBuf)
		defer bufferPool.Put(shadowBuf)
		primaryBuf.Reset()
		shadowBuf.Reset()
	}

	pRecorder := caddyhttp.NewResponseRecorder(w, primaryBuf, h.shouldBuffer)
	sRecorder := caddyhttp.NewResponseRecorder(&NopResponseWriter{}, shadowBuf, h.shouldBuffer)

	// Clone the request to help ensure that concurrent upstream handlers don't step on each other
	pr := r.Clone(primaryCtx)
	sr := r.Clone(shadowCtx)

	if r.Body != nil { // Body is strictly read-once, can't be cloned. So we multiplex it to shadow with io.TeeReader
		reqBuf := bufferPool.Get().(*bytes.Buffer)
		reqBuf.Reset()
		defer bufferPool.Put(reqBuf)

		tee := io.TeeReader(r.Body, reqBuf)
		pr.Body = io.NopCloser(tee)
		sr.Body = io.NopCloser(reqBuf)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { // Handle only the shadowed request asynchronously
		defer wg.Done()
		sErr := h.requestProcessor("shadow", h.shadow)(sRecorder, sr, next)
		if sErr != nil { // TODO: Make sure that this error is handled as idiomatically and safely as possible
			h.slogger.Error("shadow_handler_error", slog.String("error", sErr.Error()))
		}
	}()

	err = h.requestProcessor("primary", h.primary)(pRecorder, pr, next)
	if err != nil {
		return err
	}

	var pBytes []byte
	if pRecorder.Buffered() {
		// We don't want the shadowed request to block sending any response downstream. So here we send the primary response
		// without waiting for the shadowed request.
		pBytes = pRecorder.Buffer().Bytes()
		w.WriteHeader(pRecorder.Status())

		// I think the best thing to do is sit on this error until we've finished handling the shadowed request.
		// A failure to write downstream (client disconnect, etc) doesn't reflect on the shadowed handler and shouldn't be
		// allowed to impact handling the shadowed request, getting metrics, doing comparisons, etc.
		_, err = w.Write(pBytes)
	}

	// Even if we're not comparing responses, we don't want to let the shadowed request context get prematurely
	// canceled, so we'll wait for it to finish before moving on.
	wg.Wait()

	if h.shouldCompare() {
		var sBytes []byte
		if sRecorder.Buffered() {
			sBytes = sRecorder.Buffer().Bytes()
		}
		h.compare(pBytes, sBytes)
	}

	return err
}

func (h *Handler) requestProcessor(name string, inner caddyhttp.MiddlewareHandler) func(wr http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return func(wr http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
		startedAt := h.now()
		if h.MetricsName != "" {
			if r.Body != nil {
				// Since the primary and shadow request bodies are sent through a tee, it's unfair to compare response
				// timing using the original startedAt value. The shadow request body can never be fully transmitted
				// before the primary, introducing unintended skew to the metrics.
				//
				// This FinishReadCloser lets us decouple the timing of primary and shadow handlers by re-setting
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

func (h *Handler) shouldBuffer(status int, _ http.Header) bool {
	return status >= 200 && status < 300 && h.shouldCompare()
}

func (h *Handler) shouldCompare() bool {
	return h.CompareBody || len(h.compareJQ) > 0
}
