package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateRoutes(t *testing.T) {
	src := []byte("package catalog; type SearchInput struct { Q string `query:\"q\" default:\"all\"` }")
	filename := filepath.Join(t.TempDir(), "input.go")
	plain, err := generate(filename, src, "SearchInput", "bindSearch")
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := generateWithRoutes(filename, src, "SearchInput", "bindSearch", false)
	if err != nil || !bytes.Equal(plain, disabled) {
		t.Fatalf("default output changed: %v", err)
	}
	got, err := generateWithRoutes(filename, src, "SearchInput", "bindSearch", true)
	if err != nil {
		t.Fatal(err)
	}
	again, err := generateWithRoutes(filename, src, "SearchInput", "bindSearch", true)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("route output is not deterministic: %v", err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "generated.go", got, 0)
	if err != nil {
		t.Fatal(err)
	}
	var helpers []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type.TypeParams == nil {
			continue
		}
		helpers = append(helpers, fn.Name.Name)
		injection := fn.Body.List[1].(*ast.AssignStmt).Rhs[0].(*ast.CallExpr)
		if injection.Fun.(*ast.SelectorExpr).Sel.Name != "WithGeneratedPlan" || injection.Args[0].(*ast.CallExpr).Fun.(*ast.Ident).Name != "SearchInputPlan" {
			t.Fatalf("%s does not prepend its plan", fn.Name)
		}
		call := fn.Body.List[len(fn.Body.List)-1].(*ast.ExprStmt).X.(*ast.CallExpr)
		if got := call.Fun.(*ast.SelectorExpr).Sel.Name; got != strings.TrimPrefix(fn.Name.Name, "SearchInput") {
			t.Fatalf("%s delegates to %s", fn.Name, got)
		}
	}
	want := "SearchInputGet SearchInputPost SearchInputPut SearchInputDelete SearchInputPatch SearchInputGroupGet SearchInputGroupPost SearchInputGroupPut SearchInputGroupDelete SearchInputGroupPatch"
	if strings.Join(helpers, " ") != want {
		t.Fatalf("helper order = %v", helpers)
	}
}

func TestRouteDeclarationCollisionsPreserveOutput(t *testing.T) {
	for _, tc := range []struct{ input, binder string }{
		{"Input", "InputGet"}, {"Input", "InputGroupPatch"}, {"Input", "InputPlan"},
		{"Input", "Input"}, {"bindShape", "bind"}, {"bindAttested", "bind"},
		{"makeBindValidator", "bind"},
	} {
		t.Run(tc.input+"/"+tc.binder, func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "input.go"), filepath.Join(dir, "generated.go")
			writeRouteFixture(t, input, []byte("package catalog; type "+tc.input+" struct{}"))
			prior := []byte("package catalog\n// previous output\n")
			writeRouteFixture(t, output, prior)
			if err := run(input, tc.input, output, tc.binder, true); err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("expected emission collision, got %v", err)
			}
			got, err := os.ReadFile(output)
			if err != nil || !bytes.Equal(got, prior) {
				t.Fatalf("failed generation replaced output: %v", err)
			}
		})
	}
}

func TestRouteBuiltinInputNames(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeRouteFixture(t, filepath.Join(dir, "go.mod"), []byte("module routes.test\n\n"+goDirective(t, root)+"\n\nrequire github.com/go-kruda/kruda v0.0.0\nreplace github.com/go-kruda/kruda => "+strconv.Quote(filepath.ToSlash(root))+"\n"))
	for _, name := range []string{"string", "make", "len", "append"} {
		t.Run(name, func(t *testing.T) {
			pkg := filepath.Join(dir, name)
			if err := os.Mkdir(pkg, 0755); err != nil {
				t.Fatal(err)
			}
			input, output := filepath.Join(pkg, "input.go"), filepath.Join(pkg, "generated.go")
			writeRouteFixture(t, input, []byte("package catalog; type "+name+" struct { ID int `param:\"id\"` }"))
			if err := run(input, name, output, "bindInput", false); err != nil {
				t.Fatalf("binder-only generation: %v", err)
			}
			prior, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if err := run(input, name, output, "bindInput", true); err == nil || !strings.Contains(err.Error(), "builtin "+name) {
				t.Fatalf("expected route builtin rejection, got %v", err)
			}
			got, err := os.ReadFile(output)
			if err != nil || !bytes.Equal(got, prior) {
				t.Fatalf("failed generation replaced binder-only output: %v", err)
			}
		})
	}
	runRouteFixtureGo(t, dir, true, "test", "-mod=mod", "-tags", "kruda_stdjson", "./...")
	runRouteFixtureGo(t, dir, true, "test", "-mod=mod", "./...")
}

