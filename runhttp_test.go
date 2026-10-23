package argon_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.xchunk.org/argon"
)

// freeAddr returns a loopback address with a currently free port.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// waitListening dials addr until it accepts connections.
func waitListening(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never started listening", addr)
}

const initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

// postInitialize sends an MCP initialize request and returns the status.
func postInitialize(t *testing.T, url string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(initializeRequest))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func pingServer() *argon.Server {
	s := argon.New("ping")
	argon.Tool(s, "ping", "Ping", func(context.Context, struct{}) (string, error) { return "pong", nil })
	return s
}

func TestRunHTTPGracefulShutdown(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- pingServer().RunHTTP(ctx, addr) }()

	waitListening(t, addr)
	if code := postInitialize(t, "http://"+addr+"/"); code != http.StatusOK {
		t.Fatalf("initialize: status %d", code)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunHTTP after cancel: %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunHTTP did not return after its context was cancelled")
	}
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("server still listening after shutdown")
	}
}

func TestRunHTTPListenError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	err = pingServer().RunHTTP(t.Context(), l.Addr().String())
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("RunHTTP on a busy port: err = %v, want a listen error", err)
	}
}

func TestRunReportsRegistrationErrors(t *testing.T) {
	s := argon.New("bad")
	argon.Tool(s, "has space", "", func(context.Context, struct{}) (string, error) { return "", nil })
	if err := s.RunHTTP(t.Context(), freeAddr(t)); err == nil {
		t.Fatal("RunHTTP must report registration errors before listening")
	}
}
