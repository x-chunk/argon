package core

// Middleware wraps a Handler with cross-cutting behaviour.
type Middleware func(Handler) Handler

// Chain wraps h so that mws[0] is the outermost layer.
func Chain(h Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
