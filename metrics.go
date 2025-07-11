package mirror

import (
	"time"

	"github.com/caddyserver/caddy/v2"

	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	ttfb            map[string]prometheus.Histogram
	totalTime       map[string]prometheus.Histogram
	match, mismatch prometheus.Counter
}

const millisecond = float64(time.Millisecond) / float64(time.Second)

func (m *metrics) provision(ctx caddy.Context, name string) {
	m.ttfb = make(map[string]prometheus.Histogram, 2)
	m.ttfb["primary"] = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: name,
		Name:      "primary_time_to_first_byte_seconds",
		Help:      "Number of seconds before first byte of response from primary",
		Buckets:   prometheus.ExponentialBuckets(millisecond, 2, 16),
	})
	ctx.GetMetricsRegistry().Register(m.ttfb["primary"])
	m.ttfb["secondary"] = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: name,
		Name:      "shadow_time_to_first_byte_seconds",
		Help:      "Number of seconds before first byte of response from secondary",
		Buckets:   prometheus.ExponentialBuckets(millisecond, 2, 16),
	})
	ctx.GetMetricsRegistry().Register(m.ttfb["secondary"])

	m.totalTime = make(map[string]prometheus.Histogram, 2)
	m.totalTime["primary"] = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: name,
		Name:      "primary_total_time_seconds",
		Help:      "Number of seconds for full response from primary",
		Buckets:   prometheus.ExponentialBuckets(millisecond*2, 2, 16),
	})
	ctx.GetMetricsRegistry().Register(m.totalTime["primary"])
	m.totalTime["secondary"] = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: name,
		Name:      "shadow_total_time_seconds",
		Help:      "Number of seconds for full response from secondary",
		Buckets:   prometheus.ExponentialBuckets(millisecond*2, 2, 16),
	})
	ctx.GetMetricsRegistry().Register(m.totalTime["secondary"])
}
