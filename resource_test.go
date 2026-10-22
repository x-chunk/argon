package argon_test

import (
	"context"
	"errors"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
)

type noteRef struct {
	ID int `json:"id"`
}

func notesServer() *argon.Server {
	notes := map[int]string{1: "buy milk", 2: "call mom"}
	s := argon.New("notes")
	argon.Resource(s, "file:///readme.md", "readme", "Project readme",
		func(context.Context) (argon.ResourceContents, error) {
			return argon.TextResource("# Notes"), nil
		}, argon.MIMEType("text/markdown"))
	argon.ResourceTemplate(s, "notes://{id}", "note", "A note by id",
		func(_ context.Context, ref noteRef) (argon.ResourceContents, error) {
			n, ok := notes[ref.ID]
			if !ok {
				return argon.ResourceContents{}, argon.ErrResourceNotFound
			}
			return argon.TextResource(n), nil
		}, argon.MIMEType("text/plain"))
	argon.Resource(s, "secret://logo.png", "logo", "Company logo",
		func(context.Context) (argon.ResourceContents, error) {
			return argon.BlobResource("image/png", []byte{0x89, 'P', 'N', 'G'}), nil
		}, argon.ResourceScopes("brand:read"))
	return s
}

func TestReadFixedResource(t *testing.T) {
	c, err := argontest.ReadResource(t, notesServer(), "file:///readme.md")
	if err != nil {
		t.Fatal(err)
	}
	if c.Text != "# Notes" || c.MIMEType != "text/markdown" || c.URI != "file:///readme.md" {
		t.Fatalf("contents = %+v", c)
	}
}

func TestReadTemplateResource(t *testing.T) {
	s := notesServer()
	c, err := argontest.ReadResource(t, s, "notes://2")
	if err != nil || c.Text != "call mom" || c.URI != "notes://2" || c.MIMEType != "text/plain" {
		t.Fatalf("note 2 = %+v, %v", c, err)
	}
	for _, uri := range []string{
		"notes://3",   // handler says not found
		"notes://abc", // id is not an int
		"other://1",   // matches nothing
	} {
		if _, err := argontest.ReadResource(t, s, uri); !errors.Is(err, argon.ErrResourceNotFound) {
			t.Errorf("%s: err = %v, want ErrResourceNotFound", uri, err)
		}
	}
}

func TestReadScopedBlob(t *testing.T) {
	s := notesServer()
	if _, err := argontest.ReadResource(t, s, "secret://logo.png"); err == nil {
		t.Fatal("scoped resource without identity must fail")
	}
	c, err := argontest.ReadResource(t, s, "secret://logo.png",
		argontest.WithIdentity(argon.Identity{Subject: "u", Scopes: []string{"brand:read"}}))
	if err != nil || c.MIMEType != "image/png" || string(c.Blob) != "\x89PNG" {
		t.Fatalf("logo = %+v, %v", c, err)
	}
}

func TestResourcesAndToolsTogether(t *testing.T) {
	s := notesServer()
	argon.Tool(s, "count", "Count notes", func(context.Context, struct{}) (int, error) { return 2, nil })
	if got, err := argontest.Call[int](t, s, "count", struct{}{}); err != nil || got != 2 {
		t.Fatalf("tool = %v, %v", got, err)
	}
	if _, err := argontest.ReadResource(t, s, "notes://1"); err != nil {
		t.Fatalf("resource: %v", err)
	}
}
