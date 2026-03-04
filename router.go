package kruda

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
)

// errPathTraversal is returned when a request path contains directory traversal sequences.
var errPathTraversal = errors.New("kruda: path traversal detected")

// cleanPath normalizes a request path and rejects traversal attempts.
// It decodes percent-encoded sequences, resolves . and .. segments via path.Clean,
// and rejects any result that still contains "..".
// This is called before route matching in ServeKruda.
func cleanPath(raw string) (string, error) {
	// Strip null bytes — prevents null byte injection attacks
	if strings.ContainsRune(raw, 0) {
		raw = strings.ReplaceAll(raw, "\x00", "")
	}

	// Decode percent-encoded sequences in a loop to prevent double-encode bypass
	// (e.g. %252e%252e → %2e%2e → ..). Max 3 iterations to bound cost.
	decoded := raw
	for range 3 {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return "", err
		}
		if next == decoded {
			break // stable — no more encoded sequences
		}
		decoded = next
	}

	// Check for path traversal: walk segments and track depth.
	// If depth goes below 0, the path tries to escape above root → reject.
	if isTraversal(decoded) {
		return "", errPathTraversal
	}

	// Normalize with path.Clean (resolves . and safe ..)
	cleaned := path.Clean(decoded)

	// Ensure leading slash
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	return cleaned, nil
}

// isTraversal reports whether the path contains ".." segments that would
// traverse above the root directory. Safe relative ".." (e.g. /a/b/../c)
// is allowed; only root-escaping traversal is rejected.
func isTraversal(p string) bool {
	depth := 0
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
			// skip empty segments and current-dir references
		case "..":
			depth--
			if depth < 0 {
				return true
			}
		default:
			depth++
		}
	}
	return false
}

// Method index constants for the methodTrees array.
const (
	mGET = iota
	mPOST
	mPUT
	mDELETE
	mPATCH
	mOPTIONS
	mHEAD
	mCOUNT // number of standard methods (array size)
)

// methodIndex returns the array index for a standard HTTP method using
// first-byte + length dispatch for O(1) identification.
// Returns -1 for custom/unknown methods (fallback to map).
func methodIndex(method string) int {
	switch len(method) {
	case 3:
		if method[0] == 'G' && method[1] == 'E' && method[2] == 'T' { // GET
			return mGET
		}
		if method[0] == 'P' && method[1] == 'U' && method[2] == 'T' { // PUT
			return mPUT
		}
	case 4:
		if method[0] == 'P' && method[1] == 'O' { // POST
			return mPOST
		}
		if method[0] == 'H' && method[1] == 'E' { // HEAD
			return mHEAD
		}
	case 5:
		if method[0] == 'P' && method[1] == 'A' { // PATCH
			return mPATCH
		}
	case 6:
		if method[0] == 'D' && method[1] == 'E' { // DELETE
			return mDELETE
		}
	case 7:
		if method[0] == 'O' && method[1] == 'P' { // OPTIONS
			return mOPTIONS
		}
	}
	return -1 // custom method → fallback to map
}

// Router is a radix tree router with a separate tree per HTTP method.
// It provides O(1) child lookup via the indices string on each node.
type Router struct {
	methodTrees         [mCOUNT]*node
	trees               map[string]*node
	staticRoutes        [mCOUNT]map[string][]HandlerFunc // indexed by method for O(1) first lookup
	compiled            bool
	allowedMethodsCache map[string]string
}

// node is a single node in the radix tree.
// Static nodes store literal path text in path (e.g. "users/").
// Param nodes have param set (e.g. "id") and path stores the pattern (e.g. ":id").
// Wildcard nodes have wildcard=true and param stores the capture name.
type node struct {
	path     string
	children []*node
	indices  string // first byte of each child's path for O(1) lookup
	handlers []HandlerFunc
	param    string
	wildcard bool
	regex    *regexp.Regexp
	optional bool
	hits     uint32 // frequency counter for AOT sort optimization
}

// standardMethods lists the HTTP methods that get pre-initialized trees.
var standardMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}

// newRouter creates a router with empty root nodes for all standard HTTP methods.
func newRouter() *Router {
	trees := make(map[string]*node, len(standardMethods))
	for _, m := range standardMethods {
		trees[m] = &node{path: "/"}
	}
	r := &Router{trees: trees}
	// Populate methodTrees array for O(1) method dispatch
	for _, m := range standardMethods {
		if idx := methodIndex(m); idx >= 0 {
			r.methodTrees[idx] = trees[m]
		}
	}
	return r
}

