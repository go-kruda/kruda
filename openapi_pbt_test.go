package kruda

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: phase2b-extensions, Property 8: OpenAPI Type Mapping and Property Names
//
// For any Go struct type with exported fields having json tags, generateSchema
// should produce a JSON Schema where each json-tagged field appears as a property
// with the tag name, and Go types map correctly to JSON Schema types.

func TestPropertyOpenAPITypeMapping(t *testing.T) {
	// We test with a fixed struct but verify the type mapping is correct
	// for all primitive types.
	components := make(map[string]*schemaRef)

	type allTypes struct {
		S   string  `json:"s"`
		I   int     `json:"i"`
		I8  int8    `json:"i8"`
		I16 int16   `json:"i16"`
		I32 int32   `json:"i32"`
		I64 int64   `json:"i64"`
		U   uint    `json:"u"`
		U8  uint8   `json:"u8"`
		U16 uint16  `json:"u16"`
		U32 uint32  `json:"u32"`
		U64 uint64  `json:"u64"`
		F32 float32 `json:"f32"`
		F64 float64 `json:"f64"`
		B   bool    `json:"b"`
	}

	generateSchema(reflect.TypeOf(allTypes{}), components)
	s, ok := components["allTypes"]
	if !ok {
		t.Fatal("allTypes not in components")
	}

	expected := map[string]string{
		"s": "string", "i": "integer", "i8": "integer", "i16": "integer",
		"i32": "integer", "i64": "integer", "u": "integer", "u8": "integer",
		"u16": "integer", "u32": "integer", "u64": "integer",
		"f32": "number", "f64": "number", "b": "boolean",
	}

	for prop, wantType := range expected {
		ps, ok := s.Properties[prop]
		if !ok {
			t.Errorf("missing property %q", prop)
			continue
		}
		if ps.Type != wantType {
			t.Errorf("property %q: type = %q, want %q", prop, ps.Type, wantType)
		}
	}

	// Verify all json-tagged fields appear as properties
	if len(s.Properties) != len(expected) {
		t.Errorf("properties count = %d, want %d", len(s.Properties), len(expected))
	}
}

// Feature: phase2b-extensions, Property 9: OpenAPI Validation Tag to Schema Constraint Mapping

