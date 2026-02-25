package kruda

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// devErrorData holds all data for the dev error page template.
type devErrorData struct {
	ErrorMessage    string
	ErrorType       string
	StatusCode      int
	SourceCode      []sourceLine
	StackTrace      []stackFrame
	RequestMethod   string
	RequestPath     string
	RequestHeaders  map[string]string
	QueryParams     map[string]string
	RequestBody     string
	AvailableRoutes []devRouteInfo
	EnvVars         map[string]string
	Suggestions     []string
}

// sourceLine represents a single line of source code in the error display.
type sourceLine struct {
	Number  int
	Code    string
	IsError bool
}

// stackFrame represents a single frame in the stack trace.
type stackFrame struct {
	File     string
	Line     int
	Function string
}

// devRouteInfo holds method and path for route listing in the dev error page.
type devRouteInfo struct {
	Method string
	Path   string
}

// sensitiveEnvKeys contains patterns to filter from env var display.
var sensitiveEnvKeys = []string{
	"SECRET", "PASSWORD", "TOKEN", "KEY", "CREDENTIAL", "AUTH",
	"PRIVATE", "CERT", "PASS", "PWD",
	"MYSQL_PASSWORD", "POSTGRES_PASSWORD", "REDIS_PASSWORD",
}

// sourceCache caches file contents by path+mtime for dev mode performance.
type sourceCache struct {
	lines []string
	mtime time.Time
}

var (
	sourceCacheMu sync.Mutex
	sourceCacheMap = make(map[string]sourceCache)
)

// devErrorTmpl is the parsed HTML template for the dev error page.
// Compiled once at init time — zero cost on the hot path.
var devErrorTmpl = template.Must(template.New("devError").Parse(devErrorHTML))

// renderDevErrorPage renders a rich HTML error page in development mode.
// Returns nil immediately if DevMode is false (zero overhead in production).
// Returns a non-nil error only if the page was successfully rendered (signals
// to the caller that the response has been written).
func renderDevErrorPage(c *Ctx, err error, statusCode int) error {
	// Gate: never render in production — zero overhead
	if !c.app.config.DevMode {
		return nil
	}

	data := devErrorData{
		ErrorMessage:  err.Error(),
		ErrorType:     fmt.Sprintf("%T", err),
		StatusCode:    statusCode,
		RequestMethod: c.Method(),
		RequestPath:   c.Path(),
	}

	// 1. Parse stack trace
	data.StackTrace = captureStackTrace()

	// 2. Read source code ±10 lines around first frame (best-effort)
	if len(data.StackTrace) > 0 {
		data.SourceCode = readSourceContext(data.StackTrace[0].File, data.StackTrace[0].Line, 10)
	}

	// 3. Collect request details
	data.RequestHeaders = collectRequestHeaders(c)
	data.QueryParams = collectQueryParams(c)

	// 4. Skip body for multipart/form-data; truncate at 1KB to avoid large payloads in error page
	if !strings.Contains(c.Header("Content-Type"), "multipart/form-data") {
		body := c.BodyString()
		if len(body) > 1024 {
			body = body[:1024] + "... (truncated)"
		}
		data.RequestBody = body
	}

	// 5. Collect available routes
	data.AvailableRoutes = collectDevRoutes(c.app)

	// 6. Filter environment variables
	data.EnvVars = filterEnvVars()

	// 7. Generate suggestions
	data.Suggestions = generateSuggestions(err, statusCode, c)

	// 8. Render HTML
	var buf bytes.Buffer
	if tmplErr := devErrorTmpl.Execute(&buf, data); tmplErr != nil {
		// Fallback: plain text if template fails
		c.Status(statusCode)
		return c.Text(fmt.Sprintf("Error: %v\nTemplate error: %v", err, tmplErr))
	}

	c.Status(statusCode)
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	return c.HTML(buf.String())
}