// Compile freezes the router tree and performs AOT optimizations:
// sorts children by frequency, flattens single-child static chains,
// builds a static route map for O(1) exact-match lookup,
// and caches allowed methods for static paths.
func (r *Router) Compile() {
	r.optimizeTree()

	for _, root := range r.trees {
		flattenNode(root)
	}

	for method, root := range r.trees {
		m := make(map[string][]HandlerFunc)
		collectStaticRoutes(root, "/", m)
		if len(m) > 0 {
			if idx := methodIndex(method); idx >= 0 {
				r.staticRoutes[idx] = m
			}
		}
	}

	r.allowedMethodsCache = make(map[string]string)
	staticPaths := r.collectStaticPaths()
	var tmpParams routeParams
	for _, path := range staticPaths {
		var b strings.Builder
		first := true
		for _, method := range standardMethods {
			tmpParams.reset()
			if r.find(method, path, &tmpParams) != nil {
				if !first {
					b.WriteString(", ")
				}
				b.WriteString(method)
				first = false
			}
		}
		if b.Len() > 0 {
			r.allowedMethodsCache[path] = b.String()
		}
	}

	r.compiled = true
}

// collectStaticRoutes recursively traverses the radix tree and collects all
// terminal nodes whose full path contains no param (`:`) or wildcard (`*`) segments.
// Results are stored in the provided map as path → handlers.
func collectStaticRoutes(n *node, prefix string, out map[string][]HandlerFunc) {
	if n.param != "" || n.wildcard {
		return
	}

	current := prefix
	if n.path != "" {
		if n.path == "/" {
			if len(current) > 0 && current[len(current)-1] != '/' {
				current = current + "/"
			}
		} else if current == "/" {
			current = "/" + n.path
		} else {
			current = current + n.path
		}
	}

	if n.handlers != nil {
		p := current
		if p != "/" && len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		out[p] = n.handlers
	}

	for _, child := range n.children {
		collectStaticRoutes(child, current, out)
	}
}

