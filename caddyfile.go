package shadow

import (
	"encoding/json"
	"fmt"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Handler{})
	httpcaddyfile.RegisterHandlerDirective("shadow", ParseCaddyfile)
}

func ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	hnd := new(Handler)
	h.Next()
	for h.NextBlock(0) {
		handlerName := h.Val()
		switch handlerName {
		case "primary", "shadow":
			innerHnd, err := httpcaddyfile.ParseSegmentAsSubroute(h.WithDispenser(h.NewFromNextSegment()))
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling %s: %w", handlerName, err)
			}

			if handlerName == "primary" {
				hnd.PrimaryRaw, err = json.Marshal(innerHnd)
				if err != nil {
					return nil, fmt.Errorf("error marshaling %s: %w", handlerName, err)
				}
			} else {
				hnd.ShadowRaw, err = json.Marshal(innerHnd)
				if err != nil {
					return nil, fmt.Errorf("error marshaling %s: %w", handlerName, err)
				}
			}
		case "compare_body":
			hnd.ComparisonConfig.CompareBody = true
		case "compare_status":
			hnd.ComparisonConfig.CompareStatus = true
		case "compare_headers":
			hnd.ComparisonConfig.CompareHeaders = h.RemainingArgs()
		case "compare_jq":
			args := h.RemainingArgs()
			if len(args) < 1 {
				return nil, fmt.Errorf("compare_jq requires at least one jq query")
			}
			for _, qStr := range args {
				hnd.ComparisonConfig.CompareJQ = append(hnd.ComparisonConfig.CompareJQ, JQQuery(qStr))
			}
		case "no_log":
			hnd.ReportingConfig.NoLog = true
		case "log_level":
			args := h.RemainingArgs()
			if len(args) < 1 {
				return nil, fmt.Errorf("log_level requires a log level")
			}
			ll := LogLevel(args[0])
			hnd.ReportingConfig.LogLevel = &ll
		case "metrics":
			args := h.RemainingArgs()
			if len(args) < 1 {
				return nil, fmt.Errorf("metrics requires a prefix/namespace")
			}
			hnd.MetricsName = args[0]
		case "timeout":
			args := h.RemainingArgs()
			if len(args) < 1 {
				return nil, fmt.Errorf("timeout requires duration")
			}
			hnd.Timeout = args[0]
		}
	}

	if hnd.PrimaryRaw == nil {
		return nil, fmt.Errorf("primary handler is required")
	}
	if hnd.ShadowRaw == nil {
		return nil, fmt.Errorf("shadow handler is required")
	}
	return hnd, nil
}