// captureStackTrace parses the current goroutine's stack trace from runtime.Stack().
func captureStackTrace() []stackFrame {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	raw := string(buf[:n])

	var frames []stackFrame
	lines := strings.Split(raw, "\n")

	// Stack format:
	//   goroutine N [running]:
	//   package.Function(args)
	//       /path/to/file.go:LINE +0xNN
	for i := 1; i < len(lines)-1; i += 2 {
		funcLine := strings.TrimSpace(lines[i])
		if i+1 >= len(lines) {
			break
		}
		fileLine := strings.TrimSpace(lines[i+1])

		// Parse file:line from the file line
		file, line := parseFileLine(fileLine)
		if file == "" {
			continue
		}

		// Clean function name (remove args)
		fn := funcLine
		if idx := strings.LastIndex(fn, "("); idx > 0 {
			fn = fn[:idx]
		}

		frames = append(frames, stackFrame{
			File:     trimAbsPath(file),
			Line:     line,
			Function: fn,
		})
	}

	return frames
}

// parseFileLine extracts file path and line number from a stack trace line.
// Input format: "/path/to/file.go:42 +0x1a3"
func parseFileLine(s string) (string, int) {
	// Remove offset suffix like " +0x1a3"
	if idx := strings.LastIndex(s, " +0x"); idx > 0 {
		s = s[:idx]
	}

	// Split on last colon to get file:line
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return "", 0
	}

	file := s[:lastColon]
	lineStr := s[lastColon+1:]
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return "", 0
	}

	return file, line
}

// trimAbsPath converts an absolute file path to a relative one by stripping
// everything up to and including the module root (identified by "/go-kruda/kruda/").
// Falls back to the basename if no known prefix is found.
func trimAbsPath(path string) string {
	const mod = "/go-kruda/kruda/"
	if idx := strings.LastIndex(path, mod); idx >= 0 {
		return path[idx+len(mod):]
	}
	// Fallback: strip GOPATH/pkg/mod prefix patterns
	for _, prefix := range []string{"/src/", "/mod/"} {
		if idx := strings.LastIndex(path, prefix); idx >= 0 {
			return path[idx+len(prefix):]
		}
	}
	return path
}

// readSourceContext reads ±radius lines around the target line from a file.
// Returns nil if the file cannot be read (best-effort).
func readSourceContext(file string, targetLine, radius int) []sourceLine {
	sourceCacheMu.Lock()
	defer sourceCacheMu.Unlock()

	stat, err := os.Stat(file)
	if err != nil {
		return nil
	}

	cached, exists := sourceCacheMap[file]
	if exists && cached.mtime.Equal(stat.ModTime()) {
		// Use cached lines
		return buildSourceLines(cached.lines, targetLine, radius)
	}

	// Read and cache
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	sourceCacheMap[file] = sourceCache{
		lines: lines,
		mtime: stat.ModTime(),
	}

	return buildSourceLines(lines, targetLine, radius)
}

// buildSourceLines creates sourceLine slice from lines around target.
func buildSourceLines(lines []string, targetLine, radius int) []sourceLine {
	start := targetLine - radius - 1
	if start < 0 {
		start = 0
	}
	end := targetLine + radius
	if end > len(lines) {
		end = len(lines)
	}

	result := make([]sourceLine, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, sourceLine{
			Number:  i + 1,
			Code:    lines[i],
			IsError: i+1 == targetLine,
		})
	}
	return result
}

// collectRequestHeaders extracts request headers from the context.
func collectRequestHeaders(c *Ctx) map[string]string {
	if c.request == nil {
		return map[string]string{}
	}
	if p, ok := c.request.(transport.AllHeadersProvider); ok {
		return p.AllHeaders()
	}
	return map[string]string{}
}

// collectQueryParams extracts query parameters from the request URL.
func collectQueryParams(c *Ctx) map[string]string {
	if c.request == nil {
		return map[string]string{}
	}
	if p, ok := c.request.(transport.AllQueryProvider); ok {
		return p.AllQuery()
	}
	return map[string]string{}
}