// collectStaticPaths walks all method trees and returns unique terminal static paths.
func (r *Router) collectStaticPaths() []string {
	seen := make(map[string]bool)
	for _, root := range r.trees {
		collectPaths(root, "/", seen)
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}

// collectPaths recursively collects terminal static paths from the tree.
func collectPaths(n *node, prefix string, seen map[string]bool) {
	current := prefix
	if n.path != "" && n.path != "/" {
		if current == "/" {
			current = "/" + n.path
		} else {
			current = current + n.path
		}
	}

	if n.param != "" || n.wildcard {
		return
	}

	if n.handlers != nil {
		p := current
		if p != "/" && len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		seen[p] = true
	}

	for _, child := range n.children {
		collectPaths(child, current, seen)
	}
}

// optimizeTree sorts children of each node by descending hits for cache locality.
func (r *Router) optimizeTree() {
	for _, root := range r.trees {
		optimizeNode(root)
	}
}

// optimizeNode sorts children: statics by hits (desc), then params, wildcards last.
// Rebuilds the indices string after sorting.
func optimizeNode(n *node) {
	if len(n.children) > 1 {
		sort.SliceStable(n.children, func(i, j int) bool {
			ci, cj := n.children[i], n.children[j]
			if ci.wildcard != cj.wildcard {
				return !ci.wildcard
			}
			if (ci.param != "") != (cj.param != "") {
				return ci.param == ""
			}
			return ci.hits > cj.hits
		})
		// Rebuild indices
		var b strings.Builder
		for _, child := range n.children {
			if child.wildcard {
				b.WriteByte('*')
			} else if child.param != "" {
				b.WriteByte(':')
			} else if len(child.path) > 0 {
				b.WriteByte(child.path[0])
			}
		}
		n.indices = b.String()
	}
	for _, child := range n.children {
		optimizeNode(child)
	}
}

// flattenNode merges single-child static chains that have no handlers.
// e.g. node "users" → child "/" → child "settings" becomes "users/settings"
func flattenNode(n *node) {
	for i := 0; i < len(n.children); i++ {
		child := n.children[i]
		if child.param == "" && !child.wildcard &&
			len(child.children) == 1 && child.handlers == nil {
			grandchild := child.children[0]
			if grandchild.param == "" && !grandchild.wildcard {
				merged := &node{
					path:     child.path + grandchild.path,
					children: grandchild.children,
					indices:  grandchild.indices,
					handlers: grandchild.handlers,
					hits:     child.hits + grandchild.hits,
					regex:    grandchild.regex,
					optional: grandchild.optional,
				}
				n.children[i] = merged
				// Re-check same index — merged node may still be flattenable
				i--
				continue
			}
		}
		flattenNode(child)
	}
}

// addRoute inserts a route into the tree for the given method.
// It panics if the router is compiled or if a duplicate route is detected.
// For standard methods, it also updates methodTrees[idx] to keep the array
// and map pointing to the same node object.
func (r *Router) addRoute(method, path string, handlers []HandlerFunc) {
	if r.compiled {
		panic("kruda: cannot add route after Compile()")
	}
	if path == "" || path[0] != '/' {
		panic("kruda: route path must begin with '/'")
	}

	root, ok := r.trees[method]
	if !ok {
		root = &node{path: "/"}
		r.trees[method] = root
	}

	// Keep methodTrees array in sync with trees map for standard methods.
	// Both must point to the SAME node object (not a copy).
	if idx := methodIndex(method); idx >= 0 {
		r.methodTrees[idx] = root
	}

	if path == "/" {
		if root.handlers != nil {
			panic(fmt.Sprintf("kruda: duplicate route %s /", method))
		}
		root.handlers = handlers
		return
	}

	// Parse into segments and insert
	segments := parseSegments(path)
	insertRoute(root, segments, 0, handlers, method, path)
}

// segment represents a parsed path segment.
type segment struct {
	text     string         // raw text (e.g. "users", ":id", "*filepath")
	param    string         // param name if param/wildcard, empty for static
	wildcard bool           // true for wildcard
	regex    *regexp.Regexp // compiled regex constraint
	optional bool           // true for optional param
	static   bool           // true for static segment
}

// parseSegments splits "/users/:id/posts" into typed segments.
func parseSegments(path string) []segment {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	segments := make([]segment, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		segments = append(segments, parseOneSegment(p))
	}
	return segments
}

// parseOneSegment parses a single path segment.
func parseOneSegment(s string) segment {
	if s[0] == '*' {
		name := s[1:]
		if name == "" {
			panic("kruda: wildcard must have a name")
		}
		return segment{text: s, param: name, wildcard: true}
	}
	if s[0] == ':' {
		raw := s[1:]
		var optional bool
		if strings.HasSuffix(raw, "?") {
			raw = raw[:len(raw)-1]
			optional = true
		}
		var name string
		var rx *regexp.Regexp
		if idx := strings.IndexByte(raw, '<'); idx >= 0 {
			end := strings.IndexByte(raw, '>')
			if end < 0 {
				panic(fmt.Sprintf("kruda: invalid regex constraint in %s", s))
			}
			name = raw[:idx]
			pattern := raw[idx+1 : end]
			if err := validateRegexSafety(pattern); err != nil {
				panic(fmt.Sprintf("kruda: unsafe regex in route %s: %v", s, err))
			}
			rx = regexp.MustCompile("^" + pattern + "$")
		} else {
			name = raw
		}
		return segment{text: s, param: name, regex: rx, optional: optional}
	}
	return segment{text: s, static: true}
}

// validateRegexSafety rejects patterns prone to catastrophic backtracking.
// Detects quantified groups whose contents also contain quantifiers,
// e.g. (a+)+, (a*)+, (a+)*, (a{2,})+.
func validateRegexSafety(pattern string) error {
	// Track whether each group depth contains a quantifier inside it.
	type groupInfo struct {
		hasInnerQuantifier bool
	}
	var stack []groupInfo

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '\\':
			i++ // skip escaped char
		case '(':
			stack = append(stack, groupInfo{})
		case ')':
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			// Check if the closing paren is followed by a quantifier
			if i+1 < len(pattern) && isQuantifier(pattern[i+1]) && top.hasInnerQuantifier {
				return fmt.Errorf("nested quantifier at position %d", i+1)
			}
		case '+', '*':
			// Mark the current group (if any) as containing a quantifier
			if len(stack) > 0 {
				stack[len(stack)-1].hasInnerQuantifier = true
			}
		case '{':
			if len(stack) > 0 {
				stack[len(stack)-1].hasInnerQuantifier = true
			}
		}
	}
	return nil
}

func isQuantifier(c byte) bool {
	return c == '+' || c == '*' || c == '?' || c == '{'
}

