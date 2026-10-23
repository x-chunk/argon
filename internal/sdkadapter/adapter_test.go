package sdkadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
)

func ptr[T any](v T) *T { return &v }

func text(s string) *ToolResult {
	return &ToolResult{Content: []Block{{Kind: BlockText, Text: s}}}
}

func whoami(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	id := auth.FromContext(ctx)
	if id == nil {
		return text("anonymous"), nil
	}
	return text(id.Subject), nil
}

// requireScope stands in for Argon's scope check, which lives above the
// adapter; here it only proves the identity reaches the handler.
func requireScope(scope string, next func(context.Context, json.RawMessage) (*ToolResult, error)) func(context.Context, json.RawMessage) (*ToolResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
		if !auth.FromContext(ctx).HasScope(scope) {
			return nil, errors.New("insufficient scope")
		}
		return next(ctx, args)
	}
}

func testServer() *Server {
	obj := json.RawMessage(`{"type":"object"}`)
	who := ToolDef{Name: "whoami", InputSchema: obj,
		Annotations: Annotations{ReadOnly: ptr(true), OpenWorld: ptr(false)}}
	adm := ToolDef{Name: "admin", InputSchema: obj}
	return NewServer(Config{Name: "t", Version: "0", Tools: []Tool{
		{Def: who, Call: whoami},
		{Def: adm, Call: requireScope("admin", whoami)},
	}})
}

func TestToSDKToolAnnotations(t *testing.T) {
	got := toSDKTool(ToolDef{Name: "x", Annotations: Annotations{
		ReadOnly: ptr(true), Destructive: ptr(false), Idempotent: ptr(true),
	}})
	a := got.Annotations
	if a == nil || !a.ReadOnlyHint || !a.IdempotentHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint != nil {
		t.Fatalf("annotations = %+v", a)
	}
	if toSDKTool(ToolDef{Name: "y"}).Annotations != nil {
		t.Fatal("unspecified annotations must be omitted")
	}
}

type bearer struct {
	token string
	base  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func newHTTP(t *testing.T) *httptest.Server {
	t.Helper()
	ac := &auth.Config{
		Issuer:   "https://issuer.example",
		Audience: "https://mcp.example",
		Verifier: auth.VerifierFunc(func(_ context.Context, tok string) (*auth.Identity, error) {
			switch tok {
			case "user":
				return &auth.Identity{Subject: "alice"}, nil
			case "admin":
				return &auth.Identity{Subject: "root", Scopes: []string{"admin"}}, nil
			}
			return nil, auth.ErrInvalidToken
		}),
	}
	h, err := HTTPHandler(testServer(), ac)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	cs, err := c.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: &http.Client{Transport: bearer{token, http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func callText(t *testing.T, cs *mcp.ClientSession, tool string) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call %s: %v", tool, err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

func TestHTTPRejectsMissingToken(t *testing.T) {
	ts := newHTTP(t)
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if h := resp.Header.Get("WWW-Authenticate"); !strings.Contains(h, "https://mcp.example/.well-known/oauth-protected-resource") {
		t.Fatalf("WWW-Authenticate = %q", h)
	}
}

func TestHTTPResourceMetadata(t *testing.T) {
	ts := newHTTP(t)
	resp, err := http.Get(ts.URL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if meta.Resource != "https://mcp.example" || len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != "https://issuer.example" {
		t.Fatalf("metadata = %+v", meta)
	}
}

func TestHTTPIdentityAndScopes(t *testing.T) {
	ts := newHTTP(t)

	user := connect(t, ts.URL, "user")
	if got, isErr := callText(t, user, "whoami"); isErr || got != "alice" {
		t.Fatalf("whoami = %q (isError=%v), want alice", got, isErr)
	}
	if _, isErr := callText(t, user, "admin"); !isErr {
		t.Fatal("user without admin scope must get an error result")
	}

	admin := connect(t, ts.URL, "admin")
	if got, isErr := callText(t, admin, "admin"); isErr || got != "root" {
		t.Fatalf("admin = %q (isError=%v), want root", got, isErr)
	}
}

func TestContentRoundTrip(t *testing.T) {
	blocks := []Block{
		{Kind: BlockText, Text: "hi"},
		{Kind: BlockImage, MIMEType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}},
		{Kind: BlockResourceLink, URI: "file:///a.txt", Name: "a.txt", MIMEType: "text/plain"},
	}
	for _, b := range blocks {
		got, ok := fromSDKContent(toSDKContent(b))
		if !ok || !reflect.DeepEqual(got, b) {
			t.Errorf("round trip %#v → %#v", b, got)
		}
	}
}

func TestMetadataPath(t *testing.T) {
	for in, want := range map[string]string{
		"":          "/.well-known/oauth-protected-resource",
		"/":         "/.well-known/oauth-protected-resource",
		"/mcp":      "/.well-known/oauth-protected-resource/mcp",
		"/api/mcp/": "/.well-known/oauth-protected-resource/api/mcp",
	} {
		if got := metadataPath(in); got != want {
			t.Errorf("metadataPath(%q) = %q, want %q", in, got, want)
		}
	}
}
