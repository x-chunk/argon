package argon

import (
	"context"
	"testing"
)

type noteRef struct {
	Folder string `json:"folder"`
	ID     int    `json:"id"`
	Draft  bool   `json:"draft,omitempty"`
}

func TestDecodeParams(t *testing.T) {
	ps, err := paramsSchema[noteRef]()
	if err != nil {
		t.Fatal(err)
	}
	var got noteRef
	if err := ps.decodeParams(map[string]string{"folder": "work", "id": "42", "draft": "true"}, &got); err != nil {
		t.Fatal(err)
	}
	if got != (noteRef{Folder: "work", ID: 42, Draft: true}) {
		t.Fatalf("got %+v", got)
	}
	for name, params := range map[string]map[string]string{
		"not an int":       {"folder": "w", "id": "abc"},
		"not a bool":       {"folder": "w", "id": "1", "draft": "maybe"},
		"missing required": {"folder": "w"},
	} {
		if err := ps.decodeParams(params, new(noteRef)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestResourceRegistrationErrors(t *testing.T) {
	read := func(context.Context) (ResourceContents, error) { return TextResource("x"), nil }
	readNote := func(context.Context, noteRef) (ResourceContents, error) { return TextResource("x"), nil }

	s := New("t")
	Resource(s, "relative", "r", "", read)
	if s.Err() == nil {
		t.Error("relative URI must be rejected")
	}

	s = New("t")
	ResourceTemplate(s, "notes://{folder}/{id}/{extra}", "n", "", readNote)
	if s.Err() == nil {
		t.Error("template variable without a field must be rejected")
	}

	s = New("t")
	ResourceTemplate(s, "kv://{k}", "kv", "", func(context.Context, string) (ResourceContents, error) {
		return ResourceContents{}, nil
	})
	if s.Err() == nil {
		t.Error("non-struct parameter type must be rejected")
	}

	s = New("t")
	Resource(s, "file:///a", "a", "", read)
	ResourceTemplate(s, "notes://{folder}/{id}", "n", "", readNote)
	if err := s.Err(); err != nil {
		t.Fatalf("valid registrations: %v", err)
	}
	if _, err := s.build(); err != nil {
		t.Fatalf("resource-only server must build: %v", err)
	}
}

func TestEmptyServerFails(t *testing.T) {
	if _, err := New("t").build(); err == nil {
		t.Fatal("server with nothing registered must fail to build")
	}
}
