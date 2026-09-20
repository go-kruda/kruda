package externalbind

import (
	"testing"

	"github.com/go-kruda/kruda"
)

// TestExternalBinderAttests proves the committed generated file engages
// cross-package: attestation runs against the public API and accepts the
// current input shape.
func TestExternalBinderAttests(t *testing.T) {
	if bindSearchInputAttested() == nil {
		t.Fatal("external binder declined attestation for its own fixture")
	}
	if !kruda.AttestBinderShape[searchInput](bindSearchInputShape) {
		t.Fatal("AttestBinderShape rejected the committed shape")
	}
}

// TestExternalShapeDeclinesStale proves fail-closed works cross-package: any
// drift between the struct and the committed shape declines instead of
// binding the old shape.
func TestExternalShapeDeclinesStale(t *testing.T) {
	stale := kruda.BinderShape{
		NumFields: bindSearchInputShape.NumFields,
		Fields:    append([]kruda.BinderFieldShape(nil), bindSearchInputShape.Fields...),
	}
	stale.Fields[0].Default = "none"
	if kruda.AttestBinderShape[searchInput](stale) {
		t.Fatal("stale external shape attested")
	}
}

// TestExternalBinderBindsDefaults runs the generated binder outside package
// kruda. A zero Ctx carries no request, so every field keeps its default;
// this exercises the emitted defaults, fallback, and conversion paths
// through the public API only. Query-carrying behavior is proven white-box
// in bindgen_scanner_test.go, where a real wingRequest can be constructed.
func TestExternalBinderBindsDefaults(t *testing.T) {
	binder := bindSearchInputAttested()
	if binder == nil {
		t.Fatal("external binder declined attestation for its own fixture")
	}
	v, err := binder(&kruda.Ctx{})
	if err != nil {
		t.Fatalf("binder error: %v", err)
	}
	got := v.Interface().(searchInput)
	if want := (searchInput{Q: "all", Page: 1}); got != want {
		t.Fatalf("binder defaults = %+v, want %+v", got, want)
	}
}
