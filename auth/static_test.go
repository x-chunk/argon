package auth

import (
	"context"
	"testing"
)

func TestStaticReturnsCopy(t *testing.T) {
	v := Static("t", Identity{Scopes: []string{"read"}, Claims: map[string]any{"k": "v"}})
	ctx := context.Background()
	id, _ := v.Verify(ctx, "t")
	id.Scopes[0] = "admin"
	id.Claims["k"] = "x"
	id2, _ := v.Verify(ctx, "t")
	if id2.Scopes[0] != "read" || id2.Claims["k"] != "v" {
		t.Fatalf("configured identity was mutated: %+v", id2)
	}
}
