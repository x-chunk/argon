package sdkadapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

func ptr[T any](v T) *T { return &v }

func whoami(ctx context.Context, _ *core.Request) (*core.Result, error) {
	id := auth.FromContext(ctx)
	if id == nil {
		return &core.Result{Content: []core.Content{core.TextContent{Text: "anonymous"}}}, nil
	}
	return &core.Result{Content: []core.Content{core.TextContent{Text: id.Subject}}}, nil
}

func testServer() *Server {
	obj := json.RawMessage(`{"type":"object"}`)
	who := &core.Tool{Name: "whoami", InputSchema: obj, Handler: whoami,
		Annotations: core.Annotations{ReadOnly: ptr(true), OpenWorld: ptr(false)}}
	adm := &core.Tool{Name: "admin", InputSchema: obj, Handler: whoami, Scopes: []string{"admin"}}
	return NewServer(Config{Name: "t", Version: "0", Tools: []Tool{
		{Def: who, Handler: core.Build(who, core.Defaults{}, nil)},
		{Def: adm, Handler: core.Build(adm, core.Defaults{}, nil)},
	}})
}

func TestToSDKToolAnnotations(t *testing.T) {
	got := toSDKTool(&core.Tool{Name: "x", Annotations: core.Annotations{
		ReadOnly: ptr(true), Destructive: ptr(false), Idempotent: ptr(true),
	}})
	a := got.Annotations
	if a == nil || !a.ReadOnlyHint || !a.IdempotentHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint != nil {
		t.Fatalf("annotations = %+v", a)
	}
	if toSDKTool(&core.Tool{Name: "y"}).Annotations != nil {
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
	blocks := []core.Content{
		core.TextContent{Text: "hi"},
		core.ImageContent{MIMEType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}},
		core.ResourceLink{URI: "file:///a.txt", Name: "a.txt", MIMEType: "text/plain"},
	}
	for _, b := range blocks {
		got := fromSDKContent(toSDKContent(b))
		if !reflect.DeepEqual(got, b) {
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
