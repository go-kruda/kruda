package kruda

import (
	"math"
	"reflect"
	"testing"
)

// TestCompiledMinMaxParity is the drift guard for the compileMinMax mirror
// contract: every (value, param) where compilation succeeds must agree with
// the boxed originals. Extend the value/param tables when min/max semantics
// change; a narrowing edit here must justify why the dropped case can no
// longer occur.
func TestCompiledMinMaxParity(t *testing.T) {
	type signed int64
	type unsigned uint64
	type number float64
	type text string
	values := []any{
		int(0), int8(-128), int8(127), int16(-32768), int16(32767),
		int32(math.MinInt32), int32(math.MaxInt32), int64(math.MinInt64), int64(math.MaxInt64),
		int64(9007199254740993), int64(-9007199254740993), signed(math.MaxInt64),
		uint(0), uint8(255), uint16(65535), uint32(math.MaxUint32), uint64(math.MaxUint64),
		uint64(9007199254740993), unsigned(math.MaxUint64),
		float32(-1.5), float32(0), float32(math.MaxFloat32), float64(-1.5), float64(0),
		math.SmallestNonzeroFloat64, math.MaxFloat64, math.NaN(), math.Inf(-1), math.Inf(1), number(1.5),
		"", "a", "ก", "é", text("ก"), []int(nil), []int{1, 2}, [2]int{}, map[string]int(nil), map[string]int{"a": 1},
		true, uintptr(1), (*int)(nil), struct{}{},
	}
	params := []string{
		"", "invalid", " 1", "1 ", "0", "-0", "1", "+1", "-1", "1.5", "-1.5", "1e1", "0x1.8p+1", "1_000",
		"9007199254740992", "9007199254740993", "-9007199254740993", "9223372036854775807", "9223372036854775808",
		"-9223372036854775808", "-9223372036854775809", "18446744073709551615", "18446744073709551616",
		"1e999", "-1e999", "NaN", "Inf", "+Inf", "-Inf",
	}
	for _, name := range []string{"min", "max"} {
		original := builtinRules()[name]
		for _, value := range values {
			v := reflect.ValueOf(value)
			for _, param := range params {
				check := compileMinMax(v.Type(), name, param)
				if check == nil {
					continue
				}
				if got, want := check(v), original(value, param); got != want {
					t.Errorf("%s(%T(%v), %q) = %v, want %v", name, value, value, param, got, want)
				}
			}
		}
	}
}

func TestCompiledValidationErrorsAndModifiers(t *testing.T) {
	type input struct {
		ID    int64    `json:"id" validate:"min=9007199254740993,max=9007199254740995"`
		Page  int      `json:"page" validate:"min=1,max=2" message:"bad page"`
		Name  string   `json:"name" validate:"omitempty,min=3,max=4"`
		Score float64  `json:"score" validate:"min=0,max=100"`
		Tags  []string `json:"tags" validate:"min=1,dive,min=2,max=4"`
	}
	v := NewValidator().Messages(map[string]string{"max": "{field} exceeds {param}"})
	compiled := buildValidators[input](v)
	original := append([]fieldValidator(nil), compiled...)
	for i := range original {
		original[i].checks = nil
	}
	for _, in := range []input{
		{ID: 9007199254740993, Page: 1, Name: "ก", Score: 2.5, Tags: []string{"go"}},
		{ID: 9007199254740992, Page: 0, Name: "a", Score: math.NaN(), Tags: []string{"a", "abcde"}},
		{ID: 9007199254740996, Page: 3, Name: "", Score: math.Inf(1), Tags: nil},
	} {
		got := validate(compiled, reflect.ValueOf(&in).Elem(), v.messages)
		want := validate(original, reflect.ValueOf(&in).Elem(), v.messages)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("validation = %#v, want %#v", got, want)
		}
	}
}

func TestCompiledValidationCustomRuleSnapshot(t *testing.T) {
	type input struct {
		Value int `validate:"min=0,mutate,max=3"`
	}
	in := input{Value: 3}
	calls := 0
	v := NewValidator().Register("mutate", func(value any, param string) bool {
		calls++
		in.Value = 100
		return value == 3 && param == ""
	})
	if err := validate(buildValidators[input](v), reflect.ValueOf(&in).Elem(), v.messages); err != nil {
		t.Fatalf("rules must share the original field snapshot: %v", err)
	}
	if calls != 1 || in.Value != 100 {
		t.Fatalf("custom rule calls=%d, value=%d", calls, in.Value)
	}
}

func TestCompiledValidationBuiltinOverrides(t *testing.T) {
	type input struct {
		Value int `validate:"min=0,max=100"`
	}
	for _, name := range []string{"min", "max"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			v := NewValidator().Register(name, func(value any, param string) bool {
				calls++
				return false
			})
			err := validate(buildValidators[input](v), reflect.ValueOf(input{Value: 3}), v.messages)
			if calls != 1 || err == nil || len(err.Errors) != 1 || err.Errors[0].Rule != name {
				t.Fatalf("override calls=%d, error=%v", calls, err)
			}
		})
	}
}

func TestCompiledValidationCopiedValidatorOverride(t *testing.T) {
	type input struct {
		Value int `validate:"min=0,max=100"`
	}
	for _, name := range []string{"min", "max"} {
		t.Run(name, func(t *testing.T) {
			v := NewValidator()
			copied := *v
			calls := 0
			copied.Register(name, func(value any, param string) bool {
				calls++
				return false
			})
			for _, validator := range []*Validator{v, &copied} {
				err := validate(buildValidators[input](validator), reflect.ValueOf(input{Value: 3}), validator.messages)
				if err == nil || len(err.Errors) != 1 || err.Errors[0].Rule != name {
					t.Fatalf("copied override error=%v", err)
				}
			}
			if calls != 2 {
				t.Fatalf("copied override calls=%d, want 2", calls)
			}
		})
	}
}

func BenchmarkCompiledValidation(b *testing.B) {
	type input struct {
		ID    int64   `validate:"min=1"`
		Page  int     `validate:"min=1,max=1000"`
		Score float64 `validate:"min=0,max=100"`
		Name  string  `validate:"min=1,max=64"`
	}
	in := input{ID: 42, Page: 3, Score: 2.5, Name: "alice"}
	value := reflect.ValueOf(&in).Elem()
	v := NewValidator()
	for _, mode := range []string{"original", "compiled"} {
		b.Run(mode, func(b *testing.B) {
			validators := buildValidators[input](v)
			if mode == "original" {
				for i := range validators {
					validators[i].checks = nil
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := validate(validators, value, v.messages); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
