package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as an MCP stdio server for AI coding assistants",
	Long: `Start an MCP (Model Context Protocol) server over stdio.

This allows AI coding assistants like Claude Code, Cursor, and Copilot
to interact with Kruda-specific tools for scaffolding, code generation,
route analysis, and documentation lookup.

Setup:
  kruda mcp init          Generate .mcp.json for your project
  kruda mcp --test        Verify the MCP server works
  kruda mcp               Start the server (called by AI assistant)`,
	RunE: runMCP,
}

func init() {
	mcpCmd.Flags().Bool("test", false, "run self-test to verify MCP server works")
}

func runMCP(cmd *cobra.Command, args []string) error {
	test, _ := cmd.Flags().GetBool("test")
	if test {
		return runMCPTest()
	}

	s := &mcpServer{
		in:  bufio.NewScanner(os.Stdin),
		out: bufio.NewWriter(os.Stdout),
	}
	return s.serve()
}

func runMCPTest() error {
	fmt.Println("Kruda MCP self-test")
	fmt.Println()

	s := &mcpServer{
		in:  bufio.NewScanner(strings.NewReader("")),
		out: bufio.NewWriter(os.Stderr), // discard protocol output
	}

	passed := 0
	total := 3

	// Test 1: initialize
	resp := s.dispatch(jsonrpcRequest{JSONRPC: "2.0", ID: float64(1), Method: "initialize"})
	if resp != nil && resp.Error == nil {
		fmt.Printf("  initialize       OK  (server: kruda-mcp %s)\n", version)
		passed++
	} else {
		fmt.Println("  initialize       FAIL")
	}

	// Test 2: tools/list
	resp = s.dispatch(jsonrpcRequest{JSONRPC: "2.0", ID: float64(2), Method: "tools/list"})
	if resp != nil && resp.Error == nil {
		tools := allTools()
		fmt.Printf("  tools/list       OK  (%d tools registered)\n", len(tools))
		passed++
	} else {
		fmt.Println("  tools/list       FAIL")
	}

	// Test 3: tools/call (kruda_docs — read-only, safe)
	params, _ := json.Marshal(mcpToolCallParams{Name: "kruda_docs", Arguments: map[string]any{"topic": "routing"}})
	resp = s.dispatch(jsonrpcRequest{JSONRPC: "2.0", ID: float64(3), Method: "tools/call", Params: params})
	if resp != nil && resp.Error == nil {
		if result, ok := resp.Result.(mcpToolCallResult); ok && !result.IsError && len(result.Content) > 0 {
			fmt.Println("  tools/call       OK  (kruda_docs: routing)")
			passed++
		} else {
			fmt.Println("  tools/call       FAIL: unexpected result")
		}
	} else {
		fmt.Println("  tools/call       FAIL")
	}

	fmt.Println()
	if passed == total {
		fmt.Printf("All %d checks passed. MCP server is working correctly.\n", total)
		return nil
	}
	return fmt.Errorf("%d/%d checks failed", total-passed, total)
}

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 types
// ---------------------------------------------------------------------------

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// MCP protocol types
// ---------------------------------------------------------------------------

type mcpInitResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ServerInfo      mcpServerInfo  `json:"serverInfo"`
	Capabilities    map[string]any `json:"capabilities"`
}

type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// ---------------------------------------------------------------------------
// MCP Server
// ---------------------------------------------------------------------------

type mcpServer struct {
	in  *bufio.Scanner
	out *bufio.Writer
}

func (s *mcpServer) serve() error {
	for s.in.Scan() {
		line := s.in.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &rpcErr{Code: -32700, Message: "parse error"},
			})
			continue
		}

		resp := s.dispatch(req)
		if resp != nil {
			s.writeResponse(*resp)
		}
	}
	return s.in.Err()
}

func (s *mcpServer) writeResponse(resp jsonrpcResponse) {
	resp.JSONRPC = "2.0"
	data, _ := json.Marshal(resp)
	s.out.Write(data)
	s.out.WriteByte('\n')
	s.out.Flush()
}

