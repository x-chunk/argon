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

func TestBuildResourceDefaults(t *testing.T) {
	r := &Resource{URI: "file:///a", Name: "a", MIMEType: "text/markdown", Handler: readText("# hi")}
	c, err := BuildResource(r, Defaults{})(context.Background(), &ResourceRequest{URI: "file:///a"})
	if err != nil {
		t.Fatal(err)
	}
	if c.URI != "file:///a" || c.MIMEType != "text/markdown" || c.Text != "# hi" {
		t.Fatalf("contents = %+v", c)
	}
}

func TestBuildResourcePolicies(t *testing.T) {
	quiet := Defaults{Logger: slog.New(slog.DiscardHandler)}
	req := &ResourceRequest{URI: "file:///a"}
	ctx := context.Background()

	panicky := &Resource{URI: "file:///a", Name: "a", Handler: func(context.Context, *ResourceRequest) (*ResourceContents, error) {
		panic("secret detail")
	}}
	if _, err := BuildResource(panicky, quiet)(ctx, req); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("panic: err = %v", err)
	}

	slow := &Resource{URI: "file:///a", Name: "a", Timeout: 10 * time.Millisecond,
		Handler: func(ctx context.Context, _ *ResourceRequest) (*ResourceContents, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	if _, err := BuildResource(slow, quiet)(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: err = %v", err)
	}

	big := &Resource{URI: "file:///a", Name: "a", Handler: readText("12345")}
	if _, err := BuildResource(big, Defaults{MaxOutput: 4})(ctx, req); err == nil {
		t.Fatal("size: want error")
	}

	scoped := &Resource{URI: "file:///a", Name: "a", Scopes: []string{"files:read"}, Handler: readText("x")}
	if _, err := BuildResource(scoped, quiet)(ctx, req); err == nil {
		t.Fatal("scopes: want error without identity")
	}
	authed := auth.NewContext(ctx, &auth.Identity{Scopes: []string{"files:read"}})
	if _, err := BuildResource(scoped, quiet)(authed, req); err != nil {
		t.Fatalf("scopes: %v", err)
	}

	notFound := &Resource{URITemplate: "notes://{id}", Name: "n", Handler: func(context.Context, *ResourceRequest) (*ResourceContents, error) {
		return nil, ErrResourceNotFound
	}}
	if _, err := BuildResource(notFound, quiet)(ctx, &ResourceRequest{URI: "notes://1"}); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("not found must pass through: err = %v", err)
	}
}
