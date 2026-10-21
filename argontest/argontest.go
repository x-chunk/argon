// Package argontest provides helpers for unit-testing Argon servers without
// spawning a process: tools are called through an in-memory MCP session.
package argontest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/sdkadapter"
	"go.xchunk.org/argon/internal/testhook"
)

// CallOption configures a single Call.
type CallOption func(*callConfig)

type callConfig struct {
	ctx context.Context
	id  *auth.Identity
}

// WithContext sets the context for the call. Default: t.Context().
func WithContext(ctx context.Context) CallOption { return func(c *callConfig) { c.ctx = ctx } }

// WithIdentity runs the call as id, for testing Scopes and IdentityFrom.
func WithIdentity(id auth.Identity) CallOption { return func(c *callConfig) { c.id = &id } }

// Call invokes tool on s with input in and decodes the result into Out.
// A tool error (isError result) is returned as a non-nil error.
// Setup failures (bad server, encoding) fail the test immediately.
func Call[Out any](t testing.TB, s *argon.Server, tool string, in any, opts ...CallOption) (Out, error) {
	t.Helper()
	var zero Out
	cfg := callConfig{ctx: t.Context()}
	for _, o := range opts {
		o(&cfg)
	}
	// A fresh server per call keeps calls isolated (see CallInMemory).
	srv, err := testhook.BuildServer(s)
	if err != nil {
		t.Fatalf("argontest: build server: %v", err)
	}
	args, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("argontest: encode input: %v", err)
	}
	res, err := sdkadapter.CallInMemory(cfg.ctx, srv, tool, args, cfg.id)
	if err != nil {
		t.Fatalf("argontest: call %q: %v", tool, err)
	}
	text := joinText(res)
	if res.IsError {
		return zero, errors.New(text)
	}
	if p, ok := any(&zero).(*string); ok {
		*p = text
		return zero, nil
	}
	if err := json.Unmarshal([]byte(text), &zero); err != nil {
		return zero, fmt.Errorf("argontest: decode output: %w", err)
	}
	return zero, nil
}

func joinText(r *argon.Result) string {
	parts := make([]string, len(r.Content))
	for i, c := range r.Content {
		parts[i] = c.Text
	}
	return strings.Join(parts, "\n")
}
