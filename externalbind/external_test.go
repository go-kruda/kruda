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

// TestExternalPlanEngages proves the plan constructor bundles a live binder
// with a live validator factory, built only from public API.
func TestExternalPlanEngages(t *testing.T) {
	plan := searchInputPlan()
	if plan.Binder == nil {
		t.Fatal("plan binder declined attestation for its own fixture")
	}
	if plan.ValidatorFactory == nil {
		t.Fatal("plan has no validator factory")
	}
	descriptors := []kruda.ValidatorDescriptor{
		{Index: 0, Rules: []kruda.ValidatorRule{{Name: "min", Param: "1"}, {Name: "max", Param: "64"}}, NumChecks: 2},
		{Index: 1, Rules: []kruda.ValidatorRule{{Name: "min", Param: "1"}, {Name: "max", Param: "1000"}}, NumChecks: 2},
	}
	valid := plan.ValidatorFactory(descriptors)
	if valid == nil {
		t.Fatal("validator factory declined its own descriptors")
	}
	if !valid(&searchInput{Q: "go", Page: 2}) {
		t.Error("valid input rejected by the compiled predicate")
	}
	if valid(&searchInput{Q: "go", Page: 0}) {
		t.Error("invalid input accepted by the compiled predicate")
	}
	// A custom override nils the compiled checks; the factory must decline
	// and let generic validation run instead of trusting stale predicates.
	overridden := []kruda.ValidatorDescriptor{
		{Index: 0, Rules: []kruda.ValidatorRule{{Name: "min", Param: "1"}, {Name: "max", Param: "64"}}},
		{Index: 1, Rules: []kruda.ValidatorRule{{Name: "min", Param: "1"}, {Name: "max", Param: "1000"}}},
	}
	if plan.ValidatorFactory(overridden) != nil {
		t.Error("validator factory engaged despite zero compiled checks")
	}
}

// TestExternalGeneratedRoute runs the whole flow through public API only:
// generated binding plus compiled validation on a real route. Valid input
// binds and validates, a rule violation is a 422, and an unparseable value
// is a 400 from the binder.
func TestExternalGeneratedRoute(t *testing.T) {
	app := kruda.New()
	kruda.Get[searchInput, searchOutput](app, "/search", func(c *kruda.C[searchInput]) (*searchOutput, error) {
		return &searchOutput{Q: c.In.Q, Page: c.In.Page}, nil
	}, kruda.WithGeneratedPlan(searchInputPlan()))
	client := kruda.NewTestClient(app)

	resp, err := client.Get("/search?q=kruda&page=3")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("valid input status = %d, want 200 (%s)", resp.StatusCode(), resp.BodyString())
	}
	var out searchOutput
	if err := resp.JSON(&out); err != nil {
		t.Fatalf("decode error: %v (%s)", err, resp.BodyString())
	}
	if out != (searchOutput{Q: "kruda", Page: 3}) {
		t.Fatalf("bound output = %+v, want {kruda 3}", out)
	}

	resp, err = client.Get("/search?q=kruda&page=0")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 422 {
		t.Fatalf("rule violation status = %d, want 422 (%s)", resp.StatusCode(), resp.BodyString())
	}

	resp, err = client.Get("/search?q=kruda&page=abc")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("unparseable value status = %d, want 400 (%s)", resp.StatusCode(), resp.BodyString())
	}
}
