package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.xchunk.org/argon/auth"
)

func textHandler(s string) Handler {
	return func(context.Context, *Request) (*Result, error) {
		return &Result{Content: []Content{{Text: s}}}, nil
	}
}

func TestMaxOutput(t *testing.T) {
	ctx := context.Background()
	req := &Request{Tool: "t"}
	tests := []struct {
		name    string
		limit   int
		res     *Result
		wantErr bool
	}{
		{"under", 10, &Result{Content: []Content{{Text: "12345"}}}, false},
		{"exact", 5, &Result{Content: []Content{{Text: "12345"}}}, false},
		{"over text", 4, &Result{Content: []Content{{Text: "12345"}}}, true},
		{"over structured", 10, &Result{Structured: map[string]string{"k": strings.Repeat("x", 20)}}, true},
		{"disabled", 0, &Result{Content: []Content{{Text: strings.Repeat("x", 1000)}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := MaxOutput(tt.limit)(func(context.Context, *Request) (*Result, error) { return tt.res, nil })
			_, err := h(ctx, req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChainOrder(t *testing.T) {
	var trace []string
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, r *Request) (*Result, error) {
				trace = append(trace, name)
				return next(ctx, r)
			}
		}
	}
	h := Chain(textHandler("ok"), mw("a"), mw("b"), mw("c"))
	if _, err := h(context.Background(), &Request{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(trace, ""); got != "abc" {
		t.Fatalf("order = %q, want abc", got)
	}
}

func TestRecover(t *testing.T) {
	h := Recover(slog.New(slog.DiscardHandler))(func(context.Context, *Request) (*Result, error) {
		panic("boom")
	})
	res, err := h(context.Background(), &Request{Tool: "t"})
	if err == nil || res != nil {
		t.Fatalf("got %v, %v; want error", res, err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Fatal("panic value must not leak to the client")
	}
}

func TestTimeout(t *testing.T) {
	h := Timeout(10 * time.Millisecond)(func(ctx context.Context, _ *Request) (*Result, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if _, err := h(context.Background(), &Request{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestRequireScopes(t *testing.T) {
	h := RequireScopes("a", "b")(textHandler("ok"))
	tests := []struct {
		name string
		id   *auth.Identity
		ok   bool
	}{
		{"no identity", nil, false},
		{"missing one", &auth.Identity{Scopes: []string{"a"}}, false},
		{"all", &auth.Identity{Scopes: []string{"b", "a", "c"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.id != nil {
				ctx = auth.NewContext(ctx, tt.id)
			}
			_, err := h(ctx, &Request{Tool: "t"})
			if (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestBuildToolOverridesDefaults(t *testing.T) {
	tool := &Tool{Name: "t", Handler: textHandler("12345"), MaxOutput: 3}
	h := Build(tool, Defaults{MaxOutput: 100}, nil)
	if _, err := h(context.Background(), &Request{Tool: "t"}); err == nil {
		t.Fatal("per-tool MaxOutput must override the default")
	}
}

func TestBuildAttachesMeta(t *testing.T) {
	ro := true
	tool := &Tool{
		Name:        "t",
		Annotations: Annotations{ReadOnly: &ro},
		Scopes:      []string{"s"},
		Handler:     textHandler("ok"),
	}
	var seen *ToolMeta
	spy := func(next Handler) Handler {
		return func(ctx context.Context, r *Request) (*Result, error) {
			seen = r.Meta
			return next(ctx, r)
		}
	}
	h := Build(tool, Defaults{Timeout: time.Minute, MaxOutput: 7}, []Middleware{spy})
	ctx := auth.NewContext(context.Background(), &auth.Identity{Scopes: []string{"s"}})
	req := &Request{Tool: "t"}
	if _, err := h(ctx, req); err != nil {
		t.Fatal(err)
	}
	if seen == nil || seen.Name != "t" || *seen.Annotations.ReadOnly != true ||
		seen.Timeout != time.Minute || seen.MaxOutput != 7 || seen.Scopes[0] != "s" {
		t.Fatalf("meta = %+v", seen)
	}
	if req.Meta != nil {
		t.Fatal("caller's request must not be mutated")
	}
}
