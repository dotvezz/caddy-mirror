package mirror

import (
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/itchyny/gojq"
)

type LogLevel string

type JQQuery string

type ComparisonConfig struct {
	CompareStatus  bool      `json:"compare_status,omitempty"`
	CompareBody    bool      `json:"compare_body,omitempty"`
	CompareHeaders []string  `json:"compare_headers,omitempty"`
	CompareJQ      []JQQuery `json:"compare_jq,omitempty"`
	compareJQ      []*gojq.Query
}

type ReportingConfig struct {
	NoLog    bool      `json:"no_log,omitempty"`
	LogLevel *LogLevel `json:"log_level,omitempty"`
}

func (h *Handler) compareStatus(primaryStatus, shadowStatus int) {
	if primaryStatus != shadowStatus {
		h.slogger.Info("shadow_status_mismatch",
			slog.Int("primary_status", primaryStatus),
			slog.Int("shadow_status", shadowStatus),
		)
	}
}

func (h *Handler) compareHeaders(primaryH, shadowH http.Header) {
	for _, k := range h.CompareHeaders {
		ph, sh := primaryH.Values(k), shadowH.Values(k)
		if slices.Equal(ph, sh) {
			h.slogger.Info(
				"shadow_header_mismatch",
				slog.String("key", k),
				slog.Any("primary_values", ph),
				slog.Any("shadow_values", sh),
			)
		}
	}
}

func (h *Handler) compareBody(primaryBS, shadowBS []byte) {
	var match bool
	if h.CompareJQ != nil {
		match = h.compareJSON(primaryBS, shadowBS)
	} else {
		match = slices.Equal(primaryBS, shadowBS)
	}

	if h.MetricsName != "" {
		if match {
			h.metrics.match.Inc()
		} else {
			h.metrics.mismatch.Inc()
		}
	}

	if match { // If we've matched, nothing left to do
		return
	}

	if !h.NoLog {
		h.slogger.Info("shadow_mismatch",
			"primary_body", string(primaryBS),
			"shadow_body", string(shadowBS),
		)
	}
}

func (h *Handler) compareJSON(primaryBS, shadowBS []byte) bool {
	for _, jq := range h.compareJQ {
		var primary, shadow any
		_ = json.Unmarshal(primaryBS, &primary)
		_ = json.Unmarshal(shadowBS, &shadow)

		pi, si := jq.Run(primary), jq.Run(shadow)
		// These iterators should never be nil but just to be safe...
		// If both iterators are nil, something is REALLY unexpected, but *technically* that's a match
		if pi == nil && si == nil {
			continue
		}
		// If only one iterator is nil, something is REALLY unexpected, but *technically* that's a mismatch
		if (pi == nil) != (si == nil) {
			return false
		}

		for {
			pn, pok := pi.Next()
			sn, sok := si.Next()
			if sok != pok {
				// If the iterators have a different result length, that's a mismatch
				return false
			}
			if !pok {
				break
			}

			switch pn.(type) {
			case map[string]any:
				pm := pn.(map[string]any)
				sm, ok := sn.(map[string]any)
				if !ok {
					return false
				}

				if !maps.Equal(pm, sm) {
					return false
				}
			case []any:
				psl := pn.([]any)
				ssl, ok := sn.([]any)
				if !ok {
					return false
				}

				if !slices.Equal(psl, ssl) {
					return false
				}
			default:
				if pn != sn {
					return false
				}
			}
		}
	}

	return true
}

func (h *Handler) shouldBuffer(status int, hdr http.Header) bool {
	return status >= 200 &&
		status < 300 &&
		h.shouldCompare() &&
		hdr.Get("Content-Encoding") == ""
}

func (h *Handler) shouldCompare() bool {
	return h.CompareBody ||
		len(h.compareJQ) > 0 ||
		h.CompareStatus ||
		len(h.CompareHeaders) > 0
}