func (s *mcpServer) dispatch(req jsonrpcRequest) *jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return &jsonrpcResponse{
			ID: req.ID,
			Result: mcpInitResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo:      mcpServerInfo{Name: "kruda-mcp", Version: version},
				Capabilities:    map[string]any{"tools": map[string]any{}},
			},
		}

	case "notifications/initialized":
		return nil // notification, no response

	case "tools/list":
		return &jsonrpcResponse{
			ID:     req.ID,
			Result: mcpToolsListResult{Tools: allTools()},
		}

	case "tools/call":
		var params mcpToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &jsonrpcResponse{
				ID:    req.ID,
				Error: &rpcErr{Code: -32602, Message: "invalid params"},
			}
		}
		result := callTool(params.Name, params.Arguments)
		return &jsonrpcResponse{ID: req.ID, Result: result}

	default:
		return &jsonrpcResponse{
			ID:    req.ID,
			Error: &rpcErr{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

// ---------------------------------------------------------------------------
// Tool registry
// ---------------------------------------------------------------------------

func allTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "kruda_new",
			Description: "Scaffold a new Kruda project with a template (minimal, api, or fullstack)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project_name": map[string]any{"type": "string", "description": "Project directory name"},
					"template":     map[string]any{"type": "string", "enum": []string{"minimal", "api", "fullstack"}, "default": "minimal", "description": "Project template"},
				},
				"required": []string{"project_name"},
			},
		},
		{
			Name:        "kruda_add_handler",
			Description: "Generate a route handler file with optional typed C[T] pattern",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":  map[string]any{"type": "string", "description": "Route path, e.g. /users or /products/:id"},
					"typed": map[string]any{"type": "boolean", "default": false, "description": "Generate typed C[T] handler"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "kruda_add_resource",
			Description: "Generate a CRUD ResourceService[T, string] implementation with List, Get, Create, Update, Delete",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "Resource entity name in PascalCase, e.g. User, Product"},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "kruda_list_routes",
			Description: "Scan Go source files and list all registered Kruda routes",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"dir": map[string]any{"type": "string", "default": ".", "description": "Directory to scan"},
				},
			},
		},
		{
			Name:        "kruda_suggest_wing",
			Description: "Suggest Wing Feather hints (WingJSON, WingPlaintext, WingQuery, WingRender, WingStream) for routes",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"dir": map[string]any{"type": "string", "default": ".", "description": "Directory to scan for routes"},
				},
			},
		},
		{
			Name:        "kruda_docs",
			Description: "Look up Kruda documentation and code examples for a topic",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{
						"type":        "string",
						"description": "Topic to look up",
						"enum":        docTopics(),
					},
				},
				"required": []string{"topic"},
			},
		},
	}
}

func callTool(name string, args map[string]any) mcpToolCallResult {
	var text string
	var err error

	switch name {
	case "kruda_new":
		text, err = toolNew(args)
	case "kruda_add_handler":
		text, err = toolAddHandler(args)
	case "kruda_add_resource":
		text, err = toolAddResource(args)
	case "kruda_list_routes":
		text, err = toolListRoutes(args)
	case "kruda_suggest_wing":
		text, err = toolSuggestWing(args)
	case "kruda_docs":
		text, err = toolDocs(args)
	default:
		return mcpToolCallResult{
			Content: []mcpContent{{Type: "text", Text: "unknown tool: " + name}},
			IsError: true,
		}
	}

	if err != nil {
		return mcpToolCallResult{
			Content: []mcpContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		}
	}
	return mcpToolCallResult{
		Content: []mcpContent{{Type: "text", Text: text}},
	}
}

// ---------------------------------------------------------------------------
// Tool: kruda_new
// ---------------------------------------------------------------------------

func toolNew(args map[string]any) (string, error) {
	name, _ := args["project_name"].(string)
	if name == "" {
		return "", fmt.Errorf("project_name is required")
	}
	tmpl, _ := args["template"].(string)
	if tmpl == "" {
		tmpl = "minimal"
	}
	switch tmpl {
	case "minimal", "api", "fullstack":
	default:
		return "", fmt.Errorf("unknown template %q: must be minimal, api, or fullstack", tmpl)
	}

	if isNonEmptyDir(name) {
		return "", fmt.Errorf("directory %q already exists and is not empty", name)
	}

	data := templateData{ProjectName: name, ModuleName: name}
	tmplDir := filepath.Join("templates", tmpl)
	if err := scaffoldFromFS(templateFS, tmplDir, name, data); err != nil {
		return "", fmt.Errorf("scaffolding failed: %w", err)
	}

	return fmt.Sprintf("Created project %q with %s template.\n\nNext steps:\n  cd %s\n  go mod tidy\n  go run .", name, tmpl, name), nil
}