// insertRoute inserts segments into the tree starting at the given node.
// For static segments, it uses radix tree compression.
// For param/wildcard segments, it creates special child nodes.
func insertRoute(n *node, segments []segment, idx int, handlers []HandlerFunc, method, fullPath string) {
	if idx >= len(segments) {
		if n.handlers != nil {
			panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
		}
		n.handlers = handlers
		return
	}

	seg := segments[idx]

	if seg.wildcard {
		if idx != len(segments)-1 {
			panic(fmt.Sprintf("kruda: wildcard must be the last segment in route %s", fullPath))
		}
		// Check for existing wildcard child
		for _, child := range n.children {
			if child.wildcard {
				if child.handlers != nil {
					panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
				}
				child.handlers = handlers
				return
			}
		}
		child := &node{
			param:    seg.param,
			wildcard: true,
			handlers: handlers,
		}
		n.children = append(n.children, child)
		n.indices += "*"
		return
	}

	if !seg.static {
		for _, child := range n.children {
			if child.param == seg.param && !child.wildcard {
				if seg.optional && idx == len(segments)-1 {
					if child.handlers != nil {
						panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
					}
					child.handlers = handlers
					// Also set on parent for the "without param" case
					if n.handlers != nil {
						panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
					}
					n.handlers = handlers
					return
				}
				insertRoute(child, segments, idx+1, handlers, method, fullPath)
				return
			}
		}
		child := &node{
			param:    seg.param,
			regex:    seg.regex,
			optional: seg.optional,
		}
		n.children = append(n.children, child)
		n.indices += ":"

		if seg.optional && idx == len(segments)-1 {
			child.handlers = handlers
			if n.handlers != nil {
				panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
			}
			n.handlers = handlers
			return
		}

		insertRoute(child, segments, idx+1, handlers, method, fullPath)
		return
	}

	// Static segment — "segment/" if not last, "segment" if last
	var staticStr string
	if idx < len(segments)-1 {
		staticStr = seg.text + "/"
	} else {
		staticStr = seg.text
	}

	insertStatic(n, staticStr, segments, idx, handlers, method, fullPath)
}

// insertStatic inserts a static string into the radix tree at node n,
// then continues inserting remaining segments.
func insertStatic(n *node, str string, segments []segment, segIdx int, handlers []HandlerFunc, method, fullPath string) {
	if len(str) > 0 {
		idx := strings.IndexByte(n.indices, str[0])
		if idx >= 0 {
			child := n.children[idx]
			if child.param != "" || child.wildcard {
				panic(fmt.Sprintf("kruda: route conflict at %s", fullPath))
			}
			splitAndInsert(child, str, segments, segIdx, handlers, method, fullPath)
			return
		}
	}

	// No matching child — create new one
	isLast := segIdx == len(segments)-1
	if isLast {
		child := &node{path: str, handlers: handlers}
		n.children = append(n.children, child)
		if len(str) > 0 {
			n.indices += string(str[0])
		}
	} else {
		child := &node{path: str}
		n.children = append(n.children, child)
		if len(str) > 0 {
			n.indices += string(str[0])
		}
		insertRoute(child, segments, segIdx+1, handlers, method, fullPath)
	}
}

// splitAndInsert handles radix tree node splitting for static paths.
func splitAndInsert(n *node, str string, segments []segment, segIdx int, handlers []HandlerFunc, method, fullPath string) {
	i := longestPrefix(str, n.path)

	// Split node if needed
	if i < len(n.path) {
		child := &node{
			path:     n.path[i:],
			children: n.children,
			indices:  n.indices,
			handlers: n.handlers,
		}
		n.children = []*node{child}
		n.indices = string(child.path[0])
		n.handlers = nil
		n.path = n.path[:i]
	}

	if i < len(str) {
		remaining := str[i:]
		isLast := segIdx == len(segments)-1

		// Check for existing child with matching first byte
		idx := strings.IndexByte(n.indices, remaining[0])
		if idx >= 0 {
			child := n.children[idx]
			if child.param == "" && !child.wildcard {
				splitAndInsert(child, remaining, segments, segIdx, handlers, method, fullPath)
				return
			}
		}

		if isLast {
			newChild := &node{path: remaining, handlers: handlers}
			n.children = append(n.children, newChild)
			n.indices += string(remaining[0])
		} else {
			newChild := &node{path: remaining}
			n.children = append(n.children, newChild)
			n.indices += string(remaining[0])
			insertRoute(newChild, segments, segIdx+1, handlers, method, fullPath)
		}
	} else {
		// Exact match with node
		isLast := segIdx == len(segments)-1
		if isLast {
			if n.handlers != nil {
				panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
			}
			n.handlers = handlers
		} else {
			insertRoute(n, segments, segIdx+1, handlers, method, fullPath)
		}
	}
}

