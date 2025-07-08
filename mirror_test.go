package mirror

import (
	"context"
	"fmt"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type middlewareHandlerFunc func(http.ResponseWriter, *http.Request, caddyhttp.Handler) error

func (f middlewareHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return f(w, r, next)
}

func TestHandler_ServeHTTPNoComparison(t *testing.T) {
	type fields struct {
		secondary caddyhttp.MiddlewareHandler
		primary   caddyhttp.MiddlewareHandler
	}
	type args struct {
		w    http.ResponseWriter
		r    *http.Request
		next caddyhttp.Handler
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "simple get",
			fields: fields{
				secondary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
					if r.Method != "GET" {
						return fmt.Errorf("expected GET, got %s", r.Method)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, world!"))
					return nil
				}),
				primary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
					if r.Method != "GET" {
						return fmt.Errorf("expected GET, got %s", r.Method)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, world!"))
					return nil
				}),
			},
			args: args{
				w: &NopResponseWriter{
					header: make(http.Header),
					status: 0,
				},
				r: func() *http.Request {
					r, _ := http.NewRequest("GET", "http://example.com", nil)
					r = r.WithContext(context.WithValue(r.Context(), caddyhttp.VarsCtxKey, make(map[string]any)))
					return r
				}(),
				next: caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return nil
				}),
			},
		},
		{
			name: "simple post",
			fields: fields{
				secondary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
					if r.Method != "POST" {
						return fmt.Errorf("expected GET, got %s", r.Method)
					}
					bs, err := io.ReadAll(r.Body)
					if err != nil {
						return nil
					}
					if string(bs) != "Hello, world!" {
						return fmt.Errorf("expected Hello, world!, got %s", string(bs))
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, world!"))
					return nil
				}),
				primary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
					if r.Method != "POST" {
						return fmt.Errorf("expected GET, got %s", r.Method)
					}
					bs, err := io.ReadAll(r.Body)
					if err != nil {
						return nil
					}
					if string(bs) != "Hello, world!" {
						return fmt.Errorf("expected Hello, world!, got %s", string(bs))
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, world!"))
					return nil
				}),
			},
			args: args{
				w: &NopResponseWriter{
					header: make(http.Header),
					status: 0,
				},
				r: func() *http.Request {
					r, _ := http.NewRequest("POST", "http://example.com", strings.NewReader("Hello, world!"))
					r = r.WithContext(context.WithValue(r.Context(), caddyhttp.VarsCtxKey, make(map[string]any)))
					return r
				}(),
				next: caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return nil
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				secondary: tt.fields.secondary,
				primary:   tt.fields.primary,
				timeout:   time.Hour, // Just an absurdly long timeout to make it easy to debug with delve
				now:       time.Now,
				slogger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
			}
			if err := h.ServeHTTP(tt.args.w, tt.args.r, tt.args.next); (err != nil) != tt.wantErr {
				t.Errorf("ServeHTTP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
