package shadow

import (
	"fmt"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/itchyny/gojq"

	"github.com/prometheus/client_golang/prometheus"
)

// Provision implements caddy.Provisioner
func (h *Handler) Provision(ctx caddy.Context) (err error) {
	err = h.provisionHandlers(ctx)
	if err != nil {
		return
	}

	h.slogger = ctx.Slogger()

	h.now = time.Now

	if len(h.CompareJQ) > 0 {
		h.compareJQ = make([]*gojq.Query, len(h.CompareJQ))
		for i, qStr := range h.CompareJQ {
			h.compareJQ[i], err = gojq.Parse(string(qStr))
			if err != nil {
				return fmt.Errorf("error parsing jq query %d: %w", i, err)
			}
		}
	}

	h.timeout = 30 * time.Second
	if h.Timeout != "" {
		h.timeout, err = time.ParseDuration(h.Timeout)
		if err != nil {
			return fmt.Errorf("error parsing timeout: %w", err)
		}
	}

	if h.MetricsName != "" {
		// If metrics are enabled, assume that always includes basic performance metrics
		h.metrics.provision(ctx, h.MetricsName)
	}

	// Add metrics for comparisons if enabled
	if h.CompareBody || h.CompareJQ != nil {
		h.metrics.match = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: h.MetricsName,
			Name:      "shadow_body_match",
			Help:      "Number of responses that matched",
		})
		h.metrics.mismatch = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: h.MetricsName,
			Name:      "shadow_body_mismatch",
			Help:      "Number of responses that did not match",
		})
		_ = ctx.GetMetricsRegistry().Register(h.metrics.match)
		_ = ctx.GetMetricsRegistry().Register(h.metrics.mismatch)
	}

	return nil
}

func (h *Handler) provisionHandlers(ctx caddy.Context) (err error) {
	var mod any
	mod, err = ctx.LoadModuleByID("http.handlers.subroute", h.ShadowRaw)
	if err != nil {
		return fmt.Errorf("error loading shadow module: %w", err)
	}
	h.shadow = mod.(caddyhttp.MiddlewareHandler)
	mod, err = ctx.LoadModuleByID("http.handlers.subroute", h.PrimaryRaw)
	if err != nil {
		return fmt.Errorf("error loading primary module: %w", err)
	}
	h.primary = mod.(caddyhttp.MiddlewareHandler)

	if provisioner, ok := h.shadow.(caddy.Provisioner); ok {
		err = provisioner.Provision(ctx)
		if err != nil {
			return fmt.Errorf("error provisioning shadow: %w", err)
		}
	}
	if provisioner, ok := h.primary.(caddy.Provisioner); ok {
		err = provisioner.Provision(ctx)
		if err != nil {
			return fmt.Errorf("error provisioning shadow: %w", err)
		}
	}

	return nil
}
