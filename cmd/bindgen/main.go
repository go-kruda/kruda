package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

type field struct {
	name, typ, kind, query, param, def string
	validate                           string
	index                              int
}

// shapeField records one struct field for binder staleness attestation.
// Every field lands here in struct order, including unexported ones the
// binder skips, so any later add/remove/rename/retag/retype fails closed.
type shapeField struct {
	name, kind, query, param, def string
	exported                      bool
}

func main() {
	input := flag.String("input", "", "Go source containing the input struct")
	typeName := flag.String("type", "", "input struct name")
	output := flag.String("output", "", "generated Go file")
	funcName := flag.String("func", "", "generated binder name")
	flag.Parse()
	if err := run(*input, *typeName, *output, *funcName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, typeName, output, funcName string) error {
	if input == "" || output == "" || !token.IsIdentifier(typeName) || !token.IsIdentifier(funcName) {
		return fmt.Errorf("input, output, type, and func are required; type and func must be identifiers")
	}
	src, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	generated, err := generate(input, src, typeName, funcName)
	if err != nil {
		return err
	}
	return os.WriteFile(output, generated, 0644)
}

func generate(filename string, src []byte, typeName, funcName string) ([]byte, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		return nil, err
	}
	var input *ast.StructType
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, spec := range g.Specs {
			s := spec.(*ast.TypeSpec)
			if s.Name.Name != typeName {
				continue
			}
			if s.Assign.IsValid() || s.TypeParams != nil {
				return nil, fmt.Errorf("%s: aliases and generic inputs are unsupported", typeName)
			}
			input, ok = s.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("%s: input must be a struct", typeName)
			}
		}
	}
	if input == nil {
		return nil, fmt.Errorf("input struct %s not found", typeName)
	}
	var fields []field
	var shapes []shapeField
	fieldIndex := 0
	validationSupported := true
	for _, node := range input.Fields.List {
		if len(node.Names) == 0 {
			return nil, fmt.Errorf("%s: embedded fields are unsupported", typeName)
		}
		var tag reflect.StructTag
		if node.Tag != nil {
			text, err := strconv.Unquote(node.Tag.Value)
			if err != nil {
				return nil, err
			}
			tag = reflect.StructTag(text)
		}
		for _, name := range node.Names {
			index := fieldIndex
			fieldIndex++
			validation := tag.Get("validate")
			if !name.IsExported() {
				if validation != "" {
					validationSupported = false
				}
				shapes = append(shapes, shapeField{name: name.Name})
				continue
			}
			if (tag.Get("json") != "" && tag.Get("json") != "-") || tag.Get("form") != "" {
				return nil, fmt.Errorf("%s: body and form fields are unsupported", name.Name)
			}
			id, ok := node.Type.(*ast.Ident)
			if !ok || id.Obj != nil {
				return nil, fmt.Errorf("%s: only builtin scalar types are supported", name.Name)
			}
			kind := id.Name
			switch kind {
			case "byte":
				kind = "uint8"
			case "rune":
				kind = "int32"
			case "string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			default:
				return nil, fmt.Errorf("%s: unsupported type %s", name.Name, id.Name)
			}
			fields = append(fields, field{name.Name, id.Name, kind, tag.Get("query"), tag.Get("param"), tag.Get("default"), validation, index})
			shapes = append(shapes, shapeField{name.Name, kind, tag.Get("query"), tag.Get("param"), tag.Get("default"), true})
		}
	}

	if err := rejectShadowedTypes(filename, f.Name.Name, fields); err != nil {
		return nil, err
	}
	// External packages reference the framework through the kruda import;
	// white-box output inside package kruda stays unqualified.
	qualifier := ""
	if f.Name.Name != "kruda" {
		qualifier = "kruda."
	}
	var body bytes.Buffer
	fmt.Fprintf(&body, "func %s(c *%sCtx) (reflect.Value, error) {\ninput := new(%s)\n", funcName, qualifier, typeName)
	useMath, useStrconv := false, false
	for _, f := range fields {
		if f.def == "" {
			continue
		}
		value, mathUsed, err := defaultValue(f)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid default %q: %w", f.name, f.def, err)
		}
		useMath = useMath || mathUsed
		fmt.Fprintf(&body, "input.%s = %s\n", f.name, value)
	}
	queryNames := generateQueryCollection(&body, fields)
	for _, source := range []string{"query", "param"} {
		for _, f := range fields {
			tag, getter, label := f.query, "Query", "query"
			if source == "param" {
				tag, getter, label = f.param, "Param", "path"
			}
			if tag == "" {
				continue
			}
			if source == "query" {
				fmt.Fprintf(&body, "{\nraw := %s\nif !queryBound { raw = c.Query(%q) }\nif raw != \"\" {\n", queryNames[tag], tag)
			} else {
				fmt.Fprintf(&body, "if raw := c.%s(%q); raw != \"\" {\n", getter, tag)
			}
			if f.kind == "string" {
				fmt.Fprintf(&body, "input.%s = raw\n", f.name)
			} else {
				useStrconv = true
				fmt.Fprintf(&body, "value, err := %s\n", parseExpr(f.kind, "raw"))
				message := fmt.Sprintf("invalid %s parameter %q: expected %s", label, tag, f.kind)
				fmt.Fprintf(&body, "if err != nil { return reflect.Value{}, %sBadRequest(%q) }\n", qualifier, message)
				fmt.Fprintf(&body, "input.%s = %s(value)\n", f.name, f.typ)
			}
			fmt.Fprintln(&body, "}")
			if source == "query" {
				fmt.Fprintln(&body, "}")
			}
		}
	}
	fmt.Fprintln(&body, "return reflect.ValueOf(input).Elem(), nil\n}")
	body.Write(generateValidator(typeName, funcName, fields, validationSupported, qualifier))
	generateBinderAttestation(&body, typeName, funcName, shapes, qualifier)
	generatePlanConstructor(&body, typeName, funcName, qualifier)
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by cmd/bindgen; DO NOT EDIT.\n\npackage %s\n\nimport (\n\"reflect\"\n", f.Name.Name)
	if qualifier != "" {
		fmt.Fprintln(&out, "\"github.com/go-kruda/kruda\"")
	}
	if useMath {
		fmt.Fprintln(&out, "\"math\"")
	}
	if useStrconv {
		fmt.Fprintln(&out, "\"strconv\"")
	}
	if len(queryNames) > 0 {
		fmt.Fprintln(&out, "\"strings\"")
	}
	fmt.Fprintln(&out, ")")
	out.Write(body.Bytes())
	return format.Source(out.Bytes())
}

