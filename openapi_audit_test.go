package kruda

import (
	"reflect"
	"strings"
	"testing"

	m1models "github.com/go-kruda/kruda/internal/testtypes/m1/models"
	m2models "github.com/go-kruda/kruda/internal/testtypes/m2/models"
	m3models "github.com/go-kruda/kruda/internal/testtypes/m3/models"
)

// TestGenerateSchema_ThreeWayCrossPackageCollision guards component-key
// disambiguation for 3+ plain named types that share a short name AND a
// trailing package segment (e.g. several ".../models.User"). The earlier
// one-shot "_<lastSegment>" suffix overwrote the second schema with the third
// and mis-pointed $refs; the full-path + counter scheme must keep all distinct.
func TestGenerateSchema_ThreeWayCrossPackageCollision(t *testing.T) {
	components := make(map[string]*schemaRef)

	r1 := generateSchema(reflect.TypeOf(m1models.User{}), components)
	r2 := generateSchema(reflect.TypeOf(m2models.User{}), components)
	r3 := generateSchema(reflect.TypeOf(m3models.User{}), components)

	k1 := strings.TrimPrefix(r1.Ref, "#/components/schemas/")
	k2 := strings.TrimPrefix(r2.Ref, "#/components/schemas/")
	k3 := strings.TrimPrefix(r3.Ref, "#/components/schemas/")

	if k1 == k2 || k1 == k3 || k2 == k3 {
		t.Fatalf("three same-short-name/same-last-segment types collided: %q %q %q", k1, k2, k3)
	}
	for _, k := range []string{k1, k2, k3} {
		if _, ok := components[k]; !ok {
			t.Fatalf("component %q missing (schema silently dropped); keys=%v", k, componentKeys(components))
		}
	}

	// Each distinguishing field (a/b/c) must survive — none overwritten.
	want := map[string]string{"a": k1, "b": k2, "c": k3}
	for field, key := range want {
		if _, ok := components[key].Properties[field]; !ok {
			t.Errorf("schema %q lost its unique field %q (overwritten); props=%v", key, field, components[key].Properties)
		}
	}

	// Every key must remain a valid JSON-pointer segment.
	for k := range components {
		if strings.ContainsAny(k, "/[].") {
			t.Errorf("component key %q contains invalid pointer chars", k)
		}
	}
}

func keySafeComponentName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isKeySafeRune(r) {
			return false
		}
	}
	return true
}

// TestSanitizeComponentName_MultiArgGenericDropsComma verifies a multi-type-arg
// generic name (reflect uses "," as the arg separator) yields a key with no
// comma and matching the OpenAPI component-name charset.
func TestSanitizeComponentName_MultiArgGenericDropsComma(t *testing.T) {
	got := sanitizeComponentName("Pair[github.com/app/models.Foo,github.com/app/models.Bar]")
	if strings.ContainsRune(got, ',') {
		t.Fatalf("key %q still contains a comma (invalid OpenAPI component name)", got)
	}
	if !keySafeComponentName(got) {
		t.Fatalf("key %q is not a valid OpenAPI component name", got)
	}
}

// TestSanitizeComponentName_PreservesHyphen verifies a legitimate hyphen
// (module paths like "go-kruda") survives sanitization — it is legal in an
// OpenAPI component key and must not be collapsed.
func TestSanitizeComponentName_PreservesHyphen(t *testing.T) {
	got := sanitizeComponentName("ResourceList[github.com/go-kruda/kruda/internal/testtypes.User]")
	if !strings.Contains(got, "go-kruda") {
		t.Fatalf("hyphen not preserved in %q (modules like go-kruda must keep '-')", got)
	}
	if !keySafeComponentName(got) {
		t.Fatalf("key %q is not a valid OpenAPI component name", got)
	}
}
