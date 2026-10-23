package argon

// Middleware wraps a Handler with cross-cutting behaviour.
type Middleware func(Handler) Handler

// chain wraps h so that mws[0] is the outermost layer.
func chain(h Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
