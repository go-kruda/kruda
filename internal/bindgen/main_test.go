package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixtureOutputIsCurrent(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, fixture := range []struct {
		name, input, typeName, funcName, output string
	}{
		{"binder", "bindgen_fixture_test.go", "generatedBinderInput", "bindGeneratedBinderInput", "bindgen_generated_test.go"},
		{"composition_5", "composition_fixture_test.go", "composition5Input", "bindComposition5Input", "composition_5_generated_test.go"},
		{"composition_10", "composition_fixture_test.go", "composition10Input", "bindComposition10Input", "composition_10_generated_test.go"},
		{"composition_30", "composition_fixture_test.go", "composition30Input", "bindComposition30Input", "composition_30_generated_test.go"},
		{"external", filepath.Join("externalbind", "input.go"), "searchInput", "bindSearchInput", filepath.Join("externalbind", "external_generated.go")},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			input := filepath.Join(root, fixture.input)
			src, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := generate(input, src, fixture.typeName, fixture.funcName)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(root, fixture.output))
			if err != nil {
				t.Fatal(err)
			}
			if !generatedFilesEqual(got, want) {
				t.Fatal("generated fixture output is stale")
			}
		})
	}
}

func generatedFilesEqual(got, want []byte) bool {
	return bytes.Equal(got, bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")))
}

func TestGeneratedFilesEqualNormalizesWindowsLineEndings(t *testing.T) {
	if !generatedFilesEqual([]byte("package fixture\n"), []byte("package fixture\r\n")) {
		t.Fatal("expected CRLF fixture to match generated LF output")
	}
	if generatedFilesEqual([]byte("package fixture\n"), []byte("package other\r\n")) {
		t.Fatal("line-ending normalization must not hide content changes")
	}
}

func TestGenerateRejectsSiblingBuiltinShadowing(t *testing.T) {
	for _, typ := range []string{"int8", "uint", "string", "bool", "byte"} {
		t.Run(typ, func(t *testing.T) {
			dir := t.TempDir()
			sibling := []byte("package kruda; type " + typ + " = int64")
			if err := os.WriteFile(filepath.Join(dir, "shadow.go"), sibling, 0644); err != nil {
				t.Fatal(err)
			}
			src := []byte("package kruda; type Input struct { Value " + typ + " `query:\"value\"` }")
			_, err := generate(filepath.Join(dir, "input.go"), src, "Input", "bindInput")
			if err == nil || !strings.Contains(err.Error(), "shadowed by a sibling declaration") {
				t.Fatalf("expected explicit sibling shadowing rejection, got %v", err)
			}
		})
	}
}

func TestGenerateIgnoresDifferentPackageTypeNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "external_test.go"), []byte("package kruda_test; type int8 = int64"), 0644); err != nil {
		t.Fatal(err)
	}
	src := []byte("package kruda; type Input struct { Value int8 `query:\"value\"` }")
	if _, err := generate(filepath.Join(dir, "input.go"), src, "Input", "bindInput"); err != nil {
		t.Fatal(err)
	}
}

func TestNativeDefaultsArePortable(t *testing.T) {
	for _, tc := range []struct{ kind, value string }{
		{"int", "2147483648"}, {"int", "-2147483649"}, {"uint", "4294967296"},
	} {
		if _, _, err := defaultValue(field{kind: tc.kind, def: tc.value}); err == nil || !strings.Contains(err.Error(), "must fit 32 bits") {
			t.Fatalf("%s default %s: expected portable-width error, got %v", tc.kind, tc.value, err)
		}
	}
	for _, tc := range []struct{ kind, value string }{
		{"int", "2147483647"}, {"int", "-2147483648"}, {"uint", "4294967295"},
		{"int64", "9223372036854775807"}, {"uint64", "18446744073709551615"},
	} {
		if got, _, err := defaultValue(field{kind: tc.kind, def: tc.value}); err != nil || got != tc.value {
			t.Fatalf("%s default %s: got %s, %v", tc.kind, tc.value, got, err)
		}
	}
}

