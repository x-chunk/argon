package argon

// Middleware wraps a tool Handler with cross-cutting behaviour such as
// logging, metrics or authorization. Register it for all tools with Use or
// for one tool with With. It does not apply to resources and prompts.
//
//	audit := func(next argon.Handler) argon.Handler {
//		return func(ctx context.Context, req *argon.Request) (*argon.Result, error) {
//			log.Printf("tool %s called", req.Tool)
//			return next(ctx, req)
//		}
//	}
//	s := argon.New("notes", argon.Use(audit))
type Middleware func(Handler) Handler

// chain wraps h so that mws[0] is the outermost layer.
func chain(h Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