// longestPrefix returns the length of the longest common prefix of a and b.
func longestPrefix(a, b string) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	i := 0
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

// find looks up a path in the method tree and returns the handler chain.
// Populates params with extracted values. Zero alloc on the hot path.
//
// Lookup order: static map O(1) → method array → tree traversal.
func (r *Router) find(method, path string, params *routeParams) []HandlerFunc {
	// Resolve method index once — used for both static lookup and tree selection.
	idx := methodIndex(method)

	// O(1) static route lookup — array index (not map) for method, then path map.
	if idx >= 0 {
		if routes := r.staticRoutes[idx]; routes != nil {
			if handlers, ok := routes[path]; ok {
				return handlers
			}
		}
	}

	var root *node
	if idx >= 0 {
		root = r.methodTrees[idx]
	} else {
		root = r.trees[method]
	}
	if root == nil {
		return nil
	}

	trackHits := !r.compiled

	if path == "/" {
		if root.handlers != nil {
			if trackHits {
				atomic.AddUint32(&root.hits, 1)
			}
			return root.handlers
		}
		for _, child := range root.children {
			if child.optional && child.handlers != nil {
				params.set(child.param, "")
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
		}
		return nil
	}

	return findInNode(root, path[1:], params, trackHits)
}

// findInNode searches for a path match starting from node n.
// path has no leading slash. trackHits enables frequency counting before Compile.
func findInNode(n *node, path string, params *routeParams, trackHits bool) []HandlerFunc {
	if len(path) > 0 && len(n.indices) > 0 {
		idx := strings.IndexByte(n.indices, path[0])
		if idx >= 0 {
			child := n.children[idx]
			if child.param == "" && !child.wildcard {
				if strings.HasPrefix(path, child.path) {
					remaining := path[len(child.path):]
					if remaining == "" {
						if child.handlers != nil {
							if trackHits {
								atomic.AddUint32(&child.hits, 1)
							}
							return child.handlers
						}
						for _, gc := range child.children {
							if gc.optional && gc.handlers != nil {
								params.set(gc.param, "")
								if trackHits {
									atomic.AddUint32(&gc.hits, 1)
								}
								return gc.handlers
							}
						}
					} else {
						result := findInNode(child, remaining, params, trackHits)
						if result != nil {
							return result
						}
					}
				} else if child.path == path+"/" {
					// optional param: /users/:id? — /users should also match
					if child.handlers != nil {
						if trackHits {
							atomic.AddUint32(&child.hits, 1)
						}
						return child.handlers
					}
					for _, gc := range child.children {
						if gc.optional && gc.handlers != nil {
							params.set(gc.param, "")
							if trackHits {
								atomic.AddUint32(&gc.hits, 1)
							}
							return gc.handlers
						}
					}
				}
			}
		}
	}

	for _, child := range n.children {
		if child.param == "" || child.wildcard {
			continue
		}

		end := strings.IndexByte(path, '/')
		if end == -1 {
			end = len(path)
		}
		value := path[:end]

		if value == "" {
			if child.optional && child.handlers != nil {
				params.set(child.param, "")
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
			continue
		}

		if child.regex != nil && !child.regex.MatchString(value) {
			continue
		}

		params.set(child.param, value)

		if end == len(path) {
			if child.handlers != nil {
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
			for _, gc := range child.children {
				if gc.optional && gc.handlers != nil {
					params.set(gc.param, "")
					if trackHits {
						atomic.AddUint32(&gc.hits, 1)
					}
					return gc.handlers
				}
			}
		} else {
			result := findInNode(child, path[end+1:], params, trackHits)
			if result != nil {
				return result
			}
		}

		params.del(child.param)
	}

	// Try wildcard children
	for _, child := range n.children {
		if !child.wildcard {
			continue
		}
		params.set(child.param, path)
		if trackHits {
			atomic.AddUint32(&child.hits, 1)
		}
		return child.handlers
	}

	return nil
}

// findAllowedMethods returns a comma-separated list of methods that match the path,
// or empty string if none. Uses allowedMethodsCache for static paths (zero alloc).
func (r *Router) findAllowedMethods(path string) string {
	if r.allowedMethodsCache != nil {
		if cached, ok := r.allowedMethodsCache[path]; ok {
			return cached
		}
	}

	// Slow path: scan trees (dynamic paths with params/wildcards)
	var tmpParams routeParams
	var b strings.Builder
	first := true
	for _, method := range standardMethods {
		tmpParams.reset()
		if r.find(method, path, &tmpParams) != nil {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(method)
			first = false
		}
	}
	return b.String()
}
