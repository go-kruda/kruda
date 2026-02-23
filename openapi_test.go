package kruda

import (
	"encoding/json"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Test types for OpenAPI schema generation
// ---------------------------------------------------------------------------

type oaSimple struct {
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Score  float64 `json:"score"`
	Active bool    `json:"active"`
}

type oaValidated struct {
	Name  string `json:"name" validate:"required,min=2,max=50"`
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"oneof=admin user guest"`
	URL   string `json:"url" validate:"url"`
	ID    string `json:"id" validate:"uuid"`
}

type oaNested struct {
	Title  string   `json:"title"`
	Author oaSimple `json:"author"`
}

type oaPointer struct {
	Name    string    `json:"name"`
	Address *oaSimple `json:"address"`
}

type oaSlice struct {
	Tags  []string   `json:"tags"`
	Items []oaSimple `json:"items"`
}

type oaParamQuery struct {
	ID   string `param:"id"`
	Page int    `query:"page"`
	Name string `json:"name"`
}

type oaOut struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Task 11.12: Schema generation tests
// ---------------------------------------------------------------------------

func TestGenerateSchema_Primitives(t *testing.T) {
	components := make(map[string]*schemaRef)
	schema := generateSchema(reflect.TypeOf(oaSimple{}), components)

	// Should return a $ref since it's a named struct
	if schema.Ref == "" {
		t.Fatal("expected $ref for named struct")
	}

	// Check the actual schema in components
	s, ok := components["oaSimple"]
	if !ok {
		t.Fatal("oaSimple not in components")
	}
	if s.Type != "object" {
		t.Errorf("type = %q, want object", s.Type)
	}

	tests := map[string]string{
		"name":   "string",
		"age":    "integer",
		"score":  "number",
		"active": "boolean",
	}
	for prop, wantType := range tests {
		ps, ok := s.Properties[prop]
		if !ok {
			t.Errorf("missing property %q", prop)
			continue
		}
		if ps.Type != wantType {
			t.Errorf("property %q type = %q, want %q", prop, ps.Type, wantType)
		}
	}
}

func TestGenerateSchema_ValidationConstraints(t *testing.T) {
	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(oaValidated{}), components)

	s, ok := components["oaValidated"]
	if !ok {
		t.Fatal("oaValidated not in components")
	}

	// Check required array
	requiredMap := make(map[string]bool)
	for _, r := range s.Required {
		requiredMap[r] = true
	}
	if !requiredMap["name"] {
		t.Error("name should be required")
	}
	if !requiredMap["email"] {
		t.Error("email should be required")
	}

	// Check min/max on name
	nameSchema := s.Properties["name"]
	if nameSchema.MinLength == nil || *nameSchema.MinLength != 2 {
		t.Errorf("name minLength = %v, want 2", nameSchema.MinLength)
	}
	if nameSchema.MaxLength == nil || *nameSchema.MaxLength != 50 {
		t.Errorf("name maxLength = %v, want 50", nameSchema.MaxLength)
	}

	// Check email format
	emailSchema := s.Properties["email"]
	if emailSchema.Format != "email" {
		t.Errorf("email format = %q, want email", emailSchema.Format)
	}

	// Check oneof → enum
	roleSchema := s.Properties["role"]
	if len(roleSchema.Enum) != 3 {
		t.Errorf("role enum len = %d, want 3", len(roleSchema.Enum))
	}

	// Check url format
	urlSchema := s.Properties["url"]
	if urlSchema.Format != "uri" {
		t.Errorf("url format = %q, want uri", urlSchema.Format)
	}

	// Check uuid format
	idSchema := s.Properties["id"]
	if idSchema.Format != "uuid" {
		t.Errorf("id format = %q, want uuid", idSchema.Format)
	}
}

func TestGenerateSchema_NestedStruct(t *testing.T) {
	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(oaNested{}), components)

	s, ok := components["oaNested"]
	if !ok {
		t.Fatal("oaNested not in components")
	}

	authorProp := s.Properties["author"]
	if authorProp == nil {
		t.Fatal("missing author property")
	}
	if authorProp.Ref != "#/components/schemas/oaSimple" {
		t.Errorf("author ref = %q, want #/components/schemas/oaSimple", authorProp.Ref)
	}

	// oaSimple should also be in components
	if _, ok := components["oaSimple"]; !ok {
		t.Error("oaSimple should be in components from nested ref")
	}
}

func TestGenerateSchema_Pointer_Nullable(t *testing.T) {
	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(oaPointer{}), components)

	s, ok := components["oaPointer"]
	if !ok {
		t.Fatal("oaPointer not in components")
	}

	addrProp := s.Properties["address"]
	if addrProp == nil {
		t.Fatal("missing address property")
	}
	// Pointer fields should be nullable
	if !addrProp.Nullable {
		t.Error("pointer field should have nullable=true")
	}
}

func TestGenerateSchema_Slice(t *testing.T) {
	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(oaSlice{}), components)

	s, ok := components["oaSlice"]
	if !ok {
		t.Fatal("oaSlice not in components")
	}

	tagsProp := s.Properties["tags"]
	if tagsProp == nil {
		t.Fatal("missing tags property")
	}
	if tagsProp.Type != "array" {
		t.Errorf("tags type = %q, want array", tagsProp.Type)
	}
	if tagsProp.Items == nil || tagsProp.Items.Type != "string" {
		t.Error("tags items should be string")
	}

	itemsProp := s.Properties["items"]
	if itemsProp == nil {
		t.Fatal("missing items property")
	}
	if itemsProp.Type != "array" {
		t.Errorf("items type = %q, want array", itemsProp.Type)
	}
}

func TestGenerateSchema_PrimitiveTypes(t *testing.T) {
	components := make(map[string]*schemaRef)

	tests := []struct {
		typ  reflect.Type
		want string
	}{
		{reflect.TypeOf(""), "string"},
		{reflect.TypeOf(0), "integer"},
		{reflect.TypeOf(int8(0)), "integer"},
		{reflect.TypeOf(int16(0)), "integer"},
		{reflect.TypeOf(int32(0)), "integer"},
		{reflect.TypeOf(int64(0)), "integer"},
		{reflect.TypeOf(uint(0)), "integer"},
		{reflect.TypeOf(uint8(0)), "integer"},
		{reflect.TypeOf(uint16(0)), "integer"},
		{reflect.TypeOf(uint32(0)), "integer"},
		{reflect.TypeOf(uint64(0)), "integer"},
		{reflect.TypeOf(float32(0)), "number"},
		{reflect.TypeOf(float64(0)), "number"},
		{reflect.TypeOf(true), "boolean"},
	}

	for _, tt := range tests {
		s := generateSchema(tt.typ, components)
		if s.Type != tt.want {
			t.Errorf("type %v → %q, want %q", tt.typ, s.Type, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Path conversion tests
// ---------------------------------------------------------------------------

func TestConvertPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users/:id", "/users/{id}"},
		{"/users/:id/posts/:postId", "/users/{id}/posts/{postId}"},
		{"/static/path", "/static/path"},
		{"/:a/:b/:c", "/{a}/{b}/{c}"},
		{"/", "/"},
		{"/users/{id}", "/users/{id}"}, // already converted — idempotent
		// Regex constraints should be stripped
		{"/users/:id<[0-9]+>", "/users/{id}"},
		{"/posts/:slug<[a-z-]+>/comments/:cid<[0-9]+>", "/posts/{slug}/comments/{cid}"},
		// Optional params should be stripped
		{"/users/:id?", "/users/{id}"},
		// Both regex and optional
		{"/items/:id<[0-9]+>?", "/items/{id}"},
	}

	for _, tt := range tests {
		got := convertPath(tt.input)
		if got != tt.want {
			t.Errorf("convertPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Spec builder tests
// ---------------------------------------------------------------------------

func TestBuildOpenAPISpec_Basic(t *testing.T) {
	app := New(
		WithOpenAPIInfo("Test API", "1.0.0", "A test API"),
		WithOpenAPITag("users", "User operations"),
	)

	Post[oaParamQuery, oaOut](app, "/users", func(c *C[oaParamQuery]) (*oaOut, error) {
		return &oaOut{ID: "1"}, nil
	}, WithDescription("Create user"), WithTags("users"))

	Get[oaParamQuery, oaOut](app, "/users/:id", func(c *C[oaParamQuery]) (*oaOut, error) {
		return &oaOut{ID: c.In.ID}, nil
	}, WithDescription("Get user"), WithTags("users"))

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check openapi version
	if spec["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", spec["openapi"])
	}

	// Check info
	info := spec["info"].(map[string]any)
	if info["title"] != "Test API" {
		t.Errorf("title = %v, want Test API", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", info["version"])
	}

	// Check paths exist
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/users"]; !ok {
		t.Error("missing /users path")
	}
	if _, ok := paths["/users/{id}"]; !ok {
		t.Error("missing /users/{id} path")
	}

	// Check POST /users has description and tags
	usersPath := paths["/users"].(map[string]any)
	postOp := usersPath["post"].(map[string]any)
	if postOp["description"] != "Create user" {
		t.Errorf("POST description = %v, want Create user", postOp["description"])
	}

	// Check GET /users/{id} has path parameter
	usersIdPath := paths["/users/{id}"].(map[string]any)
	getOp := usersIdPath["get"].(map[string]any)
	params := getOp["parameters"].([]any)
	foundPathParam := false
	for _, p := range params {
		pm := p.(map[string]any)
		if pm["name"] == "id" && pm["in"] == "path" {
			foundPathParam = true
		}
	}
	if !foundPathParam {
		t.Error("GET /users/{id} should have path parameter 'id'")
	}

	// Check tags section
	tags := spec["tags"].([]any)
	if len(tags) == 0 {
		t.Error("expected tags section")
	}
}

func TestBuildOpenAPISpec_WithValidation_Has422(t *testing.T) {
	app := New(
		WithValidator(NewValidator()),
		WithOpenAPIInfo("Test", "1.0.0", ""),
	)

	type valIn struct {
		Name string `json:"name" validate:"required"`
	}

	Post[valIn, oaOut](app, "/items", func(c *C[valIn]) (*oaOut, error) {
		return &oaOut{ID: "1"}, nil
	})

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	var spec map[string]any
	json.Unmarshal(specJSON, &spec)

	paths := spec["paths"].(map[string]any)
	itemsPath := paths["/items"].(map[string]any)
	postOp := itemsPath["post"].(map[string]any)
	responses := postOp["responses"].(map[string]any)

	if _, ok := responses["422"]; !ok {
		t.Error("expected 422 response for validated handler")
	}
	if _, ok := responses["200"]; !ok {
		t.Error("expected 200 response")
	}
	if _, ok := responses["default"]; !ok {
		t.Error("expected default response")
	}
}

func TestBuildOpenAPISpec_NoConfig_Empty(t *testing.T) {
	app := New() // no OpenAPI config

	// Register a route but don't configure OpenAPI
	Get[oaSimple, oaOut](app, "/test", func(c *C[oaSimple]) (*oaOut, error) {
		return nil, nil
	})

	// buildOpenAPISpec should still work (returns valid JSON)
	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	var spec map[string]any
	json.Unmarshal(specJSON, &spec)

	// Info should have zero values
	info := spec["info"].(map[string]any)
	if info["title"] != "" {
		t.Errorf("title should be empty without config, got %v", info["title"])
	}
}

// ---------------------------------------------------------------------------
// Config option tests
// ---------------------------------------------------------------------------

func TestWithOpenAPIInfo(t *testing.T) {
	app := New(WithOpenAPIInfo("My API", "2.0.0", "Description"))

	if app.config.openAPIInfo.Title != "My API" {
		t.Errorf("title = %q, want My API", app.config.openAPIInfo.Title)
	}
	if app.config.openAPIInfo.Version != "2.0.0" {
		t.Errorf("version = %q, want 2.0.0", app.config.openAPIInfo.Version)
	}
	if app.config.openAPIInfo.Description != "Description" {
		t.Errorf("description = %q, want Description", app.config.openAPIInfo.Description)
	}
	if app.config.openAPIPath != "/openapi.json" {
		t.Errorf("path = %q, want /openapi.json", app.config.openAPIPath)
	}
}

func TestWithOpenAPIPath(t *testing.T) {
	app := New(
		WithOpenAPIInfo("API", "1.0.0", ""),
		WithOpenAPIPath("/api/docs.json"),
	)

	if app.config.openAPIPath != "/api/docs.json" {
		t.Errorf("path = %q, want /api/docs.json", app.config.openAPIPath)
	}
}

func TestWithOpenAPITag(t *testing.T) {
	app := New(
		WithOpenAPITag("users", "User operations"),
		WithOpenAPITag("posts", "Post operations"),
	)

	if len(app.config.openAPITags) != 2 {
		t.Fatalf("tags len = %d, want 2", len(app.config.openAPITags))
	}
	if app.config.openAPITags[0].Name != "users" {
		t.Errorf("tag[0] name = %q, want users", app.config.openAPITags[0].Name)
	}
	if app.config.openAPITags[1].Name != "posts" {
		t.Errorf("tag[1] name = %q, want posts", app.config.openAPITags[1].Name)
	}
}

// ---------------------------------------------------------------------------
// buildOperation tests
// ---------------------------------------------------------------------------

func TestBuildOperation_QueryParams(t *testing.T) {
	type queryIn struct {
		Page int    `query:"page"`
		Sort string `query:"sort"`
	}

	ri := routeInfo{
		method: "GET",
		path:   "/items",
		config: routeConfig{
			inType:  reflect.TypeOf(queryIn{}),
			outType: reflect.TypeOf(oaOut{}),
		},
	}

	components := make(map[string]*schemaRef)
	op := buildOperation(ri, components)

	queryParams := 0
	for _, p := range op.Parameters {
		if p.In == "query" {
			queryParams++
		}
	}
	if queryParams != 2 {
		t.Errorf("query params = %d, want 2", queryParams)
	}
}

func TestBuildOperation_RequestBody(t *testing.T) {
	ri := routeInfo{
		method:  "POST",
		path:    "/items",
		hasBody: true,
		config: routeConfig{
			inType:  reflect.TypeOf(oaSimple{}),
			outType: reflect.TypeOf(oaOut{}),
		},
	}

	components := make(map[string]*schemaRef)
	op := buildOperation(ri, components)

	if op.RequestBody == nil {
		t.Fatal("expected request body for POST with hasBody")
	}
	if _, ok := op.RequestBody.Content["application/json"]; !ok {
		t.Error("expected application/json content type")
	}
}

func TestBuildOperation_FormBody(t *testing.T) {
	ri := routeInfo{
		method:  "POST",
		path:    "/upload",
		hasForm: true,
		config: routeConfig{
			inType:  reflect.TypeOf(oaSimple{}),
			outType: reflect.TypeOf(oaOut{}),
		},
	}

	components := make(map[string]*schemaRef)
	op := buildOperation(ri, components)

	if op.RequestBody == nil {
		t.Fatal("expected request body for form upload")
	}
	if _, ok := op.RequestBody.Content["multipart/form-data"]; !ok {
		t.Error("expected multipart/form-data content type")
	}
}

// ---------------------------------------------------------------------------
// containsRule tests
// ---------------------------------------------------------------------------

func TestContainsRule(t *testing.T) {
	tests := []struct {
		vtag string
		rule string
		want bool
	}{
		{"required,min=2,email", "required", true},
		{"required,min=2,email", "email", true},
		{"required,min=2,email", "min", true},
		{"required,min=2,email", "max", false},
		{"", "required", false},
	}

	for _, tt := range tests {
		got := containsRule(tt.vtag, tt.rule)
		if got != tt.want {
			t.Errorf("containsRule(%q, %q) = %v, want %v", tt.vtag, tt.rule, got, tt.want)
		}
	}
}
