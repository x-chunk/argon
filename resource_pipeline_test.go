package argon

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
	r := &resourceDef{URI: "file:///a", Name: "a", MIMEType: "text/markdown", Handler: readText("# hi")}
	c, err := buildResource(r, policy{})(context.Background(), &resourceRequest{URI: "file:///a"})
	if err != nil {
		t.Fatal(err)
	}
	if c.URI != "file:///a" || c.MIMEType != "text/markdown" || c.Text != "# hi" {
		t.Fatalf("contents = %+v", c)
	}
}

func TestBuildResourcePolicies(t *testing.T) {
	quiet := policy{Logger: slog.New(slog.DiscardHandler)}
	req := &resourceRequest{URI: "file:///a"}
	ctx := context.Background()

	panicky := &resourceDef{URI: "file:///a", Name: "a", Handler: func(context.Context, *resourceRequest) (*ResourceContents, error) {
		panic("secret detail")
	}}
	if _, err := buildResource(panicky, quiet)(ctx, req); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("panic: err = %v", err)
	}

	slow := &resourceDef{URI: "file:///a", Name: "a", Timeout: 10 * time.Millisecond,
		Handler: func(ctx context.Context, _ *resourceRequest) (*ResourceContents, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	if _, err := buildResource(slow, quiet)(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: err = %v", err)
	}

	big := &resourceDef{URI: "file:///a", Name: "a", Handler: readText("12345")}
	if _, err := buildResource(big, policy{MaxOutput: 4})(ctx, req); err == nil {
		t.Fatal("size: want error")
	}

	scoped := &resourceDef{URI: "file:///a", Name: "a", Scopes: []string{"files:read"}, Handler: readText("x")}
	if _, err := buildResource(scoped, quiet)(ctx, req); err == nil {
		t.Fatal("scopes: want error without identity")
	}
	authed := auth.NewContext(ctx, &auth.Identity{Scopes: []string{"files:read"}})
	if _, err := buildResource(scoped, quiet)(authed, req); err != nil {
		t.Fatalf("scopes: %v", err)
	}

	notFound := &resourceDef{URITemplate: "notes://{id}", Name: "n", Handler: func(context.Context, *resourceRequest) (*ResourceContents, error) {
		return nil, ErrResourceNotFound
	}}
	if _, err := buildResource(notFound, quiet)(ctx, &resourceRequest{URI: "notes://1"}); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("not found must pass through: err = %v", err)
	}
}
