package kruda

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/internal/testtypes"
)

// resourceListLike mirrors the shape of the (not-yet-defined) ResourceList[T]
// envelope so the sanitizer can be exercised against a real generic
// instantiation whose type argument lives in a sub-package — i.e. t.Name()
// contains "/" and ".".
type resourceListLike[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// assertValidJSONPointers walks the spec JSON and asserts every "$ref" string
// is a valid local JSON pointer: it must start with "#/" and contain none of
// the characters that break a component-name pointer segment.
func assertValidJSONPointers(t *testing.T, v any, path string) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			if k == "$ref" {
				ref, ok := child.(string)
				if !ok {
					t.Fatalf("%s/$ref is not a string: %T", path, child)
				}
				if !strings.HasPrefix(ref, "#/") {
					t.Errorf("%s: $ref %q does not start with #/", path, ref)
				}
				seg := strings.TrimPrefix(ref, "#/components/schemas/")
				if strings.ContainsAny(seg, "/[]") {
					t.Errorf("%s: $ref component segment %q contains invalid chars (/[]): %q", path, seg, ref)
				}
				continue
			}
			assertValidJSONPointers(t, child, path+"/"+k)
		}
	case []any:
		for i, child := range node {
			assertValidJSONPointers(t, child, path)
			_ = i
		}
	}
}

func TestGenerateSchema_GenericNameSanitized(t *testing.T) {
	components := make(map[string]*schemaRef)
	rt := reflect.TypeOf(resourceListLike[testtypes.User]{})

	// Sanity: the raw type name really does contain the chars that break $ref.
	if !strings.ContainsAny(rt.Name(), "/[].") {
		t.Fatalf("test precondition: expected raw name to contain /[]. got %q", rt.Name())
	}

	ref := generateSchema(rt, components)

	// (a) The emitted $ref must be a valid JSON pointer (no /, [, ]).
	if strings.ContainsAny(strings.TrimPrefix(ref.Ref, "#/components/schemas/"), "/[]") {
		t.Errorf("emitted $ref %q is not a valid JSON pointer", ref.Ref)
	}

	// (b) The component key derived from the generic must be sanitized to a
	// deterministic form that preserves the type argument's FULL qualified name
	// (every illegal-char run → "_"), so cross-package same-short-name args never
	// collide. The arg lives at github.com/go-kruda/kruda/internal/testtypes.User.
	wantKey := "resourceListLike_github_com_go-kruda_kruda_internal_testtypes_User"
	if _, ok := components[wantKey]; !ok {
		var keys []string
		for k := range components {
			keys = append(keys, k)
		}
		t.Fatalf("expected sanitized component key %q; got keys %v", wantKey, keys)
	}

	// (c) No component key in the map may contain /, [, or ] (would break $ref).
	for k := range components {
		if strings.ContainsAny(k, "/[]") {
			t.Errorf("component key %q contains invalid pointer chars", k)
		}
	}

	// (d) The $ref must actually point at the sanitized key.
	if ref.Ref != "#/components/schemas/"+wantKey {
		t.Errorf("$ref = %q, want %q", ref.Ref, "#/components/schemas/"+wantKey)
	}
}

func TestGenerateSchema_NonGenericKeysUnchanged(t *testing.T) {
	// The sanitizer must not alter keys for non-generic types (their Name()
	// has no /[].).
	components := make(map[string]*schemaRef)
	ref := generateSchema(reflect.TypeOf(oaSimple{}), components)
	if ref.Ref != "#/components/schemas/oaSimple" {
		t.Errorf("$ref = %q, want #/components/schemas/oaSimple", ref.Ref)
	}
	if _, ok := components["oaSimple"]; !ok {
		t.Error("non-generic key oaSimple must be unchanged")
	}
}

