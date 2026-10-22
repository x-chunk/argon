// Command auth is an HTTP MCP server protected by an OAuth authorization
// server. Defaults match the Keycloak realm in test/keycloak:
//
//	docker compose -f test/keycloak/compose.yaml up -d --wait
//	go run ./examples/auth
//
// Override with ARGON_ISSUER, ARGON_AUDIENCE and ARGON_HTTP.
package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
)

type note struct {
	Text string `json:"text" jsonschema:"note text"`
}

// noteRef addresses a note in the notes://{id} resource template.
type noteRef struct {
	ID int `json:"id"`
}

type notes struct {
	mu   sync.Mutex
	list []string
}

func main() {
	issuer := cmp.Or(os.Getenv("ARGON_ISSUER"), "http://localhost:8180/realms/argon")
	audience := cmp.Or(os.Getenv("ARGON_AUDIENCE"), "http://localhost:9090")
	addr := cmp.Or(os.Getenv("ARGON_HTTP"), ":9090")

	ctx := context.Background()
	v, err := oidc.NewVerifier(ctx, issuer, audience)
	if err != nil {
		log.Fatal(err)
	}
	s := argon.New("notes", argon.LogCalls(), argon.Auth(auth.Config{
		Issuer:   issuer,
		Audience: audience,
		Verifier: v,
	}))

	var n notes
	argon.Tool(s, "whoami", "Show the authenticated caller", func(ctx context.Context, _ struct{}) (string, error) {
		id := argon.IdentityFrom(ctx)
		return id.Subject + " [" + strings.Join(id.Scopes, " ") + "]", nil
	}, argon.ReadOnly())

	argon.Tool(s, "list_notes", "List saved notes", func(context.Context, struct{}) ([]string, error) {
		n.mu.Lock()
		defer n.mu.Unlock()
		return slices.Clone(n.list), nil
	}, argon.ReadOnly(), argon.Scopes("notes:read"))

	argon.Tool(s, "add_note", "Save a note", func(_ context.Context, in note) (string, error) {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.list = append(n.list, in.Text)
		return fmt.Sprintf("saved as notes://%d", len(n.list)), nil
	}, argon.Scopes("notes:write"))

	// Each note is also readable as a resource, notes://1, notes://2, ...
	argon.ResourceTemplate(s, "notes://{id}", "note", "A saved note by number",
		func(_ context.Context, ref noteRef) (argon.ResourceContents, error) {
			n.mu.Lock()
			defer n.mu.Unlock()
			if ref.ID < 1 || ref.ID > len(n.list) {
				return argon.ResourceContents{}, argon.ErrResourceNotFound
			}
			return argon.TextResource(n.list[ref.ID-1]), nil
		}, argon.MIMEType("text/plain"), argon.ResourceScopes("notes:read"))

	if err := s.RunHTTP(ctx, addr); err != nil {
		log.Fatal(err)
	}
}
