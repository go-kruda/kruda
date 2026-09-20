package kruda

import "reflect"

// BinderFieldShape records one struct field as a generated binder saw it.
// BinderShape is the full record; AttestBinderShape fails closed when the
// source struct no longer matches, so a stale generated binder declines and
// the route silently keeps the generic parser instead of binding the old
// shape. Experimental API for generated binders; its shape may change
// before any release.
type BinderFieldShape struct {
	Name     string
	Exported bool
	// Kind is the normalized reflect kind ("uint8" for byte, "int32" for
	// rune, the predeclared name otherwise).
	Kind         string
	Query, Param string
	Default      string
}

// BinderShape describes the input struct a generated binder was built for:
// every field in struct order, including unexported ones the binder skips.
// Experimental API for generated binders; its shape may change before any
// release.
type BinderShape struct {
	NumFields int
	Fields    []BinderFieldShape
}

// AttestBinderShape reports whether T still matches the shape a generated
// binder was built for. It checks field count, per-index names, exportedness,
// kinds, and the query/param/default tags the binder baked in, and rejects
// any json/form tag the generator never supported. Any mismatch — added,
// removed, reordered, query/param/default-retagged, or kind-changing retyped
// field — returns false and the caller must fall back to the generic parser.
//
// Two intentional blind spots stay safe one layer up: a same-kind named
// retype still attests but breaks the generated conversions at compile time,
// and a validate-only retag still attests but is caught by the validator
// factory's own rule attestation with boxed fallback.
//
// Experimental API for generated binders; its shape may change before any
// release.
func AttestBinderShape[T any](want BinderShape) bool {
	return attestBinderShapeType(reflect.TypeOf((*T)(nil)).Elem(), want)
}

// ValidatorRule is one parsed validation rule in a ValidatorDescriptor:
// the rule name and its parameter exactly as buildValidators recorded them.
type ValidatorRule struct {
	Name  string
	Param string
}

// ValidatorDescriptor is the exported mirror of the fields a generated
// validator factory attests: struct index, the omitempty modifier, the
// container rules in order, the count of compiled unboxed checks, and the
// count of post-dive element rules. It carries no funcs, so generated code
// in any package can attest against it; describeValidators builds it from
// the registered validators once per route.
//
// Experimental API for generated binders; its shape may change before any
// release.
type ValidatorDescriptor struct {
	Index        int
	OmitEmpty    bool
	Rules        []ValidatorRule
	NumChecks    int
	NumElemRules int
}

// describeValidators projects compiled validators onto their exported
// descriptors. NumChecks is what rejects custom overrides and uncompilable
// chains: buildValidators nils the checks for those, so a descriptor whose
// NumChecks differs from the generated rule count declines the factory.
func describeValidators(in []fieldValidator) []ValidatorDescriptor {
	if len(in) == 0 {
		return nil
	}
	out := make([]ValidatorDescriptor, len(in))
	for i, fv := range in {
		d := ValidatorDescriptor{
			Index:        fv.index,
			OmitEmpty:    fv.omitEmpty,
			NumChecks:    len(fv.checks),
			NumElemRules: len(fv.elemRules),
		}
		if len(fv.rules) > 0 {
			d.Rules = make([]ValidatorRule, len(fv.rules))
			for j, r := range fv.rules {
				d.Rules[j] = ValidatorRule{Name: r.name, Param: r.param}
			}
		}
		out[i] = d
	}
	return out
}

// GeneratedPlan bundles one generated binder with its optional compiled
// validator factory. The generator emits a Plan constructor per input; pass
// it to a typed route with WithGeneratedPlan. Every field fails closed:
// a nil Binder keeps the generic parser, a nil factory (or one that
// declines) keeps generic validation, and a Shape that no longer matches
// the route input drops the binder silently so stale generated code can
// never bind an old shape.
//
// Experimental API for generated binders; its shape may change before any
// release.
type GeneratedPlan[In any] struct {
	Shape            BinderShape
	Binder           func(*Ctx) (reflect.Value, error)
	ValidatorFactory func([]ValidatorDescriptor) func(*In) bool
}

func attestBinderShapeType(t reflect.Type, want BinderShape) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	if t.NumField() != want.NumFields || len(want.Fields) != want.NumFields {
		return false
	}
	for i := range want.Fields {
		f := t.Field(i)
		w := want.Fields[i]
		if f.Name != w.Name || f.IsExported() != w.Exported {
			return false
		}
		if !f.IsExported() {
			// Unexported fields are inert in both paths: the generic
			// parser skips them before looking at tags, and so does
			// the generator. Only the name slot matters.
			continue
		}
		if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
			return false
		}
		if f.Tag.Get("form") != "" {
			return false
		}
		if f.Type.Kind().String() != w.Kind {
			return false
		}
		if f.Tag.Get("query") != w.Query || f.Tag.Get("param") != w.Param ||
			f.Tag.Get("default") != w.Default {
			return false
		}
	}
	return true
}
