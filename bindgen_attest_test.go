package kruda

import (
	"reflect"
	"testing"
)

// mustBindGeneratedBinder resolves the attested fixture binder, failing the
// test when the fixture no longer attests. Parity tests must exercise the
// generated path; a silent fallback to the generic parser would pass
// vacuously.
func mustBindGeneratedBinder(t *testing.T) func(*Ctx) (reflect.Value, error) {
	t.Helper()
	binder := bindGeneratedBinderInputAttested()
	if binder == nil {
		t.Fatal("generated fixture binder declined attestation")
	}
	return binder
}

// TestGeneratedBinderAttested pins that every committed fixture currently
// attests. If a fixture edit breaks this, regenerate the fixture; if
// attestation itself broke, the shape table below says where.
func TestGeneratedBinderAttested(t *testing.T) {
	if bindGeneratedBinderInputAttested() == nil {
		t.Error("bindGeneratedBinderInput declined attestation")
	}
	if bindComposition5InputAttested() == nil {
		t.Error("bindComposition5Input declined attestation")
	}
	if bindComposition10InputAttested() == nil {
		t.Error("bindComposition10Input declined attestation")
	}
	if bindComposition30InputAttested() == nil {
		t.Error("bindComposition30Input declined attestation")
	}
}

type attestNamedID int64

func TestAttestBinderShape(t *testing.T) {
	base := bindGeneratedBinderInputShape
	if !AttestBinderShape[generatedBinderInput](base) {
		t.Fatal("current fixture shape does not attest")
	}
	mutate := func(f func(*BinderShape)) BinderShape {
		out := BinderShape{NumFields: base.NumFields, Fields: append([]BinderFieldShape(nil), base.Fields...)}
		f(&out)
		return out
	}
	for _, tt := range []struct {
		name  string
		shape BinderShape
	}{
		{"field count grows", mutate(func(s *BinderShape) { s.NumFields++ })},
		{"field count shrinks", mutate(func(s *BinderShape) { s.NumFields--; s.Fields = s.Fields[:len(s.Fields)-1] })},
		{"renamed field", mutate(func(s *BinderShape) { s.Fields[0].Name = "Identifier" })},
		{"reordered fields", mutate(func(s *BinderShape) { s.Fields[0], s.Fields[1] = s.Fields[1], s.Fields[0] })},
		{"changed kind", mutate(func(s *BinderShape) { s.Fields[0].Kind = "int32" })},
		{"changed query tag", mutate(func(s *BinderShape) { s.Fields[0].Query = "identifier" })},
		{"changed param tag", mutate(func(s *BinderShape) { s.Fields[0].Param = "identifier" })},
		{"changed default", mutate(func(s *BinderShape) { s.Fields[0].Default = "8" })},
		{"exported flips", mutate(func(s *BinderShape) { s.Fields[0].Exported = false })},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if AttestBinderShape[generatedBinderInput](tt.shape) {
				t.Fatal("stale shape attested")
			}
		})
	}
}

func TestAttestBinderShapeRejectsBodyTags(t *testing.T) {
	single := BinderShape{NumFields: 1, Fields: []BinderFieldShape{
		{Name: "ID", Exported: true, Kind: "int64", Query: "id"},
	}}
	if !AttestBinderShape[struct {
		ID int64 `query:"id"`
	}](single) {
		t.Fatal("plain shape does not attest")
	}
	if AttestBinderShape[struct {
		ID int64 `query:"id" json:"id"`
	}](single) {
		t.Fatal("json tag attested: binder would ignore a body the generic parser binds")
	}
	if AttestBinderShape[struct {
		ID int64 `query:"id" form:"id"`
	}](single) {
		t.Fatal("form tag attested: binder would ignore a form field the generic parser binds")
	}
	// Unexported fields are inert in both paths (skipped before tags are
	// read), so their tags must not affect attestation either way. Built
	// with StructOf because a literal would trip vet's structtag check.
	unexported := BinderShape{NumFields: 1, Fields: []BinderFieldShape{{Name: "hidden"}}}
	hiddenType := reflect.StructOf([]reflect.StructField{{
		Name:    "hidden",
		PkgPath: "example.com/hidden",
		Type:    reflect.TypeOf((*int)(nil)),
		Tag:     reflect.StructTag(`json:"hidden"`),
	}})
	if !attestBinderShapeType(hiddenType, unexported) {
		t.Fatal("unexported field with json tag does not attest")
	}
}

// TestNilBinderFallsBackToGeneric proves the fail-closed seam: an explicit
// nil binder (what a declined attestation produces) behaves byte-identically
// to the plain typed handler.
func TestNilBinderFallsBackToGeneric(t *testing.T) {
	handler := func(c *C[generatedBinderInput]) (*generatedBinderOutput, error) {
		return &generatedBinderOutput{ID: c.In.ID, Page: c.In.Page}, nil
	}
	run := func(withBinder bool) (int, string) {
		app := New()
		if withBinder {
			app.Get("/users/:id", buildTypedHandlerWithBinder[generatedBinderInput, generatedBinderOutput](app, "GET", "/users/:id", handler, nil, nil))
		} else {
			app.Get("/users/:id", buildTypedHandler[generatedBinderInput, generatedBinderOutput](app, "GET", "/users/:id", handler, nil))
		}
		app.Compile()
		w := newMockResponse()
		app.ServeKruda(w, &mockRequest{method: "GET", path: "/users/42", query: map[string]string{"page": "3"}})
		return w.statusCode, string(w.body)
	}
	for _, withBinder := range []bool{false, true} {
		status, body := run(withBinder)
		if status != 200 || body != `{"id":42,"page":3,"active":false,"score":0,"name":""}` {
			t.Fatalf("withBinder=%t: status=%d body=%s", withBinder, status, body)
		}
	}
}

func TestAttestBinderShapeNamedKinds(t *testing.T) {
	// Attestation compares reflect kinds, so a defined type over an
	// unchanged kind still attests. That staleness is closed one layer up:
	// the generated conversions name the original predeclared type and no
	// longer compile against the renamed field.
	single := BinderShape{NumFields: 1, Fields: []BinderFieldShape{
		{Name: "ID", Exported: true, Kind: "int64", Query: "id"},
	}}
	if !AttestBinderShape[struct {
		ID attestNamedID `query:"id"`
	}](single) {
		t.Fatal("defined kind over int64 does not attest")
	}
}
