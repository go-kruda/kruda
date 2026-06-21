package kruda

import (
	"encoding/json"
	"testing"
)

// decodeSpec builds and unmarshals the OpenAPI spec for assertions.
func decodeSpec(t *testing.T, app *App) map[string]any {
	t.Helper()
	b, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return spec
}

func paths(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	p, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec has no paths object")
	}
	return p
}

func op(t *testing.T, spec map[string]any, path, method string) map[string]any {
	t.Helper()
	item, ok := paths(t, spec)[path].(map[string]any)
	if !ok {
		t.Fatalf("no path item for %q; paths=%v", path, keysOf(paths(t, spec)))
	}
	o, ok := item[method].(map[string]any)
	if !ok {
		t.Fatalf("no %s operation for %q", method, path)
	}
	return o
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestOpenAPI_Resource_AllOpsPresent(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""), WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	spec := decodeSpec(t, app)

	// All five ops at the two paths.
	op(t, spec, "/users", "get")         // list
	op(t, spec, "/users", "post")        // create
	op(t, spec, "/users/{id}", "get")    // get-by-id
	op(t, spec, "/users/{id}", "put")    // update
	op(t, spec, "/users/{id}", "delete") // delete
}

func TestOpenAPI_Resource_ListQueryParams(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(app, "/users", &mockUserService{})
	spec := decodeSpec(t, app)

	list := op(t, spec, "/users", "get")
	params, _ := list["parameters"].([]any)
	want := map[string]bool{"page": false, "limit": false}
	for _, p := range params {
		pm := p.(map[string]any)
		if pm["in"] != "query" {
			continue
		}
		name, _ := pm["name"].(string)
		if _, ok := want[name]; ok {
			want[name] = true
			sch, _ := pm["schema"].(map[string]any)
			if sch["type"] != "integer" {
				t.Errorf("query %q type = %v, want integer", name, sch["type"])
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("list op missing integer query param %q", name)
		}
	}
}

func TestOpenAPI_Resource_PathParamType(t *testing.T) {
	// string ID → string path param.
	appS := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(appS, "/users", &mockUserService{})
	specS := decodeSpec(t, appS)
	assertPathParamType(t, op(t, specS, "/users/{id}", "get"), "id", "string")

	// int ID → integer path param.
	appI := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(appI, "/items", &mockItemService{})
	specI := decodeSpec(t, appI)
	assertPathParamType(t, op(t, specI, "/items/{id}", "get"), "id", "integer")
}

func assertPathParamType(t *testing.T, o map[string]any, name, wantType string) {
	t.Helper()
	params, _ := o["parameters"].([]any)
	for _, p := range params {
		pm := p.(map[string]any)
		if pm["in"] == "path" && pm["name"] == name {
			sch, _ := pm["schema"].(map[string]any)
			if sch["type"] != wantType {
				t.Errorf("path param %q type = %v, want %v", name, sch["type"], wantType)
			}
			return
		}
	}
	t.Errorf("path param %q not found", name)
}

func TestOpenAPI_Resource_RequestBodyRefAndCodes(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""), WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	spec := decodeSpec(t, app)

	// create request body present with a $ref + 201.
	create := op(t, spec, "/users", "post")
	if create["requestBody"] == nil {
		t.Error("create must have a request body")
	}
	resps := create["responses"].(map[string]any)
	if _, ok := resps["201"]; !ok {
		t.Error("create must have 201")
	}
	if _, ok := resps["422"]; !ok {
		t.Error("create with validator+tags must have 422")
	}

	// update → 200 + 422.
	update := op(t, spec, "/users/{id}", "put")
	uresps := update["responses"].(map[string]any)
	if _, ok := uresps["200"]; !ok {
		t.Error("update must have 200")
	}
	if _, ok := uresps["422"]; !ok {
		t.Error("update with validator+tags must have 422")
	}

	// delete → 204, no body, no 422.
	del := op(t, spec, "/users/{id}", "delete")
	dresps := del["responses"].(map[string]any)
	r204, ok := dresps["204"].(map[string]any)
	if !ok {
		t.Fatal("delete must have 204")
	}
	if r204["content"] != nil {
		t.Error("204 must have no content")
	}
	if _, ok := dresps["422"]; ok {
		t.Error("delete must not have 422")
	}
}

func TestOpenAPI_Resource_No422WithoutValidator(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", "")) // no validator
	Resource(app, "/users", &validatedUserService{})
	spec := decodeSpec(t, app)

	create := op(t, spec, "/users", "post")
	resps := create["responses"].(map[string]any)
	if _, ok := resps["422"]; ok {
		t.Error("no validator configured → create must not advertise 422")
	}
}