// collectDevRoutes walks the router trees and collects all registered routes.
func collectDevRoutes(app *App) []devRouteInfo {
	var routes []devRouteInfo
	for method, root := range app.router.trees {
		walkRouteTree(root, "/", method, &routes)
	}
	return routes
}

// walkRouteTree recursively walks a radix tree node to collect routes.
func walkRouteTree(n *node, prefix, method string, routes *[]devRouteInfo) {
	current := prefix
	if n.path != "" && n.path != "/" {
		if current == "/" {
			current = "/" + n.path
		} else {
			current = current + n.path
		}
	}

	// Add param/wildcard segments to the path display
	if n.param != "" {
		if n.wildcard {
			current = current + "*" + n.param
		} else {
			current = current + ":" + n.param
		}
	}

	if n.handlers != nil {
		p := current
		if p != "/" && len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		*routes = append(*routes, devRouteInfo{Method: method, Path: p})
	}

	for _, child := range n.children {
		walkRouteTree(child, current, method, routes)
	}
}

// filterEnvVars returns environment variables with sensitive keys excluded.
func filterEnvVars() map[string]string {
	vars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if isSensitiveEnvKey(key) {
			continue
		}
		vars[key] = parts[1]
	}
	return vars
}

// isSensitiveEnvKey checks if an environment variable key matches sensitive patterns.
func isSensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range sensitiveEnvKeys {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// generateSuggestions produces helpful hints based on error type and status code.
func generateSuggestions(err error, statusCode int, c *Ctx) []string {
	var suggestions []string
	switch statusCode {
	case 404:
		suggestions = append(suggestions, "Did you forget to register this route?")
		suggestions = append(suggestions, fmt.Sprintf("Check: app.Get(\"%s\", handler)", c.Path()))
	case 405:
		suggestions = append(suggestions, "This route exists but doesn't accept "+c.Method()+" requests")
		suggestions = append(suggestions, "Check the Allow header for supported methods")
	case 413:
		suggestions = append(suggestions, "Request body exceeds the maximum size limit")
		suggestions = append(suggestions, "Use WithMaxBodySize() to increase the limit")
	case 422:
		suggestions = append(suggestions, "Check your request body matches the expected struct")
		suggestions = append(suggestions, "Verify JSON field names and types")
	case 500:
		suggestions = append(suggestions, "Check the stack trace for the error source")
		suggestions = append(suggestions, "Look for nil pointer dereferences or unhandled errors")
	}
	return suggestions
}

// devErrorHTML is the inline HTML template for the dev error page.
// Uses html/template for auto-escaping (XSS prevention).
const devErrorHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.StatusCode}} — {{.ErrorMessage}}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,monospace;background:#1a1a2e;color:#e0e0e0;line-height:1.6}
.container{max-width:960px;margin:0 auto;padding:24px}
.error-header{background:#16213e;border-left:4px solid #e94560;padding:24px;border-radius:4px;margin-bottom:20px}
.error-header h1{color:#e94560;font-size:20px;margin-bottom:8px}
.error-header .type{color:#8892b0;font-size:14px;font-family:monospace}
.error-header .status{color:#64ffda;font-size:14px;margin-top:4px}
.section{background:#16213e;border-radius:4px;margin-bottom:12px;overflow:hidden}
.section-header{padding:12px 16px;cursor:pointer;display:flex;justify-content:space-between;align-items:center;user-select:none;border-bottom:1px solid #1a1a2e}
.section-header:hover{background:#1a2744}
.section-header h2{font-size:14px;color:#64ffda;font-weight:600}
.section-header .toggle{color:#8892b0;font-size:12px;transition:transform .2s}
.section-header .toggle.open{transform:rotate(90deg)}
.section-body{padding:16px;display:none}
.section-body.open{display:block}
.suggestions{padding:16px}
.suggestions li{color:#ffd700;margin-bottom:6px;font-size:13px;list-style:none}
.suggestions li::before{content:"💡 "}
.source-code{overflow-x:auto;font-size:13px}
.source-code table{width:100%;border-collapse:collapse}
.source-code td{padding:1px 8px;white-space:pre;font-family:'SF Mono',Monaco,'Cascadia Code',monospace}
.source-code .line-num{color:#4a5568;text-align:right;width:50px;user-select:none;border-right:1px solid #2d3748;padding-right:12px}
.source-code .line-code{padding-left:12px}
.source-code .error-line{background:rgba(233,69,96,0.15)}
.source-code .error-line .line-num{color:#e94560;font-weight:bold}
.stack-trace{font-family:'SF Mono',Monaco,monospace;font-size:12px}
.stack-frame{padding:6px 0;border-bottom:1px solid #1a1a2e}
.stack-frame:last-child{border-bottom:none}
.stack-func{color:#c792ea}
.stack-file{color:#8892b0}
.stack-line{color:#64ffda}
.details-table{width:100%;font-size:13px;border-collapse:collapse}
.details-table td{padding:4px 8px;border-bottom:1px solid #1a1a2e;vertical-align:top}
.details-table .key{color:#64ffda;font-weight:600;width:200px;font-family:monospace}
.details-table .val{color:#e0e0e0;font-family:monospace;word-break:break-all}
.routes-table{width:100%;font-size:13px;border-collapse:collapse}
.routes-table th{text-align:left;padding:6px 8px;color:#64ffda;border-bottom:2px solid #2d3748}
.routes-table td{padding:4px 8px;border-bottom:1px solid #1a1a2e;font-family:monospace}
.method-badge{display:inline-block;padding:1px 6px;border-radius:3px;font-size:11px;font-weight:bold;color:#1a1a2e}
.method-GET{background:#64ffda}.method-POST{background:#ffd700}.method-PUT{background:#c792ea}
.method-DELETE{background:#e94560;color:#fff}.method-PATCH{background:#82aaff}.method-OPTIONS{background:#8892b0}
.method-HEAD{background:#a0aec0}
.body-preview{font-family:monospace;font-size:12px;white-space:pre-wrap;word-break:break-all;max-height:200px;overflow-y:auto;background:#0f0f23;padding:12px;border-radius:4px}
.copy-btn{background:#64ffda;color:#1a1a2e;border:none;padding:8px 16px;border-radius:4px;cursor:pointer;font-size:13px;font-weight:600;margin-top:16px}
.copy-btn:hover{background:#4dd8b5}
.copy-btn:active{transform:scale(0.98)}
.env-filtered{color:#8892b0;font-style:italic;font-size:12px;margin-top:8px}
</style>
</head>
<body>
<div class="container">

<div class="error-header">
  <h1>{{.ErrorMessage}}</h1>
  <div class="type">{{.ErrorType}}</div>
  <div class="status">HTTP {{.StatusCode}} — {{.RequestMethod}} {{.RequestPath}}</div>
</div>

{{if .Suggestions}}
<div class="section">
  <div class="suggestions">
    <ul>{{range .Suggestions}}<li>{{.}}</li>{{end}}</ul>
  </div>
</div>
{{end}}

{{if .SourceCode}}
<div class="section">
  <div class="section-header" onclick="toggle(this)">
    <h2>Source Code</h2><span class="toggle open">▶</span>
  </div>
  <div class="section-body open">
    <div class="source-code"><table>
      {{range .SourceCode}}<tr{{if .IsError}} class="error-line"{{end}}>
        <td class="line-num">{{.Number}}</td>
        <td class="line-code">{{.Code}}</td>
      </tr>{{end}}
    </table></div>
  </div>
</div>
{{end}}

{{if .StackTrace}}
<div class="section">
  <div class="section-header" onclick="toggle(this)">
    <h2>Stack Trace</h2><span class="toggle open">▶</span>
  </div>
  <div class="section-body open">
    <div class="stack-trace">
      {{range .StackTrace}}<div class="stack-frame">
        <span class="stack-func">{{.Function}}</span><br>
        <span class="stack-file">{{.File}}</span>:<span class="stack-line">{{.Line}}</span>
      </div>{{end}}
    </div>
  </div>
</div>
{{end}}

<div class="section">
  <div class="section-header" onclick="toggle(this)">
    <h2>Request Details</h2><span class="toggle">▶</span>
  </div>
  <div class="section-body">
    <table class="details-table">
      <tr><td class="key">Method</td><td class="val">{{.RequestMethod}}</td></tr>
      <tr><td class="key">Path</td><td class="val">{{.RequestPath}}</td></tr>
    </table>
    {{if .QueryParams}}
    <h3 style="color:#64ffda;font-size:13px;margin:12px 0 6px">Query Parameters</h3>
    <table class="details-table">
      {{range $k, $v := .QueryParams}}<tr><td class="key">{{$k}}</td><td class="val">{{$v}}</td></tr>{{end}}
    </table>
    {{end}}
    {{if .RequestHeaders}}
    <h3 style="color:#64ffda;font-size:13px;margin:12px 0 6px">Headers</h3>
    <table class="details-table">
      {{range $k, $v := .RequestHeaders}}<tr><td class="key">{{$k}}</td><td class="val">{{$v}}</td></tr>{{end}}
    </table>
    {{end}}
    {{if .RequestBody}}
    <h3 style="color:#64ffda;font-size:13px;margin:12px 0 6px">Body</h3>
    <div class="body-preview">{{.RequestBody}}</div>
    {{end}}
  </div>
</div>

{{if .AvailableRoutes}}
<div class="section">
  <div class="section-header" onclick="toggle(this)">
    <h2>Available Routes ({{len .AvailableRoutes}})</h2><span class="toggle">▶</span>
  </div>
  <div class="section-body">
    <table class="routes-table">
      <thead><tr><th>Method</th><th>Path</th></tr></thead>
      <tbody>
        {{range .AvailableRoutes}}<tr>
          <td><span class="method-badge method-{{.Method}}">{{.Method}}</span></td>
          <td>{{.Path}}</td>
        </tr>{{end}}
      </tbody>
    </table>
  </div>
</div>
{{end}}

{{if .EnvVars}}
<div class="section">
  <div class="section-header" onclick="toggle(this)">
    <h2>Environment Variables</h2><span class="toggle">▶</span>
  </div>
  <div class="section-body">
    <table class="details-table">
      {{range $k, $v := .EnvVars}}<tr><td class="key">{{$k}}</td><td class="val">{{$v}}</td></tr>{{end}}
    </table>
    <div class="env-filtered">Variables containing SECRET, PASSWORD, TOKEN, KEY, CREDENTIAL, or AUTH are hidden.</div>
  </div>
</div>
{{end}}

<button class="copy-btn" onclick="copyError()">📋 Copy Error</button>

</div>
<script>
function toggle(el){
  var body=el.nextElementSibling;
  var arrow=el.querySelector('.toggle');
  if(body.classList.contains('open')){
    body.classList.remove('open');
    arrow.classList.remove('open');
  }else{
    body.classList.add('open');
    arrow.classList.add('open');
  }
}
function copyError(){
  var t="Error: {{.ErrorMessage}}\nType: {{.ErrorType}}\nStatus: {{.StatusCode}}\nMethod: {{.RequestMethod}}\nPath: {{.RequestPath}}\n";
  {{if .StackTrace}}t+="\nStack Trace:\n";{{range .StackTrace}}t+="  {{.Function}}\n    {{.File}}:{{.Line}}\n";{{end}}{{end}}
  navigator.clipboard.writeText(t).then(function(){
    var btn=document.querySelector('.copy-btn');
    btn.textContent='✅ Copied!';
    setTimeout(function(){btn.textContent='📋 Copy Error'},2000);
  });
}
</script>
</body>
</html>`