// ---------------------------------------------------------------------------
// Tool: kruda_add_handler
// ---------------------------------------------------------------------------

func toolAddHandler(args map[string]any) (string, error) {
	routePath, _ := args["path"].(string)
	if routePath == "" {
		return "", fmt.Errorf("path is required")
	}
	typed, _ := args["typed"].(bool)

	funcName := handlerNameFromPath(routePath)
	if funcName == "" {
		return "", fmt.Errorf("cannot derive handler name from path %q", routePath)
	}

	tmplStr := handlerTemplate
	if typed {
		tmplStr = typedHandlerTemplate
	}

	tmpl, err := template.New("handler").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	data := handlerData{FuncName: funcName, Path: routePath, Method: "GET"}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}

	fileName := strings.ToLower(funcName) + ".go"
	outPath := filepath.Join("handlers", fileName)
	if err := os.MkdirAll("handlers", 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}

	result := fmt.Sprintf("Generated handler: %s\nRoute: %s %s", outPath, data.Method, routePath)
	if typed {
		result += fmt.Sprintf("\nTyped handler with C[%sRequest] pattern", funcName)
	}
	result += "\n\n" + sb.String()
	return result, nil
}

// ---------------------------------------------------------------------------
// Tool: kruda_add_resource
// ---------------------------------------------------------------------------

func toolAddResource(args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	name = strings.ToUpper(name[:1]) + name[1:]

	tmpl, err := template.New("resource").Parse(resourceTemplate)
	if err != nil {
		return "", err
	}

	data := resourceData{Name: name, LowerName: strings.ToLower(name)}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}

	fileName := strings.ToLower(name) + "_service.go"
	outPath := filepath.Join("services", fileName)
	if err := os.MkdirAll("services", 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}

	return fmt.Sprintf("Generated resource service: %s\nType: %sService implements ResourceService[%s, string]\nMethods: List, Get, Create, Update, Delete\n\n%s", outPath, name, name, sb.String()), nil
}

// ---------------------------------------------------------------------------
// Tool: kruda_list_routes
// ---------------------------------------------------------------------------

var routeRe = regexp.MustCompile(`\.\s*(Get|Post|Put|Patch|Delete|Head|Options)\s*\(\s*"([^"]+)"`)
var resourceRe = regexp.MustCompile(`\.Resource\s*(?:\[[^\]]+\])?\s*\(\s*(?:[^,]+,\s*)*"([^"]+)"`)
var groupRe = regexp.MustCompile(`\.Group\s*\(\s*"([^"]+)"`)

type routeInfo struct {
	Method string
	Path   string
	File   string
	Line   int
}

