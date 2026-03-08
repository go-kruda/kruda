package kruda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"

	"github.com/go-kruda/kruda/transport"
)

// TestClient provides a fluent API for testing Kruda applications
// without starting a real HTTP server. It is safe for concurrent use.
type TestClient struct {
	app     *App
	mu      sync.Mutex
	headers map[string]string
	cookies map[string]string
}

// NewTestClient creates a new test client for the given app.
func NewTestClient(app *App) *TestClient {
	return &TestClient{
		app:     app,
		headers: make(map[string]string),
		cookies: make(map[string]string),
	}
}

// WithHeader sets a default header for all requests from this client.
func (tc *TestClient) WithHeader(key, value string) *TestClient {
	tc.mu.Lock()
	tc.headers[key] = value
	tc.mu.Unlock()
	return tc
}

// WithCookie sets a default cookie for all requests from this client.
func (tc *TestClient) WithCookie(name, value string) *TestClient {
	tc.mu.Lock()
	tc.cookies[name] = value
	tc.mu.Unlock()
	return tc
}

// Get sends a GET request to the given path.
func (tc *TestClient) Get(path string) (*TestResponse, error) {
	return tc.do(http.MethodGet, path, nil)
}

// Post sends a POST request to the given path with the given body.
func (tc *TestClient) Post(path string, body any) (*TestResponse, error) {
	return tc.do(http.MethodPost, path, body)
}

// Put sends a PUT request to the given path with the given body.
func (tc *TestClient) Put(path string, body any) (*TestResponse, error) {
	return tc.do(http.MethodPut, path, body)
}

// Delete sends a DELETE request to the given path.
func (tc *TestClient) Delete(path string) (*TestResponse, error) {
	return tc.do(http.MethodDelete, path, nil)
}

// Patch sends a PATCH request to the given path with the given body.
func (tc *TestClient) Patch(path string, body any) (*TestResponse, error) {
	return tc.do(http.MethodPatch, path, body)
}

// Head sends a HEAD request to the given path.
func (tc *TestClient) Head(path string) (*TestResponse, error) {
	return tc.do(http.MethodHead, path, nil)
}

// Options sends an OPTIONS request to the given path.
func (tc *TestClient) Options(path string) (*TestResponse, error) {
	return tc.do(http.MethodOptions, path, nil)
}

func (tc *TestClient) do(method, path string, body any) (*TestResponse, error) {
	req := tc.Request(method, path)
	if body != nil {
		req.Body(body)
	}
	return req.Send()
}

// Request creates a new TestRequest builder for the given method and path.
func (tc *TestClient) Request(method, path string) *TestRequest {
	return &TestRequest{
		client:  tc,
		method:  method,
		path:    path,
		headers: make(map[string]string),
		cookies: make(map[string]string),
		query:   make(map[string]string),
	}
}

// TestRequest is a builder for constructing test HTTP requests.
type TestRequest struct {
	client      *TestClient
	method      string
	path        string
	headers     map[string]string
	cookies     map[string]string
	query       map[string]string
	body        any
	contentType string
}

// Header sets a request-level header.
func (tr *TestRequest) Header(key, value string) *TestRequest {
	tr.headers[key] = value
	return tr
}

// Cookie sets a request-level cookie.
func (tr *TestRequest) Cookie(name, value string) *TestRequest {
	tr.cookies[name] = value
	return tr
}

// Body sets the request body.
func (tr *TestRequest) Body(body any) *TestRequest {
	tr.body = body
	return tr
}

// Query sets a query parameter.
func (tr *TestRequest) Query(key, value string) *TestRequest {
	tr.query[key] = value
	return tr
}

// ContentType sets the Content-Type header for the request.
func (tr *TestRequest) ContentType(ct string) *TestRequest {
	tr.contentType = ct
	return tr
}

// Send executes the test request and returns the response.
func (tr *TestRequest) Send() (*TestResponse, error) {
	var bodyReader *bytes.Reader
	switch v := tr.body.(type) {
	case nil:
		bodyReader = bytes.NewReader(nil)
	case []byte:
		bodyReader = bytes.NewReader(v)
	case string:
		bodyReader = bytes.NewReader([]byte(v))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("kruda: test client: failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
		if tr.contentType == "" {
			tr.contentType = "application/json"
		}
	}

	targetURL := tr.path
	if len(tr.query) > 0 {
		vals := url.Values{}
		for k, v := range tr.query {
			vals.Set(k, v)
		}
		if strings.Contains(targetURL, "?") {
			targetURL += "&" + vals.Encode()
		} else {
			targetURL += "?" + vals.Encode()
		}
	}

	req, err := http.NewRequest(tr.method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("kruda: test client: failed to create request: %w", err)
	}

	// Snapshot and reset client-level headers/cookies under lock
	tr.client.mu.Lock()
	clientHeaders := make(map[string]string, len(tr.client.headers))
	for k, v := range tr.client.headers {
		clientHeaders[k] = v
	}
	clientCookies := make(map[string]string, len(tr.client.cookies))
	for k, v := range tr.client.cookies {
		clientCookies[k] = v
	}
	tr.client.headers = make(map[string]string)
	tr.client.cookies = make(map[string]string)
	tr.client.mu.Unlock()

	// Client-level headers first, request-level overrides
	for k, v := range clientHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range tr.headers {
		req.Header.Set(k, v)
	}

	for name, value := range clientCookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	for name, value := range tr.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	if tr.contentType != "" {
		req.Header.Set("Content-Type", tr.contentType)
	}

	w := httptest.NewRecorder()
	tr.client.app.ServeKruda(
		transport.NewNetHTTPResponseWriter(w),
		transport.NewNetHTTPRequestWithLimit(req, tr.client.app.config.BodyLimit),
	)

	return &TestResponse{recorder: w}, nil
}

// TestResponse wraps an httptest.ResponseRecorder for easy assertions.
type TestResponse struct {
	recorder *httptest.ResponseRecorder
}

// StatusCode returns the HTTP status code of the response.
func (tr *TestResponse) StatusCode() int {
	return tr.recorder.Code
}

// Header returns the value of the given response header.
func (tr *TestResponse) Header(key string) string {
	return tr.recorder.Header().Get(key)
}

// Body returns the response body as bytes.
func (tr *TestResponse) Body() []byte {
	return tr.recorder.Body.Bytes()
}

// BodyString returns the response body as a string.
func (tr *TestResponse) BodyString() string {
	return tr.recorder.Body.String()
}

// JSON unmarshals the response body into the given value.
func (tr *TestResponse) JSON(v any) error {
	return json.Unmarshal(tr.recorder.Body.Bytes(), v)
}
