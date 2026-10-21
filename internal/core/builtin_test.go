package core

import (
	"context"
	"strings"
	"testing"
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
