package sdkadapter

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/errs"
)

func resourceServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(Config{Name: "t", Version: "0", Resources: []Resource{
		{Def: ResourceDef{URI: "file:///readme.md", Name: "readme", MIMEType: "text/markdown"},
			Read: func(_ context.Context, uri string) (*ResourceData, error) {
				return &ResourceData{URI: uri, MIMEType: "text/markdown", Text: "# Argon"}, nil
			}},
		{Def: ResourceDef{URITemplate: "notes://{id}", Name: "note"},
			Read: func(_ context.Context, uri string) (*ResourceData, error) {
				id := strings.TrimPrefix(uri, "notes://")
				if id == "missing" {
					return nil, errs.ResourceNotFound
				}
				return &ResourceData{URI: uri, Text: "note " + id}, nil
			}},
		// Argon checks scopes above the adapter; this stands in for it.
		{Def: ResourceDef{URI: "secret://key", Name: "key"},
			Read: func(ctx context.Context, uri string) (*ResourceData, error) {
				if !auth.FromContext(ctx).HasScope("admin") {
					return nil, errors.New("insufficient scope")
				}
				return &ResourceData{URI: uri, Blob: []byte{1, 2, 3}, MIMEType: "application/octet-stream"}, nil
			}},
	}})
}

func TestResourceList(t *testing.T) {
	cs, closeAll, err := session(t.Context(), resourceServer(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	res, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) != 2 || res.Resources[0].URI != "file:///readme.md" || res.Resources[0].MIMEType != "text/markdown" {
		t.Fatalf("resources = %+v", res.Resources)
	}
	tmpls, err := cs.ListResourceTemplates(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpls.ResourceTemplates) != 1 || tmpls.ResourceTemplates[0].URITemplate != "notes://{id}" {
		t.Fatalf("templates = %+v", tmpls.ResourceTemplates)
	}
}

func TestResourceRead(t *testing.T) {
	ctx := t.Context()
	c, err := ReadInMemory(ctx, resourceServer(t), "file:///readme.md", nil)
	if err != nil || c.Text != "# Argon" || c.MIMEType != "text/markdown" || c.URI != "file:///readme.md" {
		t.Fatalf("readme = %+v, %v", c, err)
	}
	c, err = ReadInMemory(ctx, resourceServer(t), "notes://42", nil)
	if err != nil || c.Text != "note 42" || c.URI != "notes://42" {
		t.Fatalf("note = %+v, %v", c, err)
	}
	for _, uri := range []string{"notes://missing", "nothing://here"} {
		if _, err := ReadInMemory(ctx, resourceServer(t), uri, nil); !errors.Is(err, errs.ResourceNotFound) {
			t.Errorf("%s: err = %v, want ErrResourceNotFound", uri, err)
		}
	}
}

func TestResourceScopes(t *testing.T) {
	ctx := t.Context()
	if _, err := ReadInMemory(ctx, resourceServer(t), "secret://key", nil); err == nil {
		t.Fatal("unauthenticated read of scoped resource must fail")
	}
	c, err := ReadInMemory(ctx, resourceServer(t), "secret://key", &auth.Identity{Scopes: []string{"admin"}})
	if err != nil || len(c.Blob) != 3 || c.MIMEType != "application/octet-stream" {
		t.Fatalf("blob = %+v, %v", c, err)
	}
}

func TestResourceOverHTTPWithAuth(t *testing.T) {
	ac := &auth.Config{
		Issuer:   "https://issuer.example",
		Audience: "https://mcp.example",
		Verifier: auth.Static("admin-token", auth.Identity{Subject: "root", Scopes: []string{"admin"}}),
	}
	h, err := HTTPHandler(resourceServer(t), ac)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	cs := connect(t, ts.URL, "admin-token")
	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "secret://key"})
	if err != nil {
		t.Fatalf("scoped read with valid token: %v", err)
	}
	if len(res.Contents) != 1 || len(res.Contents[0].Blob) != 3 {
		t.Fatalf("contents = %+v", res.Contents)
	}
}

func TestResourceCacheScope(t *testing.T) {
	read := func(id *auth.Identity) string {
		cs, closeAll, err := session(t.Context(), resourceServer(t), id)
		if err != nil {
			t.Fatal(err)
		}
		defer closeAll()
		res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "file:///readme.md"})
		if err != nil {
			t.Fatal(err)
		}
		return res.CacheScope
	}
	if got := read(nil); got != "public" {
		t.Errorf("anonymous read: cacheScope = %q, want public", got)
	}
	if got := read(&auth.Identity{Subject: "u"}); got != "private" {
		t.Errorf("authenticated read: cacheScope = %q, want private", got)
	}
}
