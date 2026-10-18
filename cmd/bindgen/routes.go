package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var methodSuffixes = [...]string{"Get", "Post", "Put", "Delete", "Patch"}

func rejectRouteBuiltinShadows(filename string, input *ast.File, funcName string) error {
	builtins := map[string]bool{"string": true, "make": true, "len": true, "append": true}
	if builtins[funcName] {
		return fmt.Errorf("generated binder %s shadows builtin %s required by route helpers", funcName, funcName)
	}
	check := func(path string, f *ast.File) error {
		for _, decl := range f.Decls {
			var names []*ast.Ident
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					names = append(names, d.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, s.Name)
					case *ast.ValueSpec:
						names = append(names, s.Names...)
					}
				}
			}
			for _, name := range names {
				if builtins[name.Name] {
					return fmt.Errorf("%s: declaration shadows builtin %s required by route helpers", path, name.Name)
				}
			}
		}
		return nil
	}
	if err := check(filename, input); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Dir(filename))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || entry.Name() == filepath.Base(filename) {
			continue
		}
		path := filepath.Join(filepath.Dir(filename), entry.Name())
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		if f.Name.Name == input.Name.Name {
			if err := check(path, f); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectRouteNameCollisions(typeName, funcName string) error {
	names := map[string]bool{}
	for _, name := range []string{typeName, funcName, funcName + "Shape", funcName + "Attested", "make" + strings.ToUpper(funcName[:1]) + funcName[1:] + "Validator", typeName + "Plan"} {
		if names[name] {
			return fmt.Errorf("generated declaration %s conflicts with input or binder declarations", name)
		}
		names[name] = true
	}
	for _, target := range []string{"", "Group"} {
		for _, method := range methodSuffixes {
			name := typeName + target + method
			if names[name] {
				return fmt.Errorf("generated route helper %s conflicts with input or binder declarations", name)
			}
			names[name] = true
		}
	}
	return nil
}

func generateRouteRegistrations(out *bytes.Buffer, typeName, qualifier string) {
	outputType := "Out"
	if typeName == outputType {
		outputType = "OutResult"
	}
	for _, target := range []string{"", "Group"} {
		arg, typ := "app", "App"
		if target == "Group" {
			arg, typ = "g", "Group"
		}
		for _, method := range methodSuffixes {
			name := typeName + target + method
			fmt.Fprintf(out, "\n// %s registers %s with %sPlan before caller options.\n", name, strings.ToUpper(method), typeName)
			fmt.Fprintf(out, "func %s[%s interface{}](%s *%s%s, path string, handler func(*%sC[%s]) (*%s, error), opts ...%sRouteOption) {\n", name, outputType, arg, qualifier, typ, qualifier, typeName, outputType, qualifier)
			fmt.Fprintf(out, "options := make([]%sRouteOption, 1, len(opts)+1)\noptions[0] = %sWithGeneratedPlan(%sPlan())\noptions = append(options, opts...)\n", qualifier, qualifier, typeName)
			fmt.Fprintf(out, "%s%s%s(%s, path, handler, options...)\n}\n", qualifier, target, method, arg)
		}
	}
}