func TestBuildOperation_Resource_Create(t *testing.T) {
	components := make(map[string]*schemaRef)
	ri := routeInfo{
		method: "POST",
		path:   "/users",
		resourceOp: &resourceOp{
			bodyType:    reflect.TypeOf(oaValidated{}),
			respType:    reflect.TypeOf(oaValidated{}),
			successCode: "201",
			hasValidate: true,
		},
	}
	op := buildOperation(ri, components, false)

	if op.RequestBody == nil {
		t.Fatal("create op must have a request body")
	}
	if _, ok := op.RequestBody.Content["application/json"]; !ok {
		t.Error("create request body must be application/json")
	}
	if _, ok := op.Responses["201"]; !ok {
		t.Error("create must have a 201 response")
	}
	if op.Responses["201"].Content["application/json"] == nil {
		t.Error("201 response must carry a JSON body schema")
	}
	if _, ok := op.Responses["422"]; !ok {
		t.Error("create with hasValidate must have a 422 response")
	}
	if _, ok := op.Responses["default"]; !ok {
		t.Error("create must have a default error response")
	}
	if len(op.Parameters) != 0 {
		t.Errorf("create must have no path/query params, got %d", len(op.Parameters))
	}
}

func TestBuildOperation_Resource_Delete(t *testing.T) {
	components := make(map[string]*schemaRef)
	ri := routeInfo{
		method: "DELETE",
		path:   "/users/:id",
		resourceOp: &resourceOp{
			idParam:     "id",
			idType:      reflect.TypeOf(""),
			respType:    nil,
			successCode: "204",
			hasValidate: false,
		},
	}
	op := buildOperation(ri, components, false)

	if op.RequestBody != nil {
		t.Error("delete must not have a request body")
	}
	resp204, ok := op.Responses["204"]
	if !ok {
		t.Fatal("delete must have a 204 response")
	}
	if len(resp204.Content) != 0 {
		t.Errorf("204 response must have no body content, got %d", len(resp204.Content))
	}
	if _, ok := op.Responses["422"]; ok {
		t.Error("delete (no validate) must not have a 422 response")
	}
	// Path param.
	var foundID bool
	for _, p := range op.Parameters {
		if p.Name == "id" && p.In == "path" {
			foundID = true
			if p.Schema == nil || p.Schema.Type != "string" {
				t.Errorf("id path param schema = %+v, want string", p.Schema)
			}
		}
	}
	if !foundID {
		t.Error("delete must have an id path parameter")
	}
}

func TestBuildOperation_Resource_List(t *testing.T) {
	components := make(map[string]*schemaRef)
	ri := routeInfo{
		method: "GET",
		path:   "/users",
		resourceOp: &resourceOp{
			respType:       reflect.TypeOf(resourceListLike[oaSimple]{}),
			successCode:    "200",
			needsListQuery: true,
		},
	}
	op := buildOperation(ri, components, false)

	// page + limit integer query params.
	want := map[string]bool{"page": false, "limit": false}
	for _, p := range op.Parameters {
		if p.In != "query" {
			continue
		}
		if _, ok := want[p.Name]; ok {
			want[p.Name] = true
			if p.Schema == nil || p.Schema.Type != "integer" {
				t.Errorf("query param %q schema = %+v, want integer", p.Name, p.Schema)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("list op missing integer query param %q", name)
		}
	}
	if _, ok := op.Responses["200"]; !ok {
		t.Error("list must have a 200 response")
	}
	if op.Responses["200"].Content["application/json"] == nil {
		t.Error("list 200 response must carry a JSON body schema")
	}
	if _, ok := op.Responses["422"]; ok {
		t.Error("list (no validate) must not have a 422 response")
	}
}

func TestBuildOperation_Resource_Get(t *testing.T) {
	components := make(map[string]*schemaRef)
	ri := routeInfo{
		method: "GET",
		path:   "/users/:id",
		resourceOp: &resourceOp{
			idParam:     "id",
			idType:      reflect.TypeOf(int(0)),
			respType:    reflect.TypeOf(oaSimple{}),
			successCode: "200",
		},
	}
	op := buildOperation(ri, components, false)

	var idSchema *schemaRef
	for _, p := range op.Parameters {
		if p.Name == "id" && p.In == "path" {
			idSchema = p.Schema
		}
	}
	if idSchema == nil {
		t.Fatal("get must have an id path param")
	}
	if idSchema.Type != "integer" {
		t.Errorf("int id param type = %q, want integer", idSchema.Type)
	}
	if _, ok := op.Responses["200"]; !ok {
		t.Error("get must have a 200 response")
	}
	if op.RequestBody != nil {
		t.Error("get must not have a request body")
	}
}

// TestBuildOpenAPISpec_TypedOnly_GoldenGuard is the BLOCKING regression guard:
// a typed-only app's spec must be byte-identical before/after the resourceOp
// additions (resourceOp==nil path + sanitizer must not alter non-generic
// output). The golden bytes were captured before the openapi.go changes.
func TestBuildOpenAPISpec_TypedOnly_GoldenGuard(t *testing.T) {
	app := newGoldenGuardApp()

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}

	// Structural validity: every $ref is a valid JSON pointer.
	var spec any
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	assertValidJSONPointers(t, spec, "")

	// Byte-identity against the pre-change capture (JSON object key order is
	// stable for a given encoder + struct layout, so compare normalized JSON
	// to avoid map-ordering flakiness).
	var got, want any
	if err := json.Unmarshal(specJSON, &got); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal([]byte(goldenTypedOnlySpec), &want); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	gotN, _ := json.Marshal(got)
	wantN, _ := json.Marshal(want)
	if string(gotN) != string(wantN) {
		t.Errorf("typed-only spec changed.\n got: %s\nwant: %s", gotN, wantN)
	}
}

