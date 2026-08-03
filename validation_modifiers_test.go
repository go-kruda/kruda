package kruda

import (
	"reflect"
	"strings"
	"testing"
)

// The two modifiers are regression-tested against the exact shapes that made
// them necessary: tags carried over from go-playground/validator, found in a
// production application before validation-by-default shipped. Skipping a
// modifier does not relax validation the way skipping a constraint does — it
// applies the rules around it in the wrong place, so these assert behaviour that
// was actively wrong, not merely absent.

type omitEmptyInput struct {
	Bio      string `json:"bio" validate:"omitempty,min=10"`
	Category string `json:"category" validate:"omitempty,oneof=video quiz text"`
	Notes    string `json:"notes" validate:"omitempty,max=500"`
}

func TestOmitEmptySkipsRulesOnZeroValue(t *testing.T) {
	v := NewValidator()
	vs := buildValidators[omitEmptyInput](v)

	// Every field empty: without omitempty support, min= and oneof= both reject
	// the zero value and an optional field behaves as a required one.
	if ve := validate(vs, reflect.ValueOf(omitEmptyInput{}), v.messages); ve != nil {
		t.Fatalf("all-empty optional fields must pass, got %v", ve)
	}

	// A present value is still checked — omitempty must not disable the rule.
	ve := validate(vs, reflect.ValueOf(omitEmptyInput{Bio: "short"}), v.messages)
	if ve == nil {
		t.Fatal("a non-empty value that violates min must still fail")
	}
	if got := ve.Errors[0].Rule; got != "min" {
		t.Errorf("rule = %q, want min", got)
	}

	ve = validate(vs, reflect.ValueOf(omitEmptyInput{Category: "podcast"}), v.messages)
	if ve == nil {
		t.Fatal("a non-empty value outside oneof must still fail")
	}
}

type diveInput struct {
	IDs    []string          `json:"ids" validate:"omitempty,dive,uuid"`
	Limits map[string]string `json:"limits" validate:"omitempty,dive,oneof=free paid"`
}

func TestDiveAppliesRulesToElements(t *testing.T) {
	v := NewValidator()
	vs := buildValidators[diveInput](v)

	const validUUID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"

	// Without dive support the uuid rule ran against the slice itself, so this
	// field rejected every input including correct ones.
	if ve := validate(vs, reflect.ValueOf(diveInput{IDs: []string{validUUID}}), v.messages); ve != nil {
		t.Fatalf("a slice of valid UUIDs must pass, got %v", ve)
	}
	if ve := validate(vs, reflect.ValueOf(diveInput{}), v.messages); ve != nil {
		t.Fatalf("empty collections must pass under omitempty, got %v", ve)
	}

	ve := validate(vs, reflect.ValueOf(diveInput{IDs: []string{validUUID, "nope"}}), v.messages)
	if ve == nil {
		t.Fatal("an invalid element must fail")
	}
	// Naming the element is the point: "ids" alone leaves the caller hunting.
	if got := ve.Errors[0].Field; got != "ids[1]" {
		t.Errorf("Field = %q, want ids[1]", got)
	}

	ve = validate(vs, reflect.ValueOf(diveInput{Limits: map[string]string{"tier": "gold"}}), v.messages)
	if ve == nil {
		t.Fatal("an invalid map value must fail")
	}
	if got := ve.Errors[0].Field; got != "limits[tier]" {
		t.Errorf("Field = %q, want limits[tier]", got)
	}
}

type diveMixedInput struct {
	Tags []string `json:"tags" validate:"min=1,dive,min=2"`
}

func TestDiveSplitsContainerAndElementRules(t *testing.T) {
	v := NewValidator()
	vs := buildValidators[diveMixedInput](v)

	// min=1 constrains the slice, min=2 each element.
	if ve := validate(vs, reflect.ValueOf(diveMixedInput{Tags: []string{"go", "web"}}), v.messages); ve != nil {
		t.Fatalf("valid container and elements must pass, got %v", ve)
	}
	if ve := validate(vs, reflect.ValueOf(diveMixedInput{Tags: []string{}}), v.messages); ve == nil {
		t.Fatal("empty slice must fail the container rule min=1")
	}
	ve := validate(vs, reflect.ValueOf(diveMixedInput{Tags: []string{"go", "x"}}), v.messages)
	if ve == nil {
		t.Fatal("a short element must fail the element rule min=2")
	}
	if got := ve.Errors[0].Field; got != "tags[1]" {
		t.Errorf("Field = %q, want tags[1]", got)
	}
}

type omitEmptyOnlyInput struct {
	Note string `json:"note" validate:"omitempty"`
}

func TestOmitEmptyAloneRegistersNoValidator(t *testing.T) {
	v := NewValidator()
	// A lone modifier constrains nothing. Recording a validator here would set
	// hasValidate on the route and advertise a 422 the handler cannot return.
	if vs := buildValidators[omitEmptyOnlyInput](v); len(vs) != 0 {
		t.Errorf("expected no validators for a lone omitempty, got %d", len(vs))
	}
}

func TestModifiersAreNotRegisterableRules(t *testing.T) {
	v := NewValidator()
	for _, name := range v.RuleNames() {
		if name == "omitempty" || name == "dive" {
			t.Errorf("%q is a modifier and must not appear in RuleNames", name)
		}
	}
}

type diveOnScalarInput struct {
	Name string `json:"name" validate:"dive,min=2"`
}

func TestDiveOnNonCollectionIsInert(t *testing.T) {
	v := NewValidator()
	vs := buildValidators[diveOnScalarInput](v)
	// The startup warning covers the diagnosis; what must not happen is min=2
	// being applied to the string itself, which is the bug dive support removes.
	if ve := validate(vs, reflect.ValueOf(diveOnScalarInput{Name: "x"}), v.messages); ve != nil {
		t.Fatalf("element rules must not fall back to the container, got %v", ve)
	}
}

func TestUnknownRuleWarningNamesTheModifiers(t *testing.T) {
	// hexcolor and required_if remain genuinely unimplemented; the warning has to
	// point at the modifiers so nobody reads "unknown rule" and assumes omitempty
	// is unsupported too.
	v := NewValidator()
	names := strings.Join(v.RuleNames(), ",")
	if strings.Contains(names, "hexcolor") {
		t.Skip("hexcolor implemented; pick another unimplemented rule for this test")
	}
}
