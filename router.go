package kruda

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// errPathTraversal is returned when a request path contains directory traversal sequences.
var errPathTraversal = errors.New("kruda: path traversal detected")

// cleanPath normalizes a request path and rejects traversal attempts.
// It decodes percent-encoded sequences, resolves . and .. segments via path.Clean,
// and rejects any result that still contains "..".
// This is called before route matching in ServeKruda.
func cleanPath(raw string) (string, error) {
	// 1. Decode percent-encoded sequences (e.g. %2e%2e%2f → ../)
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}

	// 2. Check for path traversal: walk segments and track depth.
	// If depth goes below 0, the path tries to escape above root → reject.
	if isTraversal(decoded) {
		return "", errPathTraversal
	}

	// 3. Normalize with path.Clean (resolves . and safe ..)
	cleaned := path.Clean(decoded)

	// 4. Ensure leading slash
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

// Router is a radix tree router with a separate tree per HTTP method.
// It provides O(1) child lookup via the indices string on each node.
type Router struct {
	trees               map[string]*node // method → root node
	compiled            bool
	allowedMethodsCache map[string]string // path → "GET, POST, ..." (built at Compile)
}

// node is a single node in the radix tree.
// Static nodes store literal path text in path (e.g. "users/").
// Param nodes have param set (e.g. "id") and path stores the pattern (e.g. ":id").
// Wildcard nodes have wildcard=true and param stores the capture name.
type node struct {
	path     string         // static path segment text
	children []*node        // child nodes
	indices  string         // first byte of each child's path for O(1) lookup
	handlers []HandlerFunc  // pre-built chain (nil if not terminal)
	param    string         // parameter name (e.g. "id" for ":id")
	wildcard bool           // true for "*filepath" patterns
	regex    *regexp.Regexp // compiled regex for ":id<[0-9]+>" patterns
	optional bool           // true for ":id?" patterns
	hits     uint32         // frequency counter for AOT sort optimization
}

// standardMethods lists the HTTP methods that get pre-initialized trees.
var standardMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}

// newRouter creates a router with empty root nodes for all standard HTTP methods.
func newRouter() *Router {
	trees := make(map[string]*node, len(standardMethods))
	for _, m := range standardMethods {
		trees[m] = &node{path: "/"}
	}
	return &Router{trees: trees}
}

