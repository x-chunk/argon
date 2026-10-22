package sdkadapter

import (
	"context"
	"errors"
	"testing"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

func resourceServer(t *testing.T) *Server {
	t.Helper()
	reg := core.NewRegistry()
	defs := []*core.Resource{
		{URI: "file:///readme.md", Name: "readme", MIMEType: "text/markdown",
			Handler: func(context.Context, *core.ResourceRequest) (*core.ResourceContents, error) {
				return &core.ResourceContents{Text: "# Argon"}, nil
			}},
		{URITemplate: "notes://{id}", Name: "note",
			Handler: func(_ context.Context, r *core.ResourceRequest) (*core.ResourceContents, error) {
				if r.Params["id"] == "missing" {
					return nil, core.ErrResourceNotFound
				}
				return &core.ResourceContents{Text: "note " + r.Params["id"]}, nil
			}},
		{URI: "secret://key", Name: "key", Scopes: []string{"admin"},
			Handler: func(ctx context.Context, _ *core.ResourceRequest) (*core.ResourceContents, error) {
				return &core.ResourceContents{Blob: []byte{1, 2, 3}, MIMEType: "application/octet-stream"}, nil
			}},
	}
	var rs []Resource
	for _, d := range defs {
		if err := reg.AddResource(d); err != nil {
			t.Fatal(err)
		}
		rs = append(rs, Resource{Def: d, Handler: core.BuildResource(d, core.Defaults{})})
	}
	return NewServer(Config{Name: "t", Version: "0", Resources: rs})
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
		if _, err := ReadInMemory(ctx, resourceServer(t), uri, nil); !errors.Is(err, core.ErrResourceNotFound) {
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
