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
	"sync"
	"testing"
	"time"
)

type sloggerMock struct {
	err  func(str string, in ...any)
	info func(str string, in ...any)
}

func (s *sloggerMock) Error(str string, in ...any) {
	if s.err != nil {
		s.err(str, in...)
	}
}

func (s *sloggerMock) Info(str string, in ...any) {
	if s.info != nil {
		s.info(str, in...)
	}
}

type middlewareHandlerFunc func(http.ResponseWriter, *http.Request, caddyhttp.Handler) error

func (f middlewareHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return f(w, r, next)
}

func TestHandler_ServeHTTP(t *testing.T) {
	hnd := &Handler{
		ComparisonConfig: ComparisonConfig{
			CompareBody: true,
		},
		primary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hi, world!"))
			return nil
		}),
		secondary: middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello, world!"))
			return nil
		}),
		slogger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	r = r.WithContext(context.WithValue(r.Context(), caddyhttp.VarsCtxKey, make(map[string]any)))

	mismatched := false
	wg := sync.WaitGroup{}
	wg.Add(1)

	slog := &sloggerMock{
		info: func(str string, in ...any) {
			defer wg.Done()
			mismatched = true
		},
	}

	hnd.slogger = slog
	hnd.now = time.Now

	hnd.ServeHTTP(&NopResponseWriter{}, r, nil)
	wg.Wait()

	if !mismatched {
		t.Errorf("Expected mismatched to be true")
	}
}

func TestHandler_ServeHTTPWithComparisons(t *testing.T) {
	type fields struct {
		secondary func(done func()) caddyhttp.MiddlewareHandler
		primary   func(done func()) caddyhttp.MiddlewareHandler
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
				secondary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "GET" {
							return fmt.Errorf("expected GET, got %s", r.Method)
						}
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("Hello, world!"))
						return nil
					})
				},
				primary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "GET" {
							return fmt.Errorf("expected GET, got %s", r.Method)
						}
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("Hello, world!"))
						return nil
					})
				},
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
				secondary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "POST" {
							return fmt.Errorf("expected POST, got %s", r.Method)
						}
						bs, err := io.ReadAll(r.Body)
						if err != nil {
							return nil
						}
						if string(bs) != "Hello, world!" {
							return fmt.Errorf("expected Hello, world!, got %s", string(bs))
						}
						w.WriteHeader(http.StatusOK)
						w.Write(bs)
						return nil
					})
				},
				primary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "POST" {
							return fmt.Errorf("expected POST, got %s", r.Method)
						}
						bs, err := io.ReadAll(r.Body)
						if err != nil {
							return nil
						}
						if string(bs) != "Hello, world!" {
							return fmt.Errorf("expected Hello, world!, got %s", string(bs))
						}
						w.WriteHeader(http.StatusOK)
						w.Write(bs)
						return nil
					})
				},
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
		{
			name: "negative test post",
			fields: fields{
				secondary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "POST" {
							return fmt.Errorf("expected POST, got %s", r.Method)
						}
						bs, err := io.ReadAll(r.Body)
						if err != nil {
							return nil
						}
						if string(bs) != "Hello, world!" {
							return fmt.Errorf("expected Hello, world!, got %s", string(bs))
						}
						w.WriteHeader(http.StatusOK)
						w.Write(bs)
						return nil
					})
				},
				primary: func(done func()) caddyhttp.MiddlewareHandler {
					return middlewareHandlerFunc(func(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
						defer done()
						if r.Method != "POST" {
							return fmt.Errorf("expected POST, got %s", r.Method)
						}
						bs, err := io.ReadAll(r.Body)
						if err != nil {
							return nil
						}
						if string(bs) != "Oh hi, world!" {
							return fmt.Errorf("expected Hello, world!, got %s", string(bs))
						}
						w.WriteHeader(http.StatusOK)
						w.Write(bs)
						return nil
					})
				},
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(2)
			h := &Handler{
				ComparisonConfig: ComparisonConfig{
					CompareBody: true,
				},
				secondary: tt.fields.secondary(wg.Done),
				primary:   tt.fields.primary(wg.Done),
				timeout:   time.Hour, // Just an absurdly long timeout to make it easy to debug with delve
				now:       time.Now,
				slogger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
			}

			if err := h.ServeHTTP(tt.args.w, tt.args.r, tt.args.next); (err != nil) != tt.wantErr {
				t.Errorf("ServeHTTP() error = %v, wantErr %v", err, tt.wantErr)
			}
			wg.Wait()
		})
	}

	time.Sleep(time.Second)
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
					r.Body.Close()
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
					r.Body.Close()
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
	// Wait a bit for goroutines
	time.Sleep(time.Second)
}