func TestPropertyOpenAPIValidationConstraints(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	// Test min constraint for integers
	t.Run("MinInteger", func(t *testing.T) {
		f := func(minVal uint8) bool {
			// Build a struct type dynamically isn't easy, so we test applyValidationConstraints directly
			s := &schemaRef{Type: "integer"}
			vtag := "min=" + strings.TrimSpace(string(rune('0'+minVal%10)))
			applyValidationConstraints(s, vtag, reflect.TypeOf(0))
			return s.Minimum != nil
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	// Test that required fields end up in required array
	t.Run("Required", func(t *testing.T) {
		type reqStruct struct {
			Name string `json:"name" validate:"required"`
		}
		components := make(map[string]*schemaRef)
		generateSchema(reflect.TypeOf(reqStruct{}), components)
		s := components["reqStruct"]
		if s == nil {
			t.Fatal("reqStruct not in components")
		}
		found := false
		for _, r := range s.Required {
			if r == "name" {
				found = true
			}
		}
		if !found {
			t.Error("name should be in required array")
		}
	})

	// Test oneof → enum
	t.Run("OneOf", func(t *testing.T) {
		s := &schemaRef{Type: "string"}
		applyValidationConstraints(s, "oneof=a b c", reflect.TypeOf(""))
		if len(s.Enum) != 3 || s.Enum[0] != "a" || s.Enum[1] != "b" || s.Enum[2] != "c" {
			t.Errorf("enum = %v, want [a b c]", s.Enum)
		}
	})

	// Test format mappings
	t.Run("Formats", func(t *testing.T) {
		formats := map[string]string{
			"email": "email",
			"url":   "uri",
			"uuid":  "uuid",
		}
		for rule, wantFormat := range formats {
			s := &schemaRef{Type: "string"}
			applyValidationConstraints(s, rule, reflect.TypeOf(""))
			if s.Format != wantFormat {
				t.Errorf("rule %q → format %q, want %q", rule, s.Format, wantFormat)
			}
		}
	})

	// Test min/max for strings → minLength/maxLength
	t.Run("StringMinMax", func(t *testing.T) {
		s := &schemaRef{Type: "string"}
		applyValidationConstraints(s, "min=3,max=100", reflect.TypeOf(""))
		if s.MinLength == nil || *s.MinLength != 3 {
			t.Errorf("minLength = %v, want 3", s.MinLength)
		}
		if s.MaxLength == nil || *s.MaxLength != 100 {
			t.Errorf("maxLength = %v, want 100", s.MaxLength)
		}
	})
}

// Feature: phase2b-extensions, Property 10: OpenAPI Nested Struct $ref Generation

func TestPropertyOpenAPINestedRef(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Child Inner `json:"child"`
	}

	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(Outer{}), components)

	// Outer should be in components
	outerSchema, ok := components["Outer"]
	if !ok {
		t.Fatal("Outer not in components")
	}

	// child property should be a $ref
	childProp := outerSchema.Properties["child"]
	if childProp == nil {
		t.Fatal("missing child property")
	}
	if childProp.Ref != "#/components/schemas/Inner" {
		t.Errorf("child ref = %q, want #/components/schemas/Inner", childProp.Ref)
	}

	// Inner should also be in components
	if _, ok := components["Inner"]; !ok {
		t.Error("Inner should be in components")
	}
}

// Feature: phase2b-extensions, Property 11: OpenAPI Pointer Nullable

func TestPropertyOpenAPIPointerNullable(t *testing.T) {
	type WithPtr struct {
		Name    string  `json:"name"`
		OptName *string `json:"opt_name"`
		OptAge  *int    `json:"opt_age"`
	}

	components := make(map[string]*schemaRef)
	generateSchema(reflect.TypeOf(WithPtr{}), components)

	s, ok := components["WithPtr"]
	if !ok {
		t.Fatal("WithPtr not in components")
	}

	// Non-pointer field should NOT be nullable
	nameProp := s.Properties["name"]
	if nameProp.Nullable {
		t.Error("non-pointer name should not be nullable")
	}

	// Pointer fields should be nullable
	optNameProp := s.Properties["opt_name"]
	if !optNameProp.Nullable {
		t.Error("*string opt_name should be nullable")
	}

	optAgeProp := s.Properties["opt_age"]
	if !optAgeProp.Nullable {
		t.Error("*int opt_age should be nullable")
	}
}

// Feature: phase2b-extensions, Property 12: OpenAPI Path Conversion
//
// For any Kruda path string containing :param segments, convertPath should
// replace each :param with {param}. Applying convertPath twice should be idempotent.

func TestPropertyOpenAPIPathConversion(t *testing.T) {
	cfg := &quick.Config{MaxCount: 100}

	f := func(segments uint8) bool {
		// Build a path with random number of segments
		n := int(segments%5) + 1
		parts := make([]string, n)
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				parts[i] = "seg"
			} else {
				parts[i] = ":param" + string(rune('a'+i))
			}
		}
		path := "/" + strings.Join(parts, "/")

		converted := convertPath(path)

		// No colons should remain
		if strings.Contains(converted, ":") {
			return false
		}

		// Idempotent: converting again should produce same result
		if convertPath(converted) != converted {
			return false
		}

		// Each :param should become {param}
		for i := 0; i < n; i++ {
			if i%2 == 1 {
				expected := "{param" + string(rune('a'+i)) + "}"
				if !strings.Contains(converted, expected) {
					return false
				}
			}
		}

		return true
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// Feature: phase2b-extensions, Property 13: OpenAPI Spec Contains All Registered Routes

func TestPropertyOpenAPIAllRoutes(t *testing.T) {
	type emptyIn struct{}
	type emptyOut struct{}

	app := New(WithOpenAPIInfo("Test", "1.0.0", ""))

	// Register multiple routes with different methods
	Get[emptyIn, emptyOut](app, "/users", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})
	Post[emptyIn, emptyOut](app, "/users", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})
	Get[emptyIn, emptyOut](app, "/users/:id", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})
	Put[emptyIn, emptyOut](app, "/users/:id", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})
	Delete[emptyIn, emptyOut](app, "/users/:id", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})
	Patch[emptyIn, emptyOut](app, "/items/:id", func(c *C[emptyIn]) (*emptyOut, error) {
		return nil, nil
	})

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	var spec openAPISpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("invalid spec JSON: %v", err)
	}

	// Check /users has GET and POST
	usersPath, ok := spec.Paths["/users"]
	if !ok {
		t.Fatal("missing /users path")
	}
	if usersPath.Get == nil {
		t.Error("/users should have GET")
	}
	if usersPath.Post == nil {
		t.Error("/users should have POST")
	}

	// Check /users/{id} has GET, PUT, DELETE
	usersIdPath, ok := spec.Paths["/users/{id}"]
	if !ok {
		t.Fatal("missing /users/{id} path")
	}
	if usersIdPath.Get == nil {
		t.Error("/users/{id} should have GET")
	}
	if usersIdPath.Put == nil {
		t.Error("/users/{id} should have PUT")
	}
	if usersIdPath.Delete == nil {
		t.Error("/users/{id} should have DELETE")
	}

	// Check /items/{id} has PATCH
	itemsIdPath, ok := spec.Paths["/items/{id}"]
	if !ok {
		t.Fatal("missing /items/{id} path")
	}
	if itemsIdPath.Patch == nil {
		t.Error("/items/{id} should have PATCH")
	}
}