// generateBinderAttestation emits the shape record and the attested binder
// factory. The factory returns nil when the source struct no longer matches,
// so routes fall back to the generic parser instead of binding a stale
// shape. The qualifier is empty for white-box output and "kruda." for
// external packages.
func generateBinderAttestation(out *bytes.Buffer, typeName, funcName string, shapes []shapeField, qualifier string) {
	fmt.Fprintf(out, "\nvar %sShape = %sBinderShape{NumFields: %d, Fields: []%sBinderFieldShape{\n", funcName, qualifier, len(shapes), qualifier)
	for _, s := range shapes {
		fmt.Fprintf(out, "{Name: %q, Exported: %t, Kind: %q, Query: %q, Param: %q, Default: %q},\n",
			s.name, s.exported, s.kind, s.query, s.param, s.def)
	}
	fmt.Fprintln(out, "}}")
	fmt.Fprintf(out, "\nfunc %sAttested() func(*%sCtx) (reflect.Value, error) {\n", funcName, qualifier)
	fmt.Fprintf(out, "if !%sAttestBinderShape[%s](%sShape) { return nil }\nreturn %s\n}\n", qualifier, typeName, funcName, funcName)
}

// generatePlanConstructor emits the GeneratedPlan bundle for the route
// option: the committed shape, the attested binder, and the validator
// factory. The constructor is named after the input type; generating twice
// for one type in a package is a loud compile error, never a silent pick.
func generatePlanConstructor(out *bytes.Buffer, typeName, funcName, qualifier string) {
	factory := "make" + strings.ToUpper(funcName[:1]) + funcName[1:] + "Validator"
	fmt.Fprintf(out, "\nfunc %sPlan() %sGeneratedPlan[%s] {\n", typeName, qualifier, typeName)
	fmt.Fprintf(out, "return %sGeneratedPlan[%s]{Shape: %sShape, Binder: %sAttested(), ValidatorFactory: %s}\n}\n",
		qualifier, typeName, funcName, funcName, factory)
}

