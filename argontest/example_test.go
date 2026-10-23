package argontest_test

import (
	"context"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
)

func newServer() *argon.Server {
	s := argon.New("greeter")
	argon.Tool(s, "greet", "Say hi", func(ctx context.Context, in struct {
		Name string `json:"name"`
	}) (string, error) {
		return "Hi, " + in.Name, nil
	}, argon.Scopes("greet"))
	return s
}

// In a test, call tools through an in-memory MCP session; WithIdentity
// runs the call as a given caller so scopes apply.
func ExampleCall() {
	t := &testing.T{} // in a real test, use the *testing.T you are given
	got, err := argontest.Call[string](t, newServer(), "greet",
		map[string]string{"name": "Ann"},
		argontest.WithIdentity(argon.Identity{Subject: "ann", Scopes: []string{"greet"}}))
	if err != nil || got != "Hi, Ann" {
		t.Fatalf("got %q, %v", got, err)
	}
}
