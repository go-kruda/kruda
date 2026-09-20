package main

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type validationField struct {
	index  int
	name   string
	kind   string
	rules  [][2]string
	checks []string
}

// generateValidator emits the validator factory and whether it compiled to a
// real predicate. A false ok means the caller gets a nil-returning stub
// (white-box) or no validator at all (external packages, which cannot name
// the unexported fieldValidator the stub signature needs).
func generateValidator(typeName, binderName string, fields []field, supported bool) ([]byte, bool) {
	var compiled []validationField
	for _, f := range fields {
		if f.validate == "" {
			continue
		}
		v := validationField{index: f.index, name: f.name, kind: f.kind}
		for _, tag := range strings.Split(f.validate, ",") {
			name, param, _ := strings.Cut(strings.TrimSpace(tag), "=")
			check, ok := validationExpr(f, name, param)
			if !ok {
				supported = false
			}
			v.rules = append(v.rules, [2]string{name, param})
			v.checks = append(v.checks, check)
		}
		compiled = append(compiled, v)
	}
	name := []rune(binderName)
	name[0] = unicode.ToUpper(name[0])
	var out bytes.Buffer
	fmt.Fprintf(&out, "\nfunc make%sValidator(validators []fieldValidator) func(*%s) bool {\n", string(name), typeName)
	if !supported || len(compiled) == 0 {
		fmt.Fprintln(&out, "return nil\n}")
		return out.Bytes(), false
	}
	fmt.Fprintf(&out, "if len(validators) != %d { return nil }\n", len(compiled))
	fmt.Fprintf(&out, "inputType := reflect.TypeOf((*%s)(nil)).Elem()\n", typeName)
	fmt.Fprintln(&out, "// Compiled checks certify that these exact rules have no custom overrides.")
	for i, f := range compiled {
		kind := strings.ToUpper(f.kind[:1]) + f.kind[1:]
		fmt.Fprintf(&out, "if inputType.NumField() <= %d || inputType.Field(%d).Name != %q || inputType.Field(%d).Type.Kind() != reflect.%s { return nil }\n", f.index, f.index, f.name, f.index, kind)
		fmt.Fprintf(&out, "if validators[%d].index != %d || validators[%d].omitEmpty || len(validators[%d].elemRules) != 0 || len(validators[%d].rules) != %d || len(validators[%d].checks) != %d { return nil }\n", i, f.index, i, i, i, len(f.rules), i, len(f.rules))
		for j, rule := range f.rules {
			fmt.Fprintf(&out, "if validators[%d].rules[%d].name != %q || validators[%d].rules[%d].param != %q { return nil }\n", i, j, rule[0], i, j, rule[1])
		}
	}
	fmt.Fprintf(&out, "return func(input *%s) bool {\nreturn ", typeName)
	var checks []string
	for _, f := range compiled {
		checks = append(checks, f.checks...)
	}
	fmt.Fprintln(&out, strings.Join(checks, " && "))
	fmt.Fprintln(&out, "}\n}")
	return out.Bytes(), true
}

func validationExpr(f field, name, param string) (string, bool) {
	if name != "min" && name != "max" {
		return "", false
	}
	n, err := strconv.ParseFloat(param, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return "", false
	}
	op := ">="
	if name == "max" {
		op = "<="
	}
	value := "input." + f.name
	bound := strconv.FormatFloat(n, 'g', -1, 64)
	switch f.kind {
	case "string":
		value = "float64(len(" + value + "))"
	case "int", "int8", "int16", "int32", "int64":
		if ni, err := strconv.ParseInt(param, 10, 64); err == nil {
			return "int64(" + value + ") " + op + " " + strconv.FormatInt(ni, 10), true
		}
		value = "float64(" + value + ")"
	case "uint", "uint8", "uint16", "uint32", "uint64":
		if nu, err := strconv.ParseUint(param, 10, 64); err == nil {
			return "uint64(" + value + ") " + op + " " + strconv.FormatUint(nu, 10), true
		}
		value = "float64(" + value + ")"
	case "float32", "float64":
		value = "float64(" + value + ")"
	default:
		return "", false
	}
	return value + " " + op + " " + bound, true
}