func TestRouteBuiltinShadowsPreserveOutput(t *testing.T) {
	for _, name := range []string{"string", "make", "len", "append"} {
		for _, location := range []string{"current", "sibling", "excluded"} {
			for _, decl := range []struct{ kind, source string }{
				{"type", "type (other struct{}; " + name + " struct{})"},
				{"alias", "type " + name + " = int"},
				{"func", "func " + name + "() {}"},
				{"var", "var other, " + name + " int"},
				{"const", "const (other = 1; " + name + " = 2)"},
				{"binder", ""},
			} {
				if decl.kind == "binder" && location != "current" {
					continue
				}
				t.Run(name+"/"+location+"/"+decl.kind, func(t *testing.T) {
					dir := t.TempDir()
					input, output := filepath.Join(dir, "input.go"), filepath.Join(dir, "generated.go")
					src := "package catalog; type Input struct { ID int `param:\"id\"` }; "
					binder := "bindInput"
					if decl.kind == "binder" {
						binder = name
					} else if location == "current" {
						src += decl.source
					} else {
						sibling := "package catalog; " + decl.source
						if location == "excluded" {
							sibling = "//go:build ignore\n\n" + sibling
						}
						writeRouteFixture(t, filepath.Join(dir, "sibling.go"), []byte(sibling))
					}
					writeRouteFixture(t, input, []byte(src))
					if err := run(input, "Input", output, binder, false); err != nil {
						t.Fatalf("binder-only generation: %v", err)
					}
					prior, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					if err := run(input, "Input", output, binder, true); err == nil || !strings.Contains(err.Error(), "builtin "+name) {
						t.Fatalf("expected route builtin rejection, got %v", err)
					}
					got, err := os.ReadFile(output)
					if err != nil || !bytes.Equal(got, prior) {
						t.Fatalf("failed generation replaced output: %v", err)
					}
				})
			}
		}
	}
}

