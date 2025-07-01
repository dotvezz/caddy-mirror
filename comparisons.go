package shadow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/itchyny/gojq"
)

type LogLevel string

const levelDebug LogLevel = "debug"
const levelInfo LogLevel = "info"
const levelError LogLevel = "error"

func (ll *LogLevel) UnmarshalJSON(b []byte) error {
	if !slices.Contains([]LogLevel{levelDebug, levelInfo, levelError}, LogLevel(b)) {
		return fmt.Errorf("unknown log level: %s", string(b))
	}

	*ll = LogLevel(b)
	return nil
}

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

func (h *Handler) compare(primaryBS, shadowBS []byte) {
	var match bool
	if h.CompareJQ != nil {
		match = h.compareJSON(primaryBS, shadowBS)
	} else {
		match = slices.Equal(primaryBS, shadowBS)
	}

	if match {
		h.metrics.match.Inc()
		return // no need to do anything else if we have a match
	}

	h.metrics.mismatch.Inc()

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

func (h *Handler) reportMismatch(primaryBS, shadowBS []byte) {

}
