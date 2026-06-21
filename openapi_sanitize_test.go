package kruda

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/internal/testtypes"
	"github.com/go-kruda/kruda/internal/testtypes/alt"
)

// crossPkgItemA is a ResourceService over testtypes.User.
type crossPkgItemA struct{}

func (crossPkgItemA) List(context.Context, int, int) ([]testtypes.User, int, error) {
	return nil, 0, nil
}
func (crossPkgItemA) Create(_ context.Context, item testtypes.User) (testtypes.User, error) {
	return item, nil
}
func (crossPkgItemA) Get(_ context.Context, id string) (testtypes.User, error) {
	return testtypes.User{ID: id}, nil
}
func (crossPkgItemA) Update(_ context.Context, _ string, item testtypes.User) (testtypes.User, error) {
	return item, nil
}
func (crossPkgItemA) Delete(context.Context, string) error { return nil }

// crossPkgItemB is a ResourceService over alt.User (same short name, different package).
type crossPkgItemB struct{}

func (crossPkgItemB) List(context.Context, int, int) ([]alt.User, int, error) { return nil, 0, nil }
func (crossPkgItemB) Create(_ context.Context, item alt.User) (alt.User, error) {
	return item, nil
}
func (crossPkgItemB) Get(_ context.Context, id string) (alt.User, error) {
	return alt.User{ID: id}, nil
}
func (crossPkgItemB) Update(_ context.Context, _ string, item alt.User) (alt.User, error) {
	return item, nil
}
func (crossPkgItemB) Delete(context.Context, string) error { return nil }

// TestGenerateSchema_CrossPackageGenericNoCollision verifies that two generic
// instantiations whose type arguments share a short name ("User") but live in
// different packages get DISTINCT component keys and DISTINCT $refs, and that
// both inner item schemas are present (neither silently dropped).
func TestGenerateSchema_CrossPackageGenericNoCollision(t *testing.T) {
	components := make(map[string]*schemaRef)

	refA := generateSchema(reflect.TypeOf(ResourceList[testtypes.User]{}), components)
	refB := generateSchema(reflect.TypeOf(ResourceList[alt.User]{}), components)

	if refA.Ref == refB.Ref {
		t.Fatalf("cross-package generics collapsed to same $ref %q", refA.Ref)
	}

	// Both envelope component keys must exist and be distinct.
	keyA := strings.TrimPrefix(refA.Ref, "#/components/schemas/")
	keyB := strings.TrimPrefix(refB.Ref, "#/components/schemas/")
	if keyA == keyB {
		t.Fatalf("envelope keys collided: %q", keyA)
	}
	if _, ok := components[keyA]; !ok {
		t.Errorf("envelope component %q missing", keyA)
	}
	if _, ok := components[keyB]; !ok {
		t.Errorf("envelope component %q missing", keyB)
	}

	// Both inner User schemas must be present (different package qualifiers ⇒
	// different keys), so neither is silently dropped.
	var userKeys []string
	for k, v := range components {
		if v.Type == "object" && v.Properties != nil {
			if _, hasID := v.Properties["id"]; hasID {
				// distinguish the two Users: testtypes.User has "name",
				// alt.User has "email".
				_, hasName := v.Properties["name"]
				_, hasEmail := v.Properties["email"]
				if hasName || hasEmail {
					userKeys = append(userKeys, k)
				}
			}
		}
	}
	var foundName, foundEmail bool
	for _, k := range userKeys {
		if _, ok := components[k].Properties["name"]; ok {
			foundName = true
		}
		if _, ok := components[k].Properties["email"]; ok {
			foundEmail = true
		}
	}
	if !foundName {
		t.Errorf("testtypes.User schema (with 'name') missing; keys=%v", componentKeys(components))
	}
	if !foundEmail {
		t.Errorf("alt.User schema (with 'email') missing; keys=%v", componentKeys(components))
	}

	// No component key may contain invalid JSON-pointer chars.
	for k := range components {
		if strings.ContainsAny(k, "/[].") {
			t.Errorf("component key %q contains invalid pointer chars", k)
		}
	}
}

// TestGenerateSchema_CrossPackageEndToEnd registers two Resources over
// same-short-name types in different packages and asserts the full spec keeps
// both ResourceList envelopes with distinct, valid $refs.
func TestGenerateSchema_CrossPackageEndToEnd(t *testing.T) {
	app := New()
	Resource[testtypes.User, string](app, "/a", crossPkgItemA{}, WithResourceOnly("LIST"))
	Resource[alt.User, string](app, "/b", crossPkgItemB{}, WithResourceOnly("LIST"))

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec: %v", err)
	}
	var spec any
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	assertValidJSONPointers(t, spec, "")
}

// TestSanitizeComponentName_NestedBrackets verifies a generic instantiated with
// a nested-bracket type argument (map/slice/nested generic) produces a key with
// no '[', ']', '/' or '.'.
func TestSanitizeComponentName_NestedBrackets(t *testing.T) {
	cases := []string{
		"ResourceList[map[string]int]",
		"ResourceList[[]int]",
		"Wrap[ResourceList[github.com/app/models.User]]",
		"genWrap[*github.com/app/models.User]",
	}
	for _, in := range cases {
		got := sanitizeComponentName(in)
		if strings.ContainsAny(got, "/[].*") {
			t.Errorf("sanitizeComponentName(%q) = %q still contains invalid chars", in, got)
		}
	}
}

// TestSanitizeComponentName_PointerVsValue verifies a value type arg and a
// pointer type arg with the same underlying name produce DISTINCT keys.
func TestSanitizeComponentName_PointerVsValue(t *testing.T) {
	val := sanitizeComponentName("genWrap[github.com/app/models.User]")
	ptr := sanitizeComponentName("genWrap[*github.com/app/models.User]")
	if val == ptr {
		t.Errorf("value and pointer type args collapsed to same key %q", val)
	}
}

// TestSanitizeComponentName_CrossPackage verifies same-short-name type args from
// different packages produce DISTINCT keys.
func TestSanitizeComponentName_CrossPackage(t *testing.T) {
	a := sanitizeComponentName("ResourceList[a/b.User]")
	b := sanitizeComponentName("ResourceList[c/d.User]")
	if a == b {
		t.Errorf("cross-package args collapsed to same key %q", a)
	}
}

func componentKeys(m map[string]*schemaRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
