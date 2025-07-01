package shadow

import (
	"github.com/itchyny/gojq"
	"net/http"
	"testing"
)

func TestHandler_shouldCompare(t *testing.T) {
	type fields struct {
		ComparisonConfig ComparisonConfig
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "no comparisons",
			fields: fields{
				ComparisonConfig: ComparisonConfig{},
			},
			want: false,
		},
		{
			name: "body comparison",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareBody: true,
				},
			},
			want: true,
		},
		{
			name: "headers comparison",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareHeaders: []string{"Content-Encoding", "Content-Type"},
				},
			},
			want: true,
		},
		{
			name: "body comparison",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareStatus: true,
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				ComparisonConfig: tt.fields.ComparisonConfig,
			}
			if got := h.shouldCompare(); got != tt.want {
				t.Errorf("shouldCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_shouldBuffer(t *testing.T) {
	type fields struct {
		ComparisonConfig ComparisonConfig
	}
	type args struct {
		status  int
		headers http.Header
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "happy path",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareBody: true,
				},
			},
			args: args{
				status: 200,
			},
			want: true,
		},
		{
			name: "compressed response",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareBody: true,
				},
			},
			args: args{
				status: 200,
				headers: http.Header{
					"Content-Encoding": []string{"gzip"},
				},
			},
			want: false,
		},
		{
			name: "error status",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					CompareBody: true,
				},
			},
			args: args{
				status: 500,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				ComparisonConfig: tt.fields.ComparisonConfig,
			}
			if got := h.shouldBuffer(tt.args.status, tt.args.headers); got != tt.want {
				t.Errorf("shouldBuffer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_compareJSON(t *testing.T) {
	type fields struct {
		ComparisonConfig ComparisonConfig
	}
	type args struct {
		primaryBS []byte
		shadowBS  []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "string match",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greeting")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greeting": "Hello, world!", "foo": "bar"}`),
				shadowBS:  []byte(`{"greeting": "Hello, world!", "bar": "foo"}`),
			},
			want: true,
		},
		{
			name: "string mismatch",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greeting")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greeting": "Hello, world!"}`),
				shadowBS:  []byte(`{"greeting": "안녕하세요!"}`),
			},
			want: false,
		},
		{
			name: "missing prop in shadow",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greeting")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greeting": "Hello, world!"}`),
				shadowBS:  []byte(`{"foo": "bar"}`),
			},
			want: false,
		},
		{
			name: "object match",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": {"en_US": "Hello, world!"}}`),
				shadowBS:  []byte(`{"greetings": {"en_US": "Hello, world!"}}`),
			},
			want: true,
		},
		{
			name: "primary object, shadow string",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": {"en_US": "Hello, world!"}}`),
				shadowBS:  []byte(`{"greetings": "bar"`),
			},
			want: false,
		},
		{
			name: "object mismatch",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": {"en_US": "Hello, world!"}}`),
				shadowBS:  []byte(`{"greetings": {"ko_KR": "안녕하세요!"}}`),
			},
			want: false,
		},
		{
			name: "array match",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": ["Hello, world!"]}`),
				shadowBS:  []byte(`{"greetings": ["Hello, world!"]}`),
			},
			want: true,
		},
		{
			name: "array mismatch",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": ["Hello, world!", "안녕하세요!"]}`),
				shadowBS:  []byte(`{"greetings": ["안녕하세요!"]}`),
			},
			want: false,
		},
		{
			name: "primary array, shadow string",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": ["Hello, world!", "안녕하세요!"]}`),
				shadowBS:  []byte(`{"greetings": "안녕하세요!"}`),
			},
			want: false,
		},
		{
			name: "primary string, shadow array",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".greetings")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"greetings": "안녕하세요!"}`),
				shadowBS:  []byte(`{"greetings": ["Hello, world!", "안녕하세요!"]}`),
			},
			want: false,
		},
		{
			name: "primary string, shadow bool",
			fields: fields{
				ComparisonConfig: ComparisonConfig{
					compareJQ: []*gojq.Query{
						func() *gojq.Query {
							q, _ := gojq.Parse(".done")
							return q
						}(),
					},
				},
			},
			args: args{
				primaryBS: []byte(`{"done": "true"}`),
				shadowBS:  []byte(`{"done": true}`),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				ComparisonConfig: tt.fields.ComparisonConfig,
			}
			if got := h.compareJSON(tt.args.primaryBS, tt.args.shadowBS); got != tt.want {
				t.Errorf("compareJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}