func toolListRoutes(args map[string]any) (string, error) {
	dir, _ := args["dir"].(string)
	if dir == "" {
		dir = "."
	}

	routes, err := scanRoutes(dir)
	if err != nil {
		return "", err
	}

	if len(routes) == 0 {
		return "No Kruda routes found in " + dir, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d routes in %s:\n\n", len(routes), dir))
	sb.WriteString(fmt.Sprintf("%-8s %-30s %s\n", "METHOD", "PATH", "FILE"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")
	for _, r := range routes {
		sb.WriteString(fmt.Sprintf("%-8s %-30s %s:%d\n", r.Method, r.Path, r.File, r.Line))
	}
	return sb.String(), nil
}

func scanRoutes(dir string) ([]routeInfo, error) {
	var routes []routeInfo

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		prefix := ""

		for i, line := range lines {
			if m := groupRe.FindStringSubmatch(line); m != nil {
				prefix = m[1]
			}

			if m := routeRe.FindStringSubmatch(line); m != nil {
				routes = append(routes, routeInfo{
					Method: strings.ToUpper(m[1]),
					Path:   prefix + m[2],
					File:   path,
					Line:   i + 1,
				})
			}

			if m := resourceRe.FindStringSubmatch(line); m != nil {
				rpath := prefix + m[1]
				for _, method := range []string{"GET", "GET", "POST", "PUT", "DELETE"} {
					p := rpath
					if method == "GET" && routes[len(routes)-1].Path == rpath {
						p = rpath + "/:id"
					} else if method == "PUT" || method == "DELETE" {
						p = rpath + "/:id"
					}
					routes = append(routes, routeInfo{
						Method: method,
						Path:   p,
						File:   path,
						Line:   i + 1,
					})
				}
			}
		}

		return nil
	})

	return routes, err
}

// ---------------------------------------------------------------------------
// Tool: kruda_suggest_wing
// ---------------------------------------------------------------------------

func toolSuggestWing(args map[string]any) (string, error) {
	dir, _ := args["dir"].(string)
	if dir == "" {
		dir = "."
	}

	routes, err := scanRoutes(dir)
	if err != nil {
		return "", err
	}

	if len(routes) == 0 {
		return "No routes found to analyze in " + dir, nil
	}

	var sb strings.Builder
	sb.WriteString("Wing Feather Hint Suggestions:\n\n")
	sb.WriteString(fmt.Sprintf("%-8s %-25s %-18s %s\n", "METHOD", "PATH", "FEATHER", "REASON"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, r := range routes {
		feather, reason := suggestFeather(r)
		sb.WriteString(fmt.Sprintf("%-8s %-25s %-18s %s\n", r.Method, r.Path, feather, reason))
	}

	sb.WriteString("\nTo apply, add the hint as the last argument:\n")
	sb.WriteString("  app.Get(\"/path\", handler, kruda.WingJSON())\n")
	sb.WriteString("\nAvailable hints:\n")
	sb.WriteString("  WingPlaintext() — minimal responses (health, ping, plain text)\n")
	sb.WriteString("  WingJSON()      — JSON serialization endpoints\n")
	sb.WriteString("  WingQuery()     — database read/write operations\n")
	sb.WriteString("  WingRender()    — HTML template rendering\n")
	sb.WriteString("  WingStream()    — SSE / chunked streaming\n")

	return sb.String(), nil
}

func suggestFeather(r routeInfo) (string, string) {
	lpath := strings.ToLower(r.Path)

	// SSE / streaming
	if strings.Contains(lpath, "/sse") || strings.Contains(lpath, "/stream") || strings.Contains(lpath, "/events") {
		return "WingStream()", "streaming endpoint"
	}

	// Health / ping / status
	if lpath == "/health" || lpath == "/ping" || lpath == "/status" || lpath == "/healthz" || lpath == "/readyz" {
		return "WingPlaintext()", "health check, minimal response"
	}

	// Plaintext
	if lpath == "/plaintext" || lpath == "/" {
		return "WingPlaintext()", "plain text response"
	}

	// HTML / render / template
	if strings.Contains(lpath, "/render") || strings.Contains(lpath, "/page") || strings.Contains(lpath, "/template") || strings.Contains(lpath, "/fortune") {
		return "WingRender()", "HTML template rendering"
	}

	// DB operations: write methods or paths with :id
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE" {
		return "WingQuery()", "database write operation"
	}

	// DB read: path with :id param or typical DB patterns
	if strings.Contains(r.Path, ":id") || strings.Contains(lpath, "/db") ||
		strings.Contains(lpath, "/queries") || strings.Contains(lpath, "/updates") {
		return "WingQuery()", "database read/write"
	}

	// JSON endpoint
	if lpath == "/json" || strings.Contains(lpath, "/api") {
		return "WingJSON()", "JSON serialization"
	}

	// Default: JSON for GET
	return "WingJSON()", "default for GET endpoints"
}

// ---------------------------------------------------------------------------
// Tool: kruda_docs
// ---------------------------------------------------------------------------

func docTopics() []string {
	topics := make([]string, 0, len(krudaDocs))
	for k := range krudaDocs {
		topics = append(topics, k)
	}
	return topics
}

func toolDocs(args map[string]any) (string, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return "Available topics: " + strings.Join(docTopics(), ", "), nil
	}
	doc, ok := krudaDocs[topic]
	if !ok {
		return "", fmt.Errorf("unknown topic %q. Available: %s", topic, strings.Join(docTopics(), ", "))
	}
	return doc, nil
}

var krudaDocs = map[string]string{
	"typed-handlers": `# Typed Handlers — C[T] Pattern

Kruda's typed handlers parse request body, query params, and path params
into a single struct at compile time.

` + "```" + `go
type CreateUser struct {
    Name  string ` + "`" + `json:"name" validate:"required,min=2"` + "`" + `
    Email string ` + "`" + `json:"email" validate:"required,email"` + "`" + `
}

type User struct {
    ID    string ` + "`" + `json:"id"` + "`" + `
    Name  string ` + "`" + `json:"name"` + "`" + `
    Email string ` + "`" + `json:"email"` + "`" + `
}

// Register typed POST handler
kruda.Post[CreateUser, User](app, "/users", func(c *kruda.C[CreateUser]) (*User, error) {
    // c.In is already parsed and validated
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})

// Register typed GET handler with query params
type ListParams struct {
    Page  int    ` + "`" + `query:"page" default:"1"` + "`" + `
    Limit int    ` + "`" + `query:"limit" default:"20"` + "`" + `
    Sort  string ` + "`" + `query:"sort" default:"id"` + "`" + `
}

kruda.Get[ListParams, []User](app, "/users", func(c *kruda.C[ListParams]) (*[]User, error) {
    // c.In.Page, c.In.Limit, c.In.Sort are parsed from query string
    return &users, nil
})
` + "```" + `

Input parsing pipeline: defaults → body → query → params → validate
Validation uses struct tags and returns structured ValidationError with []FieldError.`,

	"routing": `# Routing

Kruda uses a radix tree router with zero-alloc matching, AOT-compiled at startup.

` + "```" + `go
app := kruda.New()

// Static routes
app.Get("/", handler)
app.Post("/users", handler)

// Path parameters
app.Get("/users/:id", handler)           // c.Param("id")
app.Get("/files/*path", handler)         // c.Param("path") — wildcard

// Optional params
app.Get("/users/:id?", handler)          // matches /users and /users/123

// Regex constraints
app.Get("/users/:id<[0-9]+>", handler)   // numeric only

// Route groups
api := app.Group("/api/v1")
api.Get("/users", listUsers)
api.Post("/users", createUser)

// Guarded groups (middleware on group)
admin := api.Group("/admin").Guard(authMiddleware)
admin.Get("/stats", statsHandler)

// Method chaining
app.Get("/a", h).Post("/b", h).Put("/c", h)
` + "```" + `

Supported methods: Get, Post, Put, Patch, Delete, Head, Options`,

	"middleware": `# Middleware

` + "```" + `go
app := kruda.New()

// Built-in middleware
app.Use(
    middleware.Recovery(),    // panic recovery
    middleware.Logger(),      // request logging via slog
    middleware.CORS(),        // CORS headers
    middleware.RequestID(),   // X-Request-Id header
)

// Per-route timeout
app.Get("/slow", handler, kruda.Timeout(30*time.Second))

// Group-level middleware with Guard
api := app.Group("/api").Guard(jwt.New(jwt.Config{
    Secret: []byte(os.Getenv("JWT_SECRET")),
}))

// Custom middleware
func MyMiddleware() kruda.Middleware {
    return func(next kruda.Handler) kruda.Handler {
        return func(c *kruda.Ctx) error {
            // before
            err := next(c)
            // after
            return err
        }
    }
}
` + "```" + `

Middleware pipeline: onRequest → beforeHandle → handler → afterHandle
Built-in: Recovery, Logger, CORS, RequestID, Timeout`,

	"di": `# Dependency Injection

Kruda has built-in, optional DI — no codegen required.

` + "```" + `go
// Create container
c := kruda.NewContainer()

// Register services
c.Give(&UserService{})                        // singleton
c.GiveLazy(func() (*DBPool, error) {          // lazy initialization
    return pgxpool.New(ctx, dsn)
})
c.GiveNamed("write", &DB{DSN: "primary"})     // named dependency
c.GiveNamed("read", &DB{DSN: "replica"})

// Use with app
app := kruda.New(kruda.WithContainer(c))

// Resolve in handlers
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)  // type-safe resolve
    db := kruda.MustResolveNamed[*DB](c, "read")
    return c.JSON(svc.ListAll(db))
})
` + "```" + `

DI is optional — use it only if your project needs it.
No reflection magic, pure Go generics.`,

	"resource": `# Auto CRUD — Resource

Implement ResourceService[T] and get 5 REST endpoints automatically.

` + "```" + `go
// Define your entity
type Product struct {
    ID    string  ` + "`" + `json:"id"` + "`" + `
    Name  string  ` + "`" + `json:"name"` + "`" + `
    Price float64 ` + "`" + `json:"price"` + "`" + `
}

// Implement the interface
type ProductService struct { db *pgxpool.Pool }

func (s *ProductService) List(ctx context.Context) ([]Product, error) { ... }
func (s *ProductService) Get(ctx context.Context, id string) (Product, error) { ... }
func (s *ProductService) Create(ctx context.Context, item Product) (Product, error) { ... }
func (s *ProductService) Update(ctx context.Context, id string, item Product) (Product, error) { ... }
func (s *ProductService) Delete(ctx context.Context, id string) error { ... }

// Register — creates 5 endpoints
kruda.Resource[Product, string](app, "/products", &ProductService{db: db})
// GET    /products       → List
// GET    /products/:id   → Get
// POST   /products       → Create
// PUT    /products/:id   → Update
// DELETE /products/:id   → Delete
` + "```",

	"wing": `# Wing Transport & Feather Hints

Wing is Kruda's custom async I/O transport:
- Linux: epoll + eventfd (bypasses net/http and fasthttp)
- macOS: kqueue
- Windows/fallback: net/http

` + "```" + `go
// Enable Wing (default on Linux)
app := kruda.New(kruda.Wing())

// Or explicit transport selection
app := kruda.New(kruda.FastHTTP())   // fasthttp
app := kruda.New(kruda.NetHTTP())    // stdlib net/http
` + "```" + `

## Feather Hints

Per-route I/O strategy optimization. Add as the last argument to route registration:

` + "```" + `go
app.Get("/json", handler, kruda.WingJSON())         // JSON serialization
app.Get("/text", handler, kruda.WingPlaintext())     // minimal plain text
app.Get("/db",   handler, kruda.WingQuery())         // database operations
app.Get("/page", handler, kruda.WingRender())        // HTML template rendering
app.Get("/sse",  handler, kruda.WingStream())        // SSE / streaming
` + "```" + `

| Feather | Best For | Optimization |
|---------|----------|-------------|
| WingPlaintext | /health, /ping, static text | Pre-allocated headers, minimal alloc |
| WingJSON | JSON APIs | Sonic encoder, pre-sized buffers |
| WingQuery | DB read/write | Connection-aware scheduling |
| WingRender | HTML templates | Buffer pooling for large responses |
| WingStream | SSE, chunked | Chunked transfer, no buffering |

Feather hints are optional — routes work fine without them.
They provide 5-15% additional throughput on hot paths.`,

	"config": `# Configuration

` + "```" + `go
app := kruda.New(
    // Timeouts
    kruda.WithReadTimeout(10 * time.Second),
    kruda.WithWriteTimeout(10 * time.Second),
    kruda.WithIdleTimeout(120 * time.Second),
    kruda.WithShutdownTimeout(30 * time.Second),

    // Limits
    kruda.WithBodyLimit(1024 * 1024),  // 1MB

    // Transport
    kruda.Wing(),       // or FastHTTP(), NetHTTP()

    // DI
    kruda.WithContainer(container),

    // Dev mode (rich error pages)
    kruda.WithDevMode(true),
)

// Environment variables with prefix
app := kruda.New(
    kruda.WithEnvPrefix("APP"),
)
// Reads: APP_READ_TIMEOUT, APP_WRITE_TIMEOUT, APP_IDLE_TIMEOUT,
//        APP_BODY_LIMIT, APP_SHUTDOWN_TIMEOUT
` + "```",

	"error-handling": `# Error Handling

` + "```" + `go
// Map Go errors to HTTP status codes
app.MapError(ErrNotFound, 404, "resource not found")
kruda.MapErrorType[*ValidationError](app, 422, "validation failed")

// Return errors from handlers — auto-mapped
func handler(c *kruda.Ctx) error {
    user, err := db.FindUser(id)
    if err != nil {
        return err  // mapped to appropriate status code
    }
    return c.JSON(user)
}

// Custom error with details
return kruda.NewError(403, "forbidden").WithDetail("insufficient permissions")

// Structured validation errors (auto from typed handlers)
// Response:
// {
//   "error": "validation failed",
//   "code": 422,
//   "fields": [
//     {"field": "email", "message": "must be a valid email", "tag": "email"}
//   ]
// }
` + "```" + `

In dev mode (WithDevMode(true)), errors render as rich HTML pages with
source code context, similar to Next.js error pages.`,

	"sse": `# Server-Sent Events (SSE)

` + "```" + `go
app.Get("/events", func(c *kruda.Ctx) error {
    return c.SSE(func(stream *kruda.SSEStream) error {
        for i := 0; i < 10; i++ {
            stream.Event("counter", map[string]int{"value": i})
            time.Sleep(time.Second)
        }
        return nil
    })
})
` + "```" + `

SSE automatically sets Content-Type: text/event-stream and flushes after each event.
Use WingStream() feather hint for optimal performance.`,

	"file-upload": `# File Upload

` + "```" + `go
type UploadRequest struct {
    File   *kruda.Upload ` + "`" + `form:"file" validate:"required" max_size:"5mb" mime:"image/*"` + "`" + `
    Title  string        ` + "`" + `form:"title" validate:"required"` + "`" + `
}

kruda.Post[UploadRequest, Response](app, "/upload", func(c *kruda.C[UploadRequest]) (*Response, error) {
    file := c.In.File
    // file.Name, file.Size, file.ContentType, file.Open()
    return &Response{URL: savedURL}, nil
})
` + "```" + `

Validation tags: max_size (e.g. "5mb", "500kb"), mime (e.g. "image/*", "application/pdf")`,

	"validation": `# Validation

Kruda validates struct tags automatically in typed handlers.

` + "```" + `go
type CreateUser struct {
    Name  string ` + "`" + `json:"name" validate:"required,min=2,max=50"` + "`" + `
    Email string ` + "`" + `json:"email" validate:"required,email"` + "`" + `
    Age   int    ` + "`" + `json:"age" validate:"gte=0,lte=150"` + "`" + `
}
` + "```" + `

Supported validators:
  required, email, url, uuid
  min=N, max=N (string length)
  gte=N, lte=N, gt=N, lt=N (numeric)
  len=N (exact length)
  oneof=a b c (enum values)

Validation errors return 422 with structured response:
` + "```" + `json
{
  "error": "validation failed",
  "code": 422,
  "fields": [
    {"field": "name", "message": "must be at least 2 characters", "tag": "min"},
    {"field": "email", "message": "must be a valid email", "tag": "email"}
  ]
}
` + "```",

	"testing": `# Testing Kruda Applications

` + "```" + `go
func TestGetUser(t *testing.T) {
    app := kruda.New()
    app.Get("/users/:id", getUserHandler)

    // Use httptest
    req := httptest.NewRequest("GET", "/users/123", nil)
    resp := httptest.NewRecorder()
    app.ServeHTTP(resp, req)

    if resp.Code != 200 {
        t.Fatalf("expected 200, got %d", resp.Code)
    }

    var user User
    json.NewDecoder(resp.Body).Decode(&user)
    if user.ID != "123" {
        t.Errorf("expected ID 123, got %s", user.ID)
    }
}

func TestTypedHandler(t *testing.T) {
    app := kruda.New()
    kruda.Post[CreateUser, User](app, "/users", createUserHandler)

    body := strings.NewReader(` + "`" + `{"name":"Tiger","email":"tiger@kruda.dev"}` + "`" + `)
    req := httptest.NewRequest("POST", "/users", body)
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    app.ServeHTTP(resp, req)

    if resp.Code != 200 {
        t.Fatalf("expected 200, got %d", resp.Code)
    }
}
` + "```" + `

Kruda implements http.Handler, so standard httptest works out of the box.`,

	"openapi": `# OpenAPI 3.1 Generation

Kruda auto-generates OpenAPI 3.1 specs from typed handler registrations.

` + "```" + `go
app := kruda.New()

// Register typed handlers — OpenAPI schema extracted automatically
kruda.Post[CreateUser, User](app, "/users", createUser)
kruda.Get[ListParams, []User](app, "/users", listUsers)

// Serve OpenAPI spec
app.Get("/openapi.json", app.OpenAPIHandler())

// Or get the spec programmatically
spec := app.OpenAPISpec()
` + "```" + `

The spec includes:
- Path + method from route registration
- Request body schema from input type (CreateUser)
- Response schema from output type (User)
- Validation rules from struct tags (required, min, max, etc.)
- Query parameter schemas from query tags

Use with contrib/swagger for Swagger UI:
` + "```" + `go
import "github.com/go-kruda/kruda/contrib/swagger"
app.Get("/docs/*", swagger.Handler("/openapi.json"))
` + "```",
}