// newGoldenGuardApp builds a fixed typed-only app used for the golden guard.
func newGoldenGuardApp() *App {
	app := New(
		WithValidator(NewValidator()),
		WithOpenAPIInfo("Golden API", "9.9.9", "golden guard"),
		WithOpenAPITag("users", "User operations"),
	)
	Post[oaValidated, oaOut](app, "/users", func(c *C[oaValidated]) (*oaOut, error) {
		return &oaOut{ID: "1"}, nil
	}, WithDescription("Create user"), WithTags("users"))
	Get[oaParamQuery, oaOut](app, "/users/:id", func(c *C[oaParamQuery]) (*oaOut, error) {
		return &oaOut{ID: c.In.ID}, nil
	}, WithDescription("Get user"), WithTags("users"))
	return app
}

// goldenTypedOnlySpec is the buildOpenAPISpec() output of newGoldenGuardApp,
// captured from the code BEFORE the resourceOp additions + sanitizer. Compared
// as normalized JSON to guard the typed-route path against regression.
const goldenTypedOnlySpec = `{"openapi":"3.1.0","info":{"title":"Golden API","version":"9.9.9","description":"golden guard"},"paths":{"/users":{"post":{"description":"Create user","tags":["users"],"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/oaValidated"}}}},"responses":{"200":{"description":"Successful response","content":{"application/json":{"schema":{"$ref":"#/components/schemas/oaOut"}}}},"422":{"description":"Validation failed","content":{"application/json":{"schema":{"$ref":"#/components/schemas/ValidationError"}}}},"default":{"description":"Error response","content":{"application/json":{"schema":{"$ref":"#/components/schemas/KrudaError"}}}}}}},"/users/{id}":{"get":{"description":"Get user","tags":["users"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"page","in":"query","schema":{"type":"integer"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/oaParamQuery"}}}},"responses":{"200":{"description":"Successful response","content":{"application/json":{"schema":{"$ref":"#/components/schemas/oaOut"}}}},"default":{"description":"Error response","content":{"application/json":{"schema":{"$ref":"#/components/schemas/KrudaError"}}}}}}}},"components":{"schemas":{"FieldError":{"type":"object","properties":{"field":{"type":"string"},"message":{"type":"string"},"param":{"type":"string"},"rule":{"type":"string"},"value":{"type":"string"}},"required":["field","rule","param","message","value"]},"KrudaError":{"type":"object","properties":{"code":{"type":"integer"},"detail":{"type":"string"},"message":{"type":"string"}},"required":["code","message"]},"ValidationError":{"type":"object","properties":{"code":{"type":"integer"},"errors":{"type":"array","items":{"$ref":"#/components/schemas/FieldError"}},"message":{"type":"string"}},"required":["code","message","errors"]},"oaOut":{"type":"object","properties":{"id":{"type":"string"},"message":{"type":"string"}}},"oaParamQuery":{"type":"object","properties":{"name":{"type":"string"}}},"oaValidated":{"type":"object","properties":{"email":{"type":"string","format":"email"},"id":{"type":"string","format":"uuid"},"name":{"type":"string","minLength":2,"maxLength":50},"role":{"type":"string","enum":["admin","user","guest"]},"url":{"type":"string","format":"uri"}},"required":["name","email"]}}},"tags":[{"name":"users","description":"User operations"}]}`