// Scan all sibling Go files conservatively, including files for other build
// configurations, so generation cannot silently change meaning across targets.
func rejectShadowedTypes(filename, packageName string, fields []field) error {
	used := make(map[string]bool)
	for _, f := range fields {
		used[f.typ] = true
	}
	dir := filepath.Dir(filename)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || entry.Name() == filepath.Base(filename) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		if f.Name.Name != packageName {
			continue
		}
		for _, decl := range f.Decls {
			g, ok := decl.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, spec := range g.Specs {
				s := spec.(*ast.TypeSpec)
				if used[s.Name.Name] {
					return fmt.Errorf("%s: builtin type %s is shadowed by a sibling declaration", path, s.Name.Name)
				}
			}
		}
	}
	return nil
}

func parseExpr(kind, value string) string {
	switch kind {
	case "bool":
		return "strconv.ParseBool(" + value + ")"
	case "float32", "float64":
		return fmt.Sprintf("strconv.ParseFloat(%s, %s)", value, kind[5:])
	case "int":
		return "strconv.ParseInt(" + value + ", 10, strconv.IntSize)"
	case "uint":
		return "strconv.ParseUint(" + value + ", 10, strconv.IntSize)"
	}
	if kind[0] == 'u' {
		return fmt.Sprintf("strconv.ParseUint(%s, 10, %s)", value, kind[4:])
	}
	return fmt.Sprintf("strconv.ParseInt(%s, 10, %s)", value, kind[3:])
}

func defaultValue(f field) (string, bool, error) {
	switch f.kind {
	case "string":
		return strconv.Quote(f.def), false, nil
	case "bool":
		v, err := strconv.ParseBool(f.def)
		return strconv.FormatBool(v), false, err
	case "float32", "float64":
		bits, _ := strconv.Atoi(f.kind[5:])
		v, err := strconv.ParseFloat(f.def, bits)
		if math.IsNaN(v) || math.IsInf(v, 0) || (v == 0 && math.Signbit(v)) {
			if bits == 32 {
				return fmt.Sprintf("math.Float32frombits(%d)", math.Float32bits(float32(v))), true, err
			}
			return fmt.Sprintf("math.Float64frombits(%d)", math.Float64bits(v)), true, err
		}
		return strconv.FormatFloat(v, 'g', -1, bits), false, err
	}
	// Native-width defaults must also compile on 32-bit targets. Runtime
	// query and parameter parsing still uses the target's strconv.IntSize.
	bits := 32
	if f.kind[0] == 'u' {
		if f.kind != "uint" {
			bits, _ = strconv.Atoi(f.kind[4:])
		}
		v, err := strconv.ParseUint(f.def, 10, bits)
		if err != nil && f.kind == "uint" {
			return "", false, fmt.Errorf("uint defaults must fit 32 bits for portable generation: %w", err)
		}
		return strconv.FormatUint(v, 10), false, err
	}
	if f.kind != "int" {
		bits, _ = strconv.Atoi(f.kind[3:])
	}
	v, err := strconv.ParseInt(f.def, 10, bits)
	if err != nil && f.kind == "int" {
		return "", false, fmt.Errorf("int defaults must fit 32 bits for portable generation: %w", err)
	}
	return strconv.FormatInt(v, 10), false, err
}
