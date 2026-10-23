package argon_test

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
)

// A complete server: one tool, served over stdio by default or over
// Streamable HTTP with --http :8080.
func Example() {
	type GreetInput struct {
		Name string `json:"name" jsonschema:"who to greet"`
	}
	s := argon.New("hello")
	argon.Tool(s, "greet", "Say hi", func(ctx context.Context, in GreetInput) (string, error) {
		return "Hi, " + in.Name + "!", nil
	}, argon.ReadOnly())
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}

// Tools with a struct output send structured content alongside its JSON
// text; per-tool options set annotations, scopes and limits.
func ExampleTool() {
	type SearchInput struct {
		Query string `json:"query" jsonschema:"full-text query"`
		Limit int    `json:"limit,omitempty" jsonschema:"maximum number of hits"`
	}
	type SearchOutput struct {
		Hits []string `json:"hits"`
	}
	s := argon.New("search")
	argon.Tool(s, "search", "Search the notes",
		func(ctx context.Context, in SearchInput) (SearchOutput, error) {
			return SearchOutput{Hits: []string{"note about " + in.Query}}, nil
		},
		argon.ReadOnly(),
		argon.Scopes("notes:read"),
		argon.ToolTimeout(5*time.Second),
	)
}

// A tool can return several content blocks.
func ExampleNewResult() {
	s := argon.New("charts")
	argon.Tool(s, "chart", "Draw a chart", func(ctx context.Context, _ struct{}) (*argon.Result, error) {
		png := []byte{0x89, 'P', 'N', 'G'} // rendered elsewhere
		return argon.NewResult(
			argon.TextContent{Text: "Sales by month"},
			argon.ImageContent{MIMEType: "image/png", Data: png},
			argon.ResourceLink{URI: "file:///sales.csv", Name: "sales.csv"},
		), nil
	})
}

func ExampleResource() {
	s := argon.New("docs")
	argon.Resource(s, "file:///readme.md", "readme", "Project readme",
		func(ctx context.Context) (argon.ResourceContents, error) {
			return argon.TextResource("# Project"), nil
		}, argon.MIMEType("text/markdown"))
}

// Template variables decode into a struct; a URI whose variables do not
// decode (here, a non-numeric id) reads as not found.
func ExampleResourceTemplate() {
	type NoteRef struct {
		ID int `json:"id"`
	}
	notes := map[int]string{1: "buy milk"}
	s := argon.New("notes")
	argon.ResourceTemplate(s, "notes://{id}", "note", "A note by id",
		func(ctx context.Context, ref NoteRef) (argon.ResourceContents, error) {
			n, ok := notes[ref.ID]
			if !ok {
				return argon.ResourceContents{}, argon.ErrResourceNotFound
			}
			return argon.TextResource(n), nil
		}, argon.MIMEType("text/plain"))
}

func ExamplePrompt() {
	type ReviewArgs struct {
		Code     string `json:"code" jsonschema:"code to review"`
		Language string `json:"language,omitempty" jsonschema:"programming language"`
	}
	s := argon.New("review")
	argon.Prompt(s, "code_review", "Review a piece of code",
		func(ctx context.Context, a ReviewArgs) ([]argon.PromptMessage, error) {
			return []argon.PromptMessage{
				argon.UserMessage("Review this " + a.Language + " code:\n" + a.Code),
			}, nil
		}, argon.PromptTitle("Code review"))
}

// Middleware sees every tool call with the tool's metadata.
func ExampleUse() {
	audit := func(next argon.Handler) argon.Handler {
		return func(ctx context.Context, req *argon.Request) (*argon.Result, error) {
			if d := req.Meta.Annotations.Destructive; d != nil && *d {
				log.Printf("destructive tool %s called", req.Tool)
			}
			return next(ctx, req)
		}
	}
	_ = argon.New("audited", argon.Use(audit))
}

// An HTTP server protected by an OAuth authorization server, mounted under a
// path prefix in an existing mux.
func ExampleServer_Handler() {
	ctx := context.Background()
	v, err := oidc.NewVerifier(ctx, "https://auth.example.com", "https://api.example.com/mcp")
	if err != nil {
		log.Fatal(err)
	}
	s := argon.New("notes", argon.Auth(auth.Config{
		Issuer:   "https://auth.example.com",
		Audience: "https://api.example.com/mcp",
		Verifier: v,
	}))
	argon.Tool(s, "whoami", "Show the caller", func(ctx context.Context, _ struct{}) (string, error) {
		return argon.IdentityFrom(ctx).Subject, nil
	})

	h, err := s.Handler()
	if err != nil {
		log.Fatal(err)
	}
	path, meta, err := s.ResourceMetadata()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.Handle(path, meta)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
