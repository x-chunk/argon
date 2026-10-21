package auth

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestDefaultClaimsMapper(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
		want   []string
	}{
		{"scope string", map[string]any{"scope": "a b  c"}, []string{"a", "b", "c"}},
		{"scp string", map[string]any{"scp": "a"}, []string{"a"}},
		{"scp array", map[string]any{"scp": []any{"a", 1, "b"}}, []string{"a", "b"}},
		{"both", map[string]any{"scope": "a", "scp": []any{"b"}}, []string{"a", "b"}},
		{"none", map[string]any{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.claims["sub"] = "u1"
			id := DefaultClaimsMapper(tt.claims)
			if id.Subject != "u1" || !slices.Equal(id.Scopes, tt.want) {
				t.Fatalf("got %q %v, want u1 %v", id.Subject, id.Scopes, tt.want)
			}
		})
	}
}

func TestStatic(t *testing.T) {
	v := Static("secret", Identity{Subject: "admin", Scopes: []string{"x"}})
	ctx := context.Background()
	id, err := v.Verify(ctx, "secret")
	if err != nil || id.Subject != "admin" || !id.HasScope("x") {
		t.Fatalf("valid token: %+v, %v", id, err)
	}
	for _, bad := range []string{"", "secre", "secret2"} {
		if _, err := v.Verify(ctx, bad); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("token %q: err = %v, want ErrInvalidToken", bad, err)
		}
	}
	if _, err := Static("", Identity{}).Verify(ctx, ""); err == nil {
		t.Error("empty configured token must reject everything")
	}
}

func TestConfigValidate(t *testing.T) {
	v := Static("t", Identity{})
	for _, tt := range []struct {
		cfg Config
		ok  bool
	}{
		{Config{Audience: "https://mcp", Verifier: v}, true},
		{Config{Verifier: v}, false},
		{Config{Audience: "https://mcp"}, false},
	} {
		if err := tt.cfg.Validate(); (err == nil) != tt.ok {
			t.Errorf("%+v: err = %v, want ok=%v", tt.cfg, err, tt.ok)
		}
	}
}

func TestContext(t *testing.T) {
	ctx := context.Background()
	if FromContext(ctx) != nil {
		t.Fatal("empty context must have no identity")
	}
	id := &Identity{Subject: "s"}
	if FromContext(NewContext(ctx, id)) != id {
		t.Fatal("identity not round-tripped")
	}
	var nilID *Identity
	if nilID.HasScope("x") {
		t.Fatal("nil identity has no scopes")
	}
}
