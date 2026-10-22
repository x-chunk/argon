package core

import (
	"context"
	"maps"
	"testing"
)

func readText(s string) ResourceHandler {
	return func(context.Context, *ResourceRequest) (*ResourceContents, error) {
		return &ResourceContents{Text: s}, nil
	}
}

func TestAddResourceValidation(t *testing.T) {
	r := NewRegistry()
	ok := []*Resource{
		{URI: "file:///readme.md", Name: "readme", Handler: readText("x")},
		{URITemplate: "notes://{id}", Name: "note", Handler: readText("x")},
	}
	for _, res := range ok {
		if err := r.AddResource(res); err != nil {
			t.Fatalf("%s: %v", res.Key(), err)
		}
	}
	bad := map[string]*Resource{
		"duplicate":         {URI: "file:///readme.md", Name: "dup", Handler: readText("")},
		"relative uri":      {URI: "readme.md", Name: "r", Handler: readText("")},
		"no name":           {URI: "file:///a", Handler: readText("")},
		"no handler":        {URI: "file:///b", Name: "b"},
		"both":              {URI: "file:///c", URITemplate: "file:///{c}", Name: "c", Handler: readText("")},
		"neither":           {Name: "d", Handler: readText("")},
		"bad template":      {URITemplate: "notes://{id", Name: "e", Handler: readText("")},
		"relative template": {URITemplate: "{id}", Name: "f", Handler: readText("")},
	}
	for name, res := range bad {
		if err := r.AddResource(res); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if got := len(r.Resources()); got != 2 {
		t.Fatalf("resources = %d, want 2", got)
	}
}

func TestResourceMatch(t *testing.T) {
	res := &Resource{URITemplate: "notes://{folder}/{id}", Name: "n", Handler: readText("")}
	if err := res.validate(); err != nil {
		t.Fatal(err)
	}
	params, ok := res.Match("notes://work/42")
	if !ok || !maps.Equal(params, map[string]string{"folder": "work", "id": "42"}) {
		t.Fatalf("match = %v, %v", params, ok)
	}
	if _, ok := res.Match("other://work/42"); ok {
		t.Fatal("foreign scheme must not match")
	}
	fixed := &Resource{URI: "file:///a", Name: "a", Handler: readText("")}
	if _, ok := fixed.Match("file:///a"); !ok {
		t.Fatal("fixed URI must match itself")
	}
}

func TestHasScheme(t *testing.T) {
	for s, want := range map[string]bool{
		"notes://{id}":   true,
		"file:///{path}": true,
		"x-app+v1.2:{a}": true,
		"{scheme}://x":   false,
		"1abc://x":       false,
		"no-colon":       false,
		":empty":         false,
	} {
		if got := hasScheme(s); got != want {
			t.Errorf("hasScheme(%q) = %v, want %v", s, got, want)
		}
	}
}
