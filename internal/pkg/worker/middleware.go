package worker

// Middleware is a function that wraps a Handler with additional functionality
type Middleware func(Handler) Handler

// Chain combines multiple middlewares into a single middleware
func Chain(middlewares ...Middleware) Middleware {
	return func(handler Handler) Handler {
		// Apply middlewares in reverse order so they execute in the correct order
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}
