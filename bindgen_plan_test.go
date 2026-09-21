package kruda

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestWithGeneratedPlanServesRoute wires a generated plan through the public
// registration path: binding, validation, and error mapping all behave.
func TestWithGeneratedPlanServesRoute(t *testing.T) {
	app := New()
	Get[generatedBinderInput, generatedBinderOutput](app, "/users/:id", func(c *C[generatedBinderInput]) (*generatedBinderOutput, error) {
		return &generatedBinderOutput{ID: c.In.ID, Page: c.In.Page, Active: c.In.Active, Score: c.In.Score, Name: c.In.Name}, nil
	}, WithGeneratedPlan(generatedBinderInputPlan()))
	client := NewTestClient(app)

	resp, err := client.Get("/users/42?page=3&active=false&score=2.5&name=ada")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode(), resp.BodyString())
	}
	var out generatedBinderOutput
	if err := resp.JSON(&out); err != nil {
		t.Fatalf("decode error: %v (%s)", err, resp.BodyString())
	}
	want := generatedBinderOutput{ID: 42, Page: 3, Active: false, Score: 2.5, Name: "ada"}
	if out != want {
		t.Fatalf("output = %+v, want %+v", out, want)
	}

	resp, err = client.Get("/users/42?page=0")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 422 {
		t.Fatalf("rule violation status = %d, want 422 (%s)", resp.StatusCode(), resp.BodyString())
	}
}

// TestWithGeneratedPlanStaleShapeFallsBack proves a stale plan degrades to
// the generic parser with identical bytes: the shape drift is silent because
// the route stays correct.
func TestWithGeneratedPlanStaleShapeFallsBack(t *testing.T) {
	handler := func(c *C[generatedBinderInput]) (*generatedBinderOutput, error) {
		return &generatedBinderOutput{ID: c.In.ID, Page: c.In.Page}, nil
	}
	run := func(plan *GeneratedPlan[generatedBinderInput]) (int, string) {
		app := New()
		if plan == nil {
			Get[generatedBinderInput, generatedBinderOutput](app, "/users/:id", handler)
		} else {
			Get[generatedBinderInput, generatedBinderOutput](app, "/users/:id", handler, WithGeneratedPlan(*plan))
		}
		resp, err := NewTestClient(app).Get("/users/42?page=3")
		if err != nil {
			t.Fatalf("request error: %v", err)
		}
		return resp.StatusCode(), resp.BodyString()
	}

	stale := generatedBinderInputPlan()
	// Deep-copy: plan.Shape.Fields shares its backing array with the
	// committed shape, and mutating that would poison other tests.
	fields := append([]BinderFieldShape(nil), stale.Shape.Fields...)
	fields[0].Default = "changed"
	stale.Shape.Fields = fields
	stale.Binder = bindGeneratedBinderInput // bypass the attested (nil) wrapper: the route must still decline it

	wantStatus, wantBody := run(nil)
	gotStatus, gotBody := run(&stale)
	if gotStatus != wantStatus || gotBody != wantBody {
		t.Fatalf("stale plan = %d %q, want generic %d %q", gotStatus, gotBody, wantStatus, wantBody)
	}
}

// TestWithGeneratedPlanCrossedTypeWarnsAndFallsBack wires a plan generated
// for one input to a route for another. The plan cannot match, so the route
// keeps the generic parser and warns once instead of binding the wrong type.
func TestWithGeneratedPlanCrossedTypeWarnsAndFallsBack(t *testing.T) {
	type other struct {
		Q string `query:"q" default:"none"`
	}
	type otherOut struct {
		Q string `json:"q"`
	}

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)

	app := New()
	plan := generatedBinderInputPlan()
	Get[other, otherOut](app, "/crossed", func(c *C[other]) (*otherOut, error) {
		return &otherOut{Q: c.In.Q}, nil
	}, WithGeneratedPlan(plan))
	// Registering again must not warn again.
	app2 := New()
	Get[other, otherOut](app2, "/crossed", func(c *C[other]) (*otherOut, error) {
		return &otherOut{Q: c.In.Q}, nil
	}, WithGeneratedPlan(plan))

	resp, err := NewTestClient(app).Get("/crossed?q=kept")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode(), resp.BodyString())
	}
	var out otherOut
	if err := resp.JSON(&out); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if out.Q != "kept" {
		t.Fatalf("generic binding lost: %+v", out)
	}
	if n := strings.Count(logs.String(), "generated plan for a different input type"); n != 1 {
		t.Fatalf("expected exactly one crossed-plan warning, got %d:\n%s", n, logs.String())
	}
}

// TestDescribeValidatorsMarksCustomOverrides proves the descriptor carries
// the exact signal the generated factory needs: a custom rule override nils
// the compiled checks, and the factory declines rather than trusting
// predicates built for the built-in meaning.
func TestDescribeValidatorsMarksCustomOverrides(t *testing.T) {
	v := NewValidator()
	v.Register("min", func(value any, param string) bool { return true })
	descriptors := describeValidators(buildValidators[generatedBinderInput](v))
	if len(descriptors) == 0 {
		t.Fatal("no descriptors built")
	}
	for _, d := range descriptors {
		if d.NumChecks != 0 {
			t.Fatalf("descriptor %+v kept compiled checks despite a custom override", d)
		}
	}
	if f := generatedBinderInputPlan().ValidatorFactory(descriptors); f != nil {
		t.Fatal("validator factory engaged despite a custom override")
	}

	plain := describeValidators(buildValidators[generatedBinderInput](NewValidator()))
	if f := generatedBinderInputPlan().ValidatorFactory(plain); f == nil {
		t.Fatal("validator factory declined unmodified descriptors")
	}
}
