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
	res := CallResult(t, s, tool, in, opts...)
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

// CallResult invokes tool and returns the raw result, for tools that return
// several content blocks (text, images, resource links). Tool errors are
// reported via Result.IsError; setup failures fail the test.
func CallResult(t testing.TB, s *argon.Server, tool string, in any, opts ...CallOption) *argon.Result {
	t.Helper()
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
	out := &argon.Result{Structured: res.Structured, IsError: res.IsError}
	for _, b := range res.Content {
		out.Content = append(out.Content, fromBlock(b))
	}
	return out
}

// joinText concatenates the text blocks of r, ignoring other content.
func joinText(r *argon.Result) string {
	var parts []string
	for _, c := range r.Content {
		if t, ok := c.(argon.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ReadResource reads uri from s through an in-memory MCP session. A missing
// resource is reported as an error wrapping argon.ErrResourceNotFound; other
// read failures are returned as errors. Setup failures fail the test.
// WithContext and WithIdentity apply as for Call.
func ReadResource(t testing.TB, s *argon.Server, uri string, opts ...CallOption) (argon.ResourceContents, error) {
	t.Helper()
	cfg := callConfig{ctx: t.Context()}
	for _, o := range opts {
		o(&cfg)
	}
	srv, err := testhook.BuildServer(s)
	if err != nil {
		t.Fatalf("argontest: build server: %v", err)
	}
	c, err := sdkadapter.ReadInMemory(cfg.ctx, srv, uri, cfg.id)
	if err != nil {
		return argon.ResourceContents{}, err
	}
	return argon.ResourceContents{URI: c.URI, MIMEType: c.MIMEType, Text: c.Text, Blob: c.Blob}, nil
}

// GetPrompt renders prompt name from s through an in-memory MCP session.
// Arguments are passed as strings, as a client would send them. Rejected
// arguments are reported as an error wrapping argon.ErrInvalidArguments;
// setup failures fail the test. WithContext and WithIdentity apply as for
// Call.
func GetPrompt(t testing.TB, s *argon.Server, name string, args map[string]string, opts ...CallOption) ([]argon.PromptMessage, error) {
	t.Helper()
	cfg := callConfig{ctx: t.Context()}
	for _, o := range opts {
		o(&cfg)
	}
	srv, err := testhook.BuildServer(s)
	if err != nil {
		t.Fatalf("argontest: build server: %v", err)
	}
	msgs, err := sdkadapter.GetPromptInMemory(cfg.ctx, srv, name, args, cfg.id)
	if err != nil {
		return nil, err
	}
	out := make([]argon.PromptMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, argon.PromptMessage{Role: argon.Role(m.Role), Content: fromBlock(m.Content)})
	}
	return out, nil
}

// fromBlock converts an adapter content block to Argon's content types.
func fromBlock(b sdkadapter.Block) argon.Content {
	switch b.Kind {
	case sdkadapter.BlockImage:
		return argon.ImageContent{MIMEType: b.MIMEType, Data: b.Data}
	case sdkadapter.BlockResourceLink:
		return argon.ResourceLink{URI: b.URI, Name: b.Name, Title: b.Title, Description: b.Description, MIMEType: b.MIMEType}
	default:
		return argon.TextContent{Text: b.Text}
	}
}
