package sdkadapter

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// recoverMiddleware turns a panic anywhere in request handling into an
// internal JSON-RPC error. The SDK runs handlers on their own goroutines and
// does not recover, so without this a panic in conversion code would crash
// the whole process. Argon's pipelines recover handler panics themselves;
// this is the last line of defense. The stack is logged, never returned.
func recoverMiddleware(log *slog.Logger) mcp.Middleware {
	if log == nil {
		log = slog.Default()
	}
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (res mcp.Result, err error) {
			defer func() {
				if p := recover(); p != nil {
					log.ErrorContext(ctx, "request handler panicked", "method", method, "panic", p, "stack", string(debug.Stack()))
					res, err = nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
				}
			}()
			return next(ctx, method, req)
		}
	}
}