func TestOpenAPI_Resource_No422WithoutTags(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""), WithValidator(NewValidator()))
	Resource(app, "/users", &mockUserService{}) // mockUser has no validate tags
	spec := decodeSpec(t, app)

	create := op(t, spec, "/users", "post")
	resps := create["responses"].(map[string]any)
	if _, ok := resps["422"]; ok {
		t.Error("T without validate tags → create must not advertise 422")
	}
}

func TestOpenAPI_Resource_ValidJSONPointers(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""), WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	b, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec: %v", err)
	}
	var spec any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertValidJSONPointers(t, spec, "")
}

func TestOpenAPI_Resource_ListEnvelopeComponentSanitized(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(app, "/users", &mockUserService{})
	spec := decodeSpec(t, app)

	comps, _ := spec["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)
	// ResourceList[github.com/go-kruda/kruda.mockUser] → sanitized to a key that
	// preserves the type arg's full package path (every illegal-char run → "_").
	if _, ok := schemas["ResourceList_github_com_go-kruda_kruda_mockUser"]; !ok {
		t.Errorf("missing sanitized ResourceList component; schemas=%v", keysOf(schemas))
	}
	for k := range schemas {
		if containsAnyChar(k, "/[]") {
			t.Errorf("component key %q has invalid pointer chars", k)
		}
	}
}

func containsAnyChar(s, chars string) bool {
	for _, c := range chars {
		for _, x := range s {
			if x == c {
				return true
			}
		}
	}
	return false
}

// =============================================================================
// only/except gating mirrors routeInfo append (§5.5c)
// =============================================================================

func TestOpenAPI_Resource_OnlyGET_BothListAndGetByID(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(app, "/users", &mockUserService{}, WithResourceOnly("GET"))
	spec := decodeSpec(t, app)

	op(t, spec, "/users", "get")      // list
	op(t, spec, "/users/{id}", "get") // get-by-id

	// No create/update/delete.
	if item, ok := paths(t, spec)["/users"].(map[string]any); ok {
		if _, ok := item["post"]; ok {
			t.Error("WithResourceOnly(GET) must not register POST in spec")
		}
	}
	if item, ok := paths(t, spec)["/users/{id}"].(map[string]any); ok {
		if _, ok := item["put"]; ok {
			t.Error("WithResourceOnly(GET) must not register PUT in spec")
		}
		if _, ok := item["delete"]; ok {
			t.Error("WithResourceOnly(GET) must not register DELETE in spec")
		}
	}
}

// WithResourceExcept("DELETE") must remove the delete op from the spec, and the
// spec must mirror the router exactly (the §5.5c invariant). NOTE: the spec's §7
// prose claims Except("GET") removes both list+get-by-id, but that contradicts
// the runtime resourceShouldRegister logic §5.5c mandates mirroring: excepting
// the "GET" shortcut still registers the LIST/GET_BY_ID sub-keys at the router,
// so OpenAPI registers them too (router/OpenAPI parity is the actual contract).
// This test pins the parity-respecting behavior with Except("DELETE").
func TestOpenAPI_Resource_ExceptDelete_RemovesDelete(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""))
	Resource(app, "/users", &mockUserService{}, WithResourceExcept("DELETE"))
	spec := decodeSpec(t, app)

	// Survivors present.
	op(t, spec, "/users", "get")
	op(t, spec, "/users", "post")
	op(t, spec, "/users/{id}", "get")
	op(t, spec, "/users/{id}", "put")

	// Delete removed.
	if item, ok := paths(t, spec)["/users/{id}"].(map[string]any); ok {
		if _, ok := item["delete"]; ok {
			t.Error("WithResourceExcept(DELETE) must not register delete in spec")
		}
	}
}

// =============================================================================
// Group prefix → OpenAPI path == convertPath(joinPath(prefix, idPath)) (§5.1)
// =============================================================================

func TestOpenAPI_GroupResource_PrefixedPaths(t *testing.T) {
	app := New(WithOpenAPIInfo("API", "1.0", ""))
	g := app.Group("/api/v1")
	GroupResource(g, "/users", &mockUserService{})
	spec := decodeSpec(t, app)

	op(t, spec, "/api/v1/users", "get")      // list
	op(t, spec, "/api/v1/users/{id}", "get") // get-by-id
	op(t, spec, "/api/v1/users", "post")     // create
	op(t, spec, "/api/v1/users/{id}", "put") // update
	op(t, spec, "/api/v1/users/{id}", "delete")

	// Sanity: matches convertPath(joinPath(...)).
	wantID := convertPath(joinPath("/api/v1", "/users/:id"))
	if _, ok := paths(t, spec)[wantID].(map[string]any); !ok {
		t.Errorf("expected path %q in spec; paths=%v", wantID, keysOf(paths(t, spec)))
	}
}
