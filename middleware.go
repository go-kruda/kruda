package kruda

// MiddlewareFunc is a type alias for HandlerFunc for semantic clarity.
// Using a type alias (=) means MiddlewareFunc and HandlerFunc are interchangeable.
type MiddlewareFunc = HandlerFunc

// buildChain creates a pre-built handler chain from global, group, and route-level handlers.
// Called once at route registration time — the returned slice is reused for every request,
// achieving zero allocation on the hot path.
//
// Order: global middleware → group middleware → route handler
func buildChain(global, group []HandlerFunc, handler HandlerFunc) []HandlerFunc {
	chain := make([]HandlerFunc, 0, len(global)+len(group)+1)
	chain = append(chain, global...)
	chain = append(chain, group...)
	chain = append(chain, handler)
	return chain
}
