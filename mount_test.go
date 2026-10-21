package argon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/auth"
)

func TestMountUnderPrefix(t *testing.T) {
	const audience = "https://api.example/mcp"
	s := argon.New("prefixed", argon.Auth(auth.Config{
		Issuer:   "https://issuer.example",
		Audience: audience,
		Verifier: auth.Static("secret", auth.Identity{Subject: "u"}),
	}))
	argon.Tool(s, "ping", "Ping", func(context.Context, struct{}) (string, error) { return "pong", nil })

	h, err := s.Handler()
	if err != nil {
		t.Fatal(err)
	}
	path, meta, err := s.ResourceMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if path != "/.well-known/oauth-protected-resource/mcp" {
		t.Fatalf("metadata path = %q", path)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.Handle(path, meta)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Unauthenticated: 401 pointing at the RFC 9728 location for /mcp.
	resp, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	want := "https://api.example/.well-known/oauth-protected-resource/mcp"
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(resp.Header.Get("WWW-Authenticate"), want) {
		t.Fatalf("status %d, WWW-Authenticate %q; want 401 with %s", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), want)
	}

	// The advertised metadata is reachable and names the prefixed resource.
	resp, err = http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	var md struct {
		Resource string `json:"resource"`
	}
	json.NewDecoder(resp.Body).Decode(&md)
	resp.Body.Close()
	if md.Resource != audience {
		t.Fatalf("metadata resource = %q, want %q", md.Resource, audience)
	}

	// Authenticated MCP traffic works at the prefix.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize at prefix: status %d", resp.StatusCode)
	}
}

func TestResourceMetadataRequiresAuth(t *testing.T) {
	if _, _, err := argon.New("x").ResourceMetadata(); err == nil {
		t.Fatal("want error without Auth")
	}
}