// Compile freezes the router tree and performs AOT optimizations:
// 1. Sort children by frequency (most-hit routes first)
// 2. Flatten single-child static chains to reduce tree depth
// 3. Build allowed methods cache for static paths (P3-006 fix)
func (r *Router) Compile() {
	// 1. Sort children by frequency (statics by hits desc, then params, wildcards last)
	r.optimizeTree()

	// 2. Flatten single-child static chains
	for _, root := range r.trees {
		flattenNode(root)
	}

	// 3. Build allowed methods cache for static paths
	r.allowedMethodsCache = make(map[string]string)
	staticPaths := r.collectStaticPaths()
	tmpParams := make(map[string]string, 4)
	for _, path := range staticPaths {
		var b strings.Builder
		first := true
		for _, method := range standardMethods {
			clear(tmpParams)
			if r.find(method, path, tmpParams) != nil {
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

	// Skip param/wildcard subtrees — only collect fully static paths
	if n.param != "" || n.wildcard {
		return
	}

	if n.handlers != nil {
		// Normalize: ensure leading slash, remove trailing slash (except root)
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
			// Wildcards always last
			if ci.wildcard != cj.wildcard {
				return !ci.wildcard
			}
			// Params after statics
			if (ci.param != "") != (cj.param != "") {
				return ci.param == ""
			}
			// Sort statics by hits descending
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
			rx = regexp.MustCompile("^" + raw[idx+1:end] + "$")
		} else {
			name = raw
		}
		return segment{text: s, param: name, regex: rx, optional: optional}
	}
	return segment{text: s, static: true}
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
		// Param segment
		// Look for existing param child with same name
		for _, child := range n.children {
			if child.param == seg.param && !child.wildcard {
				if seg.optional && idx == len(segments)-1 {
					if child.handlers != nil {
						panic(fmt.Sprintf("kruda: duplicate route %s %s", method, fullPath))
					}
					child.handlers = handlers
					// Also set on parent for "without param" case
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

	// Static segment — use radix tree insertion
	// Build the static string: "segment/" if not last, "segment" if last
	var staticStr string
	if idx < len(segments)-1 && !segments[idx+1].static {
		// Next segment is param/wildcard — just use the segment text + "/"
		staticStr = seg.text + "/"
	} else if idx < len(segments)-1 {
		// Next segment is also static — combine with "/"
		staticStr = seg.text + "/"
	} else {
		// Last segment
		staticStr = seg.text
	}

	insertStatic(n, staticStr, segments, idx, handlers, method, fullPath)
}

// insertStatic inserts a static string into the radix tree at node n,
// then continues inserting remaining segments.
func insertStatic(n *node, str string, segments []segment, segIdx int, handlers []HandlerFunc, method, fullPath string) {
	// Look for a child with matching first byte
	if len(str) > 0 {
		idx := strings.IndexByte(n.indices, str[0])
		if idx >= 0 {
			child := n.children[idx]
			if child.param != "" || child.wildcard {
				// This shouldn't happen for static insertion
				panic(fmt.Sprintf("kruda: route conflict at %s", fullPath))
			}
			// Found matching child — split if needed
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
// It populates the params map with extracted parameter values.
// Zero allocation on the hot path — params map is pre-allocated on Ctx.
func (r *Router) find(method, path string, params map[string]string) []HandlerFunc {
	root, ok := r.trees[method]
	if !ok {
		return nil
	}

	// Track hits only before Compile — after Compile the tree is frozen
	trackHits := !r.compiled

	// Root path
	if path == "/" {
		if root.handlers != nil {
			if trackHits {
				atomic.AddUint32(&root.hits, 1)
			}
			return root.handlers
		}
		// Check optional param children
		for _, child := range root.children {
			if child.optional && child.handlers != nil {
				params[child.param] = ""
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
		}
		return nil
	}

	// Strip leading slash
	return findInNode(root, path[1:], params, trackHits)
}

// findInNode searches for a path match starting from node n.
// path has no leading slash. trackHits enables frequency counting before Compile.
func findInNode(n *node, path string, params map[string]string, trackHits bool) []HandlerFunc {
	// 1. Try static children via indices (O(1) lookup by first byte)
	if len(path) > 0 && len(n.indices) > 0 {
		idx := strings.IndexByte(n.indices, path[0])
		if idx >= 0 {
			child := n.children[idx]
			if child.param == "" && !child.wildcard {
				// Static child — check prefix match
				if strings.HasPrefix(path, child.path) {
					remaining := path[len(child.path):]
					if remaining == "" {
						// Exact match
						if child.handlers != nil {
							if trackHits {
								atomic.AddUint32(&child.hits, 1)
							}
							return child.handlers
						}
						// Check optional param children
						for _, gc := range child.children {
							if gc.optional && gc.handlers != nil {
								params[gc.param] = ""
								if trackHits {
									atomic.AddUint32(&gc.hits, 1)
								}
								return gc.handlers
							}
						}
					} else {
						// Continue searching in child
						result := findInNode(child, remaining, params, trackHits)
						if result != nil {
							return result
						}
					}
				} else if child.path == path+"/" {
					// Path matches static child minus trailing slash (e.g. path="users", child.path="users/")
					// This happens with optional params: /users/:id? where /users should also match
					if child.handlers != nil {
						if trackHits {
							atomic.AddUint32(&child.hits, 1)
						}
						return child.handlers
					}
					for _, gc := range child.children {
						if gc.optional && gc.handlers != nil {
							params[gc.param] = ""
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

	// 2. Try param children
	for _, child := range n.children {
		if child.param == "" || child.wildcard {
			continue
		}

		// Extract param value up to next "/"
		end := strings.IndexByte(path, '/')
		if end == -1 {
			end = len(path)
		}
		value := path[:end]

		if value == "" {
			if child.optional && child.handlers != nil {
				params[child.param] = ""
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
			continue
		}

		// Regex validation
		if child.regex != nil && !child.regex.MatchString(value) {
			continue
		}

		params[child.param] = value

		if end == len(path) {
			// No more path after param value
			if child.handlers != nil {
				if trackHits {
					atomic.AddUint32(&child.hits, 1)
				}
				return child.handlers
			}
			// Check optional param grandchildren
			for _, gc := range child.children {
				if gc.optional && gc.handlers != nil {
					params[gc.param] = ""
					if trackHits {
						atomic.AddUint32(&gc.hits, 1)
					}
					return gc.handlers
				}
			}
		} else {
			// More path after "/" — continue from child
			result := findInNode(child, path[end+1:], params, trackHits)
			if result != nil {
				return result
			}
		}

		// Didn't match — clean up
		delete(params, child.param)
	}

	// 3. Try wildcard children
	for _, child := range n.children {
		if !child.wildcard {
			continue
		}
		params[child.param] = path
		if trackHits {
			atomic.AddUint32(&child.hits, 1)
		}
		return child.handlers
	}

	return nil
}

// tmpParamsPool reuses maps for findAllowedMethods slow path (P3-006 fix).
var tmpParamsPool = sync.Pool{
	New: func() any { return make(map[string]string, 4) },
}

// findAllowedMethods scans all method trees for a path match.
// Returns a comma-separated list of methods that match (e.g. "GET, POST"),
// or an empty string if no method matches.
// P3-006 fix: uses allowedMethodsCache for static paths (zero alloc),
// falls back to pooled tmpParams + strings.Builder for dynamic paths.
func (r *Router) findAllowedMethods(path string) string {
	// Fast path: check cache (zero alloc for static paths)
	if r.allowedMethodsCache != nil {
		if cached, ok := r.allowedMethodsCache[path]; ok {
			return cached
		}
	}

	// Slow path: scan trees (dynamic paths with params/wildcards)
	tmpParams := tmpParamsPool.Get().(map[string]string)
	defer func() {
		clear(tmpParams)
		tmpParamsPool.Put(tmpParams)
	}()

	var b strings.Builder
	first := true
	for _, method := range standardMethods {
		clear(tmpParams)
		if r.find(method, path, tmpParams) != nil {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(method)
			first = false
		}
	}
	return b.String()
}
