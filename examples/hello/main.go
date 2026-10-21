// Command hello is the smallest useful Argon server.
//
//	go run ./examples/hello              # stdio
//	go run ./examples/hello --http :8080 # Streamable HTTP
package main

import (
	"context"
	"log"

	"go.xchunk.org/argon"
)

type GreetInput struct {
	Name string `json:"name" jsonschema:"who to greet"`
}

func main() {
	s := argon.New("hello")
	argon.Tool(s, "greet", "Say hi", func(ctx context.Context, in GreetInput) (string, error) {
		return "Hi, " + in.Name + "!", nil
	}, argon.ReadOnly())
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