func TestGenerateSourceOrderAndErrors(t *testing.T) {
	src := "package kruda\ntype Input struct {\n" +
		"ID int64 `param:\"id\" query:\"id\" default:\"7\"`\n" +
		"Page int `query:\"page\" default:\"1\"`\n" +
		"Name string `query:\"name\" default:\"guest\"`\n}\n"
	got, err := generate("input.go", []byte(src), "Input", "bindInput")
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	ordered := []string{
		"input := new(Input)", "input.ID = 7", "input.Page = 1", `input.Name = "guest"`,
		`c.Query("id")`, `c.Query("page")`, `c.Query("name")`, `c.Param("id")`,
		"return reflect.ValueOf(input).Elem(), nil",
	}
	position := 0
	for _, fragment := range ordered {
		n := strings.Index(text[position:], fragment)
		if n < 0 {
			t.Fatalf("missing or misplaced %q in:\n%s", fragment, text)
		}
		position += n + len(fragment)
	}
	for _, fragment := range []string{
		`strconv.ParseInt(raw, 10, strconv.IntSize)`,
		`BadRequest("invalid query parameter \"id\": expected int64")`,
		`BadRequest("invalid path parameter \"id\": expected int64")`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing %q in:\n%s", fragment, text)
		}
	}
}

func TestGenerateScalarWidths(t *testing.T) {
	cases := []struct{ typ, parse, kind string }{
		{"int", "ParseInt(raw, 10, strconv.IntSize)", "int"},
		{"int8", "ParseInt(raw, 10, 8)", "int8"},
		{"int16", "ParseInt(raw, 10, 16)", "int16"},
		{"int32", "ParseInt(raw, 10, 32)", "int32"},
		{"int64", "ParseInt(raw, 10, 64)", "int64"},
		{"uint", "ParseUint(raw, 10, strconv.IntSize)", "uint"},
		{"uint8", "ParseUint(raw, 10, 8)", "uint8"},
		{"uint16", "ParseUint(raw, 10, 16)", "uint16"},
		{"uint32", "ParseUint(raw, 10, 32)", "uint32"},
		{"uint64", "ParseUint(raw, 10, 64)", "uint64"},
		{"float32", "ParseFloat(raw, 32)", "float32"},
		{"float64", "ParseFloat(raw, 64)", "float64"},
		{"bool", "ParseBool(raw)", "bool"},
		{"byte", "ParseUint(raw, 10, 8)", "uint8"},
		{"rune", "ParseInt(raw, 10, 32)", "int32"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			src := "package kruda; type Input struct { Value " + tc.typ + " `query:\"v\"` }"
			got, err := generate("input.go", []byte(src), "Input", "bindInput")
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"strconv." + tc.parse, "expected " + tc.kind, "input.Value = " + tc.typ + "(value)"} {
				if !strings.Contains(string(got), want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

func TestGenerateRejectsUnsupportedInputs(t *testing.T) {
	for _, declaration := range []string{
		"type Input int",
		"type Input = struct { Value int }",
		"type Input[T any] struct { Value int }",
		"type Other struct {}; type Input struct { Other }",
		"type Input struct { Value *int }",
		"type Input struct { Value []string }",
		"type Named string; type Input struct { Value Named }",
		"type Named bool; type Input struct { Value Named }",
		"type int int64; type Input struct { Value int }",
		"type Input struct { Value uintptr }",
		"type Input struct { Value string `json:\"value\"` }",
		"type Input struct { Value string `form:\"value\"` }",
		"type Input struct { Value int8 `default:\"128\"` }",
		"type Input struct { Value uint `default:\"-1\"` }",
		"type Input struct { Value bool `default:\"yes\"` }",
		"type Input struct { Value float32 `default:\"1e100\"` }",
		"type Missing struct{}",
	} {
		t.Run(declaration, func(t *testing.T) {
			if _, err := generate("input.go", []byte("package kruda; "+declaration), "Input", "bindInput"); err == nil {
				t.Fatal("unsupported input accepted")
			}
		})
	}
}

func TestGenerateStringOnlyAndUnexportedFields(t *testing.T) {
	src := "package kruda; type Input struct { Name string `query:\"name\"`; hidden *int `json:\"hidden\"` }"
	got, err := generate("input.go", []byte(src), "Input", "bindInput")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "strconv") || strings.Contains(string(got), "input.hidden") {
		t.Fatalf("unused converter or unexported field binding emitted:\n%s", got)
	}
	// The unexported name is recorded in the attestation shape so a later
	// add/remove/rename fails closed; it must never be bound.
	if !strings.Contains(string(got), `{Name: "hidden", Exported: false`) {
		t.Fatalf("unexported field missing from attestation shape:\n%s", got)
	}
}

func TestGenerateExternalBinder(t *testing.T) {
	src := "package shop; type Search struct { Q string `query:\"q\" default:\"all\"`; Page int `query:\"page\" default:\"1\"` }"
	got, err := generate("input.go", []byte(src), "Search", "bindSearch")
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		"package shop",
		`"github.com/go-kruda/kruda"`,
		"func bindSearch(c *kruda.Ctx)",
		"kruda.BadRequest(",
		"var bindSearchShape = kruda.BinderShape{",
		"kruda.AttestBinderShape[Search](bindSearchShape)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	for _, banned := range []string{"fieldValidator", "makeBindSearchValidator", "(*Ctx)"} {
		if strings.Contains(text, banned) {
			t.Errorf("external output must not contain %q:\n%s", banned, text)
		}
	}
}

func TestGenerateExternalValidatedIsRejected(t *testing.T) {
	for _, tt := range []struct {
		name, field string
	}{
		{"compiled", "Q string `query:\"q\" validate:\"min=1\"`"},
		{"required", "Q string `query:\"q\" validate:\"required\"`"},
		{"email", "Q string `query:\"q\" validate:\"email\"`"},
		{"nan", "Q string `query:\"q\" validate:\"min=NaN\"`"},
		{"infinity", "Q string `query:\"q\" validate:\"min=Inf\"`"},
		{"unexported", "q string `validate:\"required\"`"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := "package shop; type Search struct { " + tt.field + " }"
			if _, err := generate("input.go", []byte(src), "Search", "bindSearch"); err == nil ||
				!strings.Contains(err.Error(), "exported validator descriptor") {
				t.Fatalf("expected white-box-only rejection, got %v", err)
			}
		})
	}
}

func TestGenerateEmitsAttestedBinder(t *testing.T) {
	src := "package kruda; type Input struct { ID int64 `param:\"id\" query:\"id\" default:\"7\"` }"
	got, err := generate("input.go", []byte(src), "Input", "bindInput")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"var bindInputShape = BinderShape{NumFields: 1",
		`{Name: "ID", Exported: true, Kind: "int64", Query: "id", Param: "id", Default: "7"}`,
		"func bindInputAttested() func(*Ctx) (reflect.Value, error)",
		"AttestBinderShape[Input](bindInputShape)",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestDefaultFloatPreservesSpecialValues(t *testing.T) {
	for _, tc := range []struct{ kind, def, want string }{
		{"float64", "-0", "math.Float64frombits(9223372036854775808)"},
		{"float32", "-0", "math.Float32frombits(2147483648)"},
		{"float64", "+Inf", "math.Float64frombits(9218868437227405312)"},
		{"float32", "-Inf", "math.Float32frombits(4286578688)"},
	} {
		got, useMath, err := defaultValue(field{kind: tc.kind, def: tc.def})
		if err != nil || !useMath || got != tc.want {
			t.Fatalf("%s(%s): got %s math=%v err=%v", tc.kind, tc.def, got, useMath, err)
		}
	}
}