func TestGeneratedRoutesCompile(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeRouteFixture(t, filepath.Join(dir, "go.mod"), []byte("module routes.test\n\n"+goDirective(t, root)+"\n\nrequire github.com/go-kruda/kruda v0.0.0\nreplace github.com/go-kruda/kruda => "+strconv.Quote(filepath.ToSlash(root))+"\n"))
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	writeRouteFixture(t, filepath.Join(dir, "go.sum"), sum)
	for i, name := range []string{"searchInput", "Out", "Result", "out", "result", "app", "path", "handler", "opts", "any", "scopedInput", "siblingScopedInput"} {
		pkg := filepath.Join(dir, fmt.Sprintf("input%d", i))
		if err := os.Mkdir(pkg, 0755); err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(pkg, "input.go")
		src := []byte("package catalog; type " + name + " struct { Q string `query:\"q\"` }")
		if name == "scopedInput" || name == "siblingScopedInput" {
			declarations := []byte(`
type receiver struct{}
func (receiver) string() {}
func (receiver) make() {}
func (receiver) len() {}
func (receiver) append() {}
func localBindings() {
	type string struct{}
	var make int
	const len = 1
	append := func() {}
	_ = string{}
	_, _, _ = make, len, append
}
`)
			if name == "scopedInput" {
				src = append(src, declarations...)
			} else {
				writeRouteFixture(t, filepath.Join(pkg, "sibling.go"), append([]byte("package catalog\n"), declarations...))
			}
			writeRouteFixture(t, filepath.Join(pkg, "sibling_test.go"), []byte("package catalog_test; type string struct{}; var make int; const len = 1; func append() {}"))
		}
		writeRouteFixture(t, input, src)
		got, err := generateWithRoutes(input, src, name, "bindInput", true)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte("interface{}")) {
			t.Fatal("output constraint must not use a shadowable any alias")
		}
		writeRouteFixture(t, filepath.Join(pkg, "generated.go"), got)
		var calls strings.Builder
		fmt.Fprintf(&calls, "package catalog\nimport \"github.com/go-kruda/kruda\"\ntype response struct{ Q string }\nfunc register(a *kruda.App) {\nh := func(c *kruda.C[%s]) (*response, error) { return &response{c.In.Q}, nil }\ng := a.Group(\"/api\")\n", name)
		for _, method := range []string{"Get", "Post", "Put", "Delete", "Patch"} {
			fmt.Fprintf(&calls, "%s%s(a, \"/search\", h)\n%sGroup%s(g, \"/search\", h)\n", name, method, name, method)
		}
		calls.WriteString("}\n")
		writeRouteFixture(t, filepath.Join(pkg, "register.go"), []byte(calls.String()))
	}
	runRouteFixtureGo(t, dir, true, "test", "-mod=mod", "-tags", "kruda_stdjson", "./...")
	runRouteFixtureGo(t, dir, true, "test", "-mod=mod", "./...")
	bad := filepath.Join(dir, "input0", "bad.go")
	for _, callback := range []string{
		"func(c *kruda.C[response]) (*response, error) { return nil, nil }",
		"func(c *kruda.C[searchInput]) (response, error) { return response{}, nil }",
	} {
		writeRouteFixture(t, bad, []byte("package catalog\nimport \"github.com/go-kruda/kruda\"\nfunc invalid(a *kruda.App) { searchInputGet(a, \"/wrong\", "+callback+") }"))
		runRouteFixtureGo(t, dir, false, "test", "-mod=mod", "-tags", "kruda_stdjson", "./input0")
	}
}

func TestRouteGenerationLeavesSiblingNameConflictsToCompiler(t *testing.T) {
	dir := t.TempDir()
	writeRouteFixture(t, filepath.Join(dir, "sibling.go"), []byte("package catalog; func InputGet() {}"))
	src := []byte("package catalog; type Input struct{}")
	if _, err := generateWithRoutes(filepath.Join(dir, "input.go"), src, "Input", "bindInput", true); err != nil {
		t.Fatalf("generation unexpectedly scanned sibling declarations: %v", err)
	}
}

func TestGeneratedRoutesWhiteBoxCompile(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, folder := range []string{".", "transport", "json"} {
		entries, err := os.ReadDir(filepath.Join(root, folder))
		if err != nil {
			t.Fatal(err)
		}
		if folder != "." {
			if err := os.Mkdir(filepath.Join(dir, folder), 0755); err != nil {
				t.Fatal(err)
			}
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || strings.HasSuffix(name, "_test.go") || !(strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, folder, name))
			if err != nil {
				t.Fatal(err)
			}
			writeRouteFixture(t, filepath.Join(dir, folder, name), data)
		}
	}
	input := filepath.Join(dir, "route_input.go")
	src := []byte("package kruda; type routeInput struct { Q string `query:\"q\"` }")
	writeRouteFixture(t, input, src)
	got, err := generateWithRoutes(input, src, "routeInput", "bindRouteInput", true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("kruda.")) {
		t.Fatal("white-box output imports itself")
	}
	writeRouteFixture(t, filepath.Join(dir, "route_generated.go"), got)
	runRouteFixtureGo(t, dir, true, "test", "-mod=mod", "-tags", "kruda_stdjson", ".")
}

func writeRouteFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func runRouteFixtureGo(t *testing.T, dir string, success bool, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if success && err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
	if !success && (err == nil || !(bytes.Contains(out, []byte("does not match")) || bytes.Contains(out, []byte("cannot infer")) || bytes.Contains(out, []byte("cannot use")))) {
		t.Fatalf("expected callback compilation error, got %v\n%s", err, out)
	}
}

// goDirective returns the repository's go.mod "go" line, so fixture modules
// always meet the current Go floor.
func goDirective(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatal("no go directive in " + root + "/go.mod")
	return ""
}