// Feature: phase2b-extensions, Property 14: OpenAPI Response Schema Generation

func TestPropertyOpenAPIResponseSchema(t *testing.T) {
	type respOut struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	app := New(
		WithValidator(NewValidator()),
		WithOpenAPIInfo("Test", "1.0.0", ""),
	)

	type valIn struct {
		Name string `json:"name" validate:"required"`
	}

	// Handler with validation → should have 200 + 422 + default
	Post[valIn, respOut](app, "/items", func(c *C[valIn]) (*respOut, error) {
		return &respOut{ID: "1", Name: c.In.Name}, nil
	})

	// Handler without validation → should have 200 + default (no 422)
	Get[emptyOut, respOut](app, "/items", func(c *C[emptyOut]) (*respOut, error) {
		return &respOut{ID: "1"}, nil
	})

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	var spec openAPISpec
	json.Unmarshal(specJSON, &spec)

	itemsPath := spec.Paths["/items"]

	// POST should have 422
	if itemsPath.Post == nil {
		t.Fatal("missing POST /items")
	}
	if _, ok := itemsPath.Post.Responses["422"]; !ok {
		t.Error("POST with validation should have 422 response")
	}
	if _, ok := itemsPath.Post.Responses["200"]; !ok {
		t.Error("POST should have 200 response")
	}

	// GET should NOT have 422
	if itemsPath.Get == nil {
		t.Fatal("missing GET /items")
	}
	if _, ok := itemsPath.Get.Responses["422"]; ok {
		t.Error("GET without validation should NOT have 422 response")
	}
	if _, ok := itemsPath.Get.Responses["200"]; !ok {
		t.Error("GET should have 200 response")
	}
	if _, ok := itemsPath.Get.Responses["default"]; !ok {
		t.Error("GET should have default response")
	}

	// Check 200 response has schema with correct properties
	resp200 := itemsPath.Post.Responses["200"]
	if resp200.Content == nil {
		t.Fatal("200 response should have content")
	}
	jsonContent := resp200.Content["application/json"]
	if jsonContent == nil {
		t.Fatal("200 response should have application/json content")
	}
	if jsonContent.Schema == nil {
		t.Fatal("200 response should have schema")
	}
}
