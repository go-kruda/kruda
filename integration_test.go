package kruda

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// ===========================================================================
// Test 1: TestIntegrationFullRequestLifecycle
// ===========================================================================

func TestIntegrationFullRequestLifecycle(t *testing.T) {
	app := New()

	// Middleware that sets a response header and a local value
	app.Use(func(c *Ctx) error {
		c.SetHeader("X-Request-Traced", "true")
		c.Set("traced", "true")
		return c.Next()
	})

	// GET /api/users/:id — returns JSON with id and traced value from locals
	app.Get("/api/users/:id", func(c *Ctx) error {
		traced, _ := c.Get("traced").(string)
		return c.JSON(Map{
			"id":     c.Param("id"),
			"traced": traced,
		})
	})

	// POST /api/users — binds JSON body and echoes back
	type createUserReq struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	app.Post("/api/users", func(c *Ctx) error {
		var req createUserReq
		if err := c.Bind(&req); err != nil {
			return BadRequest("invalid body")
		}
		return c.Status(201).JSON(Map{"name": req.Name, "email": req.Email})
	})

	app.Compile()
	tc := NewTestClient(app)

	// --- GET /api/users/42 ---
	t.Run("GET with param and middleware", func(t *testing.T) {
		resp, err := tc.Get("/api/users/42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode())
		}
		if resp.Header("X-Request-Traced") != "true" {
			t.Fatalf("expected X-Request-Traced header, got %q", resp.Header("X-Request-Traced"))
		}
		var body map[string]any
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body["id"] != "42" {
			t.Fatalf("expected id=42, got %v", body["id"])
		}
		if body["traced"] != "true" {
			t.Fatalf("expected traced=true, got %v", body["traced"])
		}
	})

	// --- POST /api/users with JSON body ---
	t.Run("POST with JSON body binding", func(t *testing.T) {
		resp, err := tc.Post("/api/users", map[string]string{
			"name":  "Alice",
			"email": "alice@example.com",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 201 {
			t.Fatalf("expected 201, got %d", resp.StatusCode())
		}
		var body map[string]any
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body["name"] != "Alice" {
			t.Fatalf("expected name=Alice, got %v", body["name"])
		}
		if body["email"] != "alice@example.com" {
			t.Fatalf("expected email=alice@example.com, got %v", body["email"])
		}
	})
}

// ===========================================================================
// Test 2: TestIntegrationDIResourceErrorMapping
// ===========================================================================

// Sentinel error for user not found
var errIntegrationUserNotFound = errors.New("user not found")

// integrationUser is the model for the resource test.
type integrationUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// inMemoryUserService implements ResourceService[integrationUser, string].
type inMemoryUserService struct {
	mu    sync.Mutex
	store map[string]integrationUser
	seq   int
}

func newInMemoryUserService() *inMemoryUserService {
	return &inMemoryUserService{store: make(map[string]integrationUser)}
}

func (s *inMemoryUserService) List(_ context.Context, page, limit int) ([]integrationUser, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	users := make([]integrationUser, 0, len(s.store))
	for _, u := range s.store {
		users = append(users, u)
	}
	return users, len(users), nil
}

func (s *inMemoryUserService) Create(_ context.Context, item integrationUser) (integrationUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	item.ID = strings.Repeat("0", 3-len(string(rune('0'+s.seq)))) + string(rune('0'+s.seq))
	// Simple sequential ID
	item.ID = "u" + strings.TrimLeft(item.ID, "0")
	s.store[item.ID] = item
	return item, nil
}

func (s *inMemoryUserService) Get(_ context.Context, id string) (integrationUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.store[id]
	if !ok {
		return integrationUser{}, errIntegrationUserNotFound
	}
	return u, nil
}

func (s *inMemoryUserService) Update(_ context.Context, id string, item integrationUser) (integrationUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[id]; !ok {
		return integrationUser{}, errIntegrationUserNotFound
	}
	item.ID = id
	s.store[id] = item
	return item, nil
}

func (s *inMemoryUserService) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[id]; !ok {
		return errIntegrationUserNotFound
	}
	delete(s.store, id)
	return nil
}

func TestIntegrationDIResourceErrorMapping(t *testing.T) {
	svc := newInMemoryUserService()

	c := NewContainer()
	if err := c.Give(svc); err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	app := New(WithContainer(c))
	app.MapError(errIntegrationUserNotFound, 404, "user not found")
	Resource[integrationUser, string](app, "/users", svc)
	app.Compile()
	tc := NewTestClient(app)

	// --- POST /users (create) ---
	var createdID string
	t.Run("Create user", func(t *testing.T) {
		resp, err := tc.Post("/users", map[string]string{"name": "Bob"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 201 {
			t.Fatalf("expected 201, got %d; body: %s", resp.StatusCode(), resp.BodyString())
		}
		var body integrationUser
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body.Name != "Bob" {
			t.Fatalf("expected name=Bob, got %v", body.Name)
		}
		if body.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		createdID = body.ID
	})

	// --- GET /users/:id (get) ---
	t.Run("Get user by ID", func(t *testing.T) {
		resp, err := tc.Get("/users/" + createdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 200 {
			t.Fatalf("expected 200, got %d; body: %s", resp.StatusCode(), resp.BodyString())
		}
		var body integrationUser
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body.ID != createdID {
			t.Fatalf("expected id=%s, got %s", createdID, body.ID)
		}
	})

	// --- GET /users (list) ---
	t.Run("List users", func(t *testing.T) {
		resp, err := tc.Get("/users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode())
		}
		var body map[string]any
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		total, _ := body["total"].(float64)
		if total < 1 {
			t.Fatalf("expected at least 1 user, got total=%v", total)
		}
	})

	// --- DELETE /users/:id ---
	t.Run("Delete user", func(t *testing.T) {
		resp, err := tc.Delete("/users/" + createdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 204 {
			t.Fatalf("expected 204, got %d; body: %s", resp.StatusCode(), resp.BodyString())
		}
	})

	// --- GET /users/:id after delete → 404 via error mapping ---
	t.Run("Get deleted user returns 404", func(t *testing.T) {
		resp, err := tc.Get("/users/" + createdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode() != 404 {
			t.Fatalf("expected 404, got %d; body: %s", resp.StatusCode(), resp.BodyString())
		}
		var body map[string]any
		if err := resp.JSON(&body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body["message"] != "user not found" {
			t.Fatalf("expected message='user not found', got %v", body["message"])
		}
	})
}

// ===========================================================================
// Test 3: TestIntegrationGracefulShutdown
// ===========================================================================

func TestIntegrationGracefulShutdown(t *testing.T) {
	hookCalled := false

	app := New()
	app.OnShutdown(func() {
		hookCalled = true
	})

	ctx := context.Background()
	_ = app.Shutdown(ctx)

	if !hookCalled {
		t.Fatal("expected OnShutdown hook to be called")
	}
}

// ===========================================================================
// Test 4: TestIntegrationMiddlewareChain
// ===========================================================================

func TestIntegrationMiddlewareChain(t *testing.T) {
	app := New()

	makeMiddleware := func(label string) HandlerFunc {
		return func(c *Ctx) error {
			existing, _ := c.Get("chain").(string)
			if existing != "" {
				existing += ","
			}
			c.Set("chain", existing+label)
			return c.Next()
		}
	}

	app.Use(makeMiddleware("A"))
	app.Use(makeMiddleware("B"))
	app.Use(makeMiddleware("C"))

	app.Get("/chain", func(c *Ctx) error {
		chain, _ := c.Get("chain").(string)
		return c.Text(chain)
	})

	app.Compile()
	tc := NewTestClient(app)

	resp, err := tc.Get("/chain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "A,B,C" {
		t.Fatalf("expected middleware chain 'A,B,C', got %q", resp.BodyString())
	}
}

// ===========================================================================
// Test 5: TestIntegrationTypedHandlerWithDI
// ===========================================================================

// greetingService is a simple DI service for the typed handler test.
type greetingService struct {
	prefix string
}

func (s *greetingService) Greet(name string) string {
	return s.prefix + ", " + name + "!"
}

// getGreetingInput is the typed input for the greeting handler.
type getGreetingInput struct {
	Name string `param:"name"`
}

// greetingOutput is the typed output for the greeting handler.
type greetingOutput struct {
	Greeting string `json:"greeting"`
}

func TestIntegrationTypedHandlerWithDI(t *testing.T) {
	svc := &greetingService{prefix: "Hello"}

	c := NewContainer()
	if err := c.Give(svc); err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	app := New(WithContainer(c))

	Get[getGreetingInput, greetingOutput](app, "/greet/:name", func(tc *C[getGreetingInput]) (*greetingOutput, error) {
		svc := MustResolve[*greetingService](tc.Ctx)
		return &greetingOutput{
			Greeting: svc.Greet(tc.In.Name),
		}, nil
	})

	app.Compile()
	client := NewTestClient(app)

	resp, err := client.Get("/greet/World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d; body: %s", resp.StatusCode(), resp.BodyString())
	}
	var body greetingOutput
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if body.Greeting != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", body.Greeting)
	}
}

// ===========================================================================
// Test 6: TestIntegrationHealthCheckWithServices
// ===========================================================================

// integrationHealthyService implements HealthChecker and always returns ok.
type integrationHealthyService struct{}

func (s *integrationHealthyService) Check(_ context.Context) error { return nil }

// integrationUnhealthyService implements HealthChecker and always returns an error.
type integrationUnhealthyService struct{}

func (s *integrationUnhealthyService) Check(_ context.Context) error {
	return errors.New("database connection lost")
}

func TestIntegrationHealthCheckWithServices(t *testing.T) {
	c := NewContainer()
	if err := c.Give(&integrationHealthyService{}); err != nil {
		t.Fatalf("failed to register healthy service: %v", err)
	}
	if err := c.Give(&integrationUnhealthyService{}); err != nil {
		t.Fatalf("failed to register unhealthy service: %v", err)
	}

	app := New(WithContainer(c))
	app.Get("/health", HealthHandler())
	app.Compile()
	tc := NewTestClient(app)

	resp, err := tc.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// One service is unhealthy → expect 503
	if resp.StatusCode() != 503 {
		t.Fatalf("expected 503, got %d; body: %s", resp.StatusCode(), resp.BodyString())
	}

	var body map[string]any
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected status=unhealthy, got %v", body["status"])
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("expected checks to be an object, got %T", body["checks"])
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d: %v", len(checks), checks)
	}

	// Verify we have one "ok" and one error
	foundOK := false
	foundErr := false
	for _, v := range checks {
		vs, _ := v.(string)
		if vs == "ok" {
			foundOK = true
		}
		if vs == "database connection lost" {
			foundErr = true
		}
	}
	if !foundOK {
		t.Fatal("expected at least one healthy check result")
	}
	if !foundErr {
		t.Fatal("expected at least one unhealthy check result with 'database connection lost'")
	}
}
