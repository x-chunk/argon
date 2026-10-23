package oidc_test

import (
	"context"
	"log"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
)

// Verify tokens issued by an OIDC provider for this server's audience, and
// introspect opaque tokens with the server's own client credentials.
func ExampleNewVerifier() {
	ctx := context.Background()
	v, err := oidc.NewVerifier(ctx, "https://auth.example.com", "https://mcp.example.com",
		oidc.Introspection("mcp-server", "client-secret"))
	if err != nil {
		log.Fatal(err)
	}
	_ = argon.New("notes", argon.Auth(auth.Config{
		Issuer:   "https://auth.example.com",
		Audience: "https://mcp.example.com",
		Verifier: v,
	}))
}
