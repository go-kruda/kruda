package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateBinderRoutesCLI(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kruda")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	runBinderCommand(t, "", true, "go", "build", "-o", bin, ".")
	writeBinderFile(t, filepath.Join(dir, "go.mod"), []byte("module routes.test\n\n"+goDirective(t, root)+"\n\nrequire github.com/go-kruda/kruda v0.0.0\nreplace github.com/go-kruda/kruda => "+strconv.Quote(filepath.ToSlash(root))+"\n"))
	for _, name := range []string{"go.sum", "cmd/bindgen/testdata/routes_input.go.txt", "cmd/bindgen/testdata/routes_consumer_test.go.txt"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Base(name)
		switch target {
		case "routes_input.go.txt":
			target = "input.go"
		case "routes_consumer_test.go.txt":
			target = "consumer_test.go"
		}
		writeBinderFile(t, filepath.Join(dir, target), data)
	}
	args := []string{"generate", "binder", "--input=input.go", "--type=SearchInput", "--output=search_generated.go", "--func=bindSearchInput"}
	t.Run("default false and parity", func(t *testing.T) {
		runBinderCommand(t, dir, true, bin, args...)
		plain := readBinderFile(t, filepath.Join(dir, "search_generated.go"))
		runBinderCommand(t, dir, true, bin, append(args, "--routes=false")...)
		if !bytes.Equal(plain, readBinderFile(t, filepath.Join(dir, "search_generated.go"))) {
			t.Fatal("explicit false changed binder-only output")
		}
		standalone := []string{"run", "github.com/go-kruda/kruda/cmd/bindgen", "-input=input.go", "-type=SearchInput", "-output=standalone.go", "-func=bindSearchInput"}
		runBinderCommand(t, dir, true, "go", append(standalone, "-routes=false")...)
		if !bytes.Equal(plain, readBinderFile(t, filepath.Join(dir, "standalone.go"))) {
			t.Fatal("standalone false changed binder-only output")
		}
		var first []byte
		for _, flag := range []string{"--routes", "--routes=true"} {
			runBinderCommand(t, dir, true, bin, append(args, flag)...)
			got := readBinderFile(t, filepath.Join(dir, "search_generated.go"))
			if first == nil {
				first = got
			} else if !bytes.Equal(first, got) {
				t.Fatal("repeated route generation changed bytes")
			}
			runBinderCommand(t, dir, true, "go", append(standalone, "-routes")...)
			if !bytes.Equal(got, readBinderFile(t, filepath.Join(dir, "standalone.go"))) {
				t.Fatal("CLI and standalone route output differ")
			}
		}
		if err := os.Remove(filepath.Join(dir, "standalone.go")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("external HTTP consumer", func(t *testing.T) {
		original := readBinderFile(t, filepath.Join(dir, "input.go"))
		mod := readBinderFile(t, filepath.Join(dir, "go.mod"))
		sum := readBinderFile(t, filepath.Join(dir, "go.sum"))
		for _, typ := range []string{"StaleInput", "UnsupportedInput", "ChangedValidationInput"} {
			runBinderCommand(t, dir, true, bin, "generate", "binder", "--input=input.go", "--type="+typ, "--output="+typ+"_generated.go", "--func=bind"+typ, "--routes")
		}
		for name, want := range map[string][]byte{"input.go": original, "go.mod": mod, "go.sum": sum} {
			if !bytes.Equal(readBinderFile(t, filepath.Join(dir, name)), want) {
				t.Fatalf("generation changed %s", name)
			}
		}
		generated := readBinderFile(t, filepath.Join(dir, "search_generated.go"))
		needle := []byte("func bindSearchInput(c *kruda.Ctx) (reflect.Value, error) {\n")
		if bytes.Count(generated, needle) != 1 {
			t.Fatal("test binder instrumentation point missing")
		}
		writeBinderFile(t, filepath.Join(dir, "search_generated.go"), bytes.Replace(generated, needle, append(append([]byte(nil), needle...), []byte("generatedBinds++\n")...), 1))
		retagged := strings.Replace(string(original), "query:\"q\" default:\"old\"", "query:\"fresh\" default:\"fresh\"", 1)
		retagged = strings.Replace(retagged, "query:\"page\" validate:\"min=1\"", "query:\"page\" validate:\"min=5\"", 1)
		if retagged == string(original) {
			t.Fatal("stale fixture retag did not change input")
		}
		writeBinderFile(t, filepath.Join(dir, "input.go"), []byte(retagged))
		runBinderCommand(t, dir, true, "go", "test", "-mod=mod", "-tags", "kruda_stdjson", "-count=1", "-v", ".")
	})
	t.Run("route failures preserve output", func(t *testing.T) {
		for _, tc := range []struct{ name, source, typ, binder, flag, want string }{
			{"unreadable input", "", "Input", "bindInput", "", "input.go"},
			{"malformed source", "package consumer; type Input struct {", "Input", "bindInput", "", "expected"},
			{"invalid type", "package consumer", "bad-type", "bindInput", "", "identifiers"},
			{"invalid binder", "package consumer", "Input", "bad-func", "", "identifiers"},
			{"missing type", "package consumer", "Input", "bindInput", "", "not found"},
			{"body", "package consumer; type Input struct { Q string `json:\"q\"` }", "Input", "bindInput", "", "body and form"},
			{"form", "package consumer; type Input struct { Q string `form:\"q\"` }", "Input", "bindInput", "", "body and form"},
			{"unsupported", "package consumer; type Input struct { Q []string }", "Input", "bindInput", "", "scalar"},
			{"default", "package consumer; type Input struct { Q int `default:\"bad\"` }", "Input", "bindInput", "", "invalid default"},
			{"collision", "package consumer; type Input struct{}", "Input", "InputGroupPost", "", "conflicts"},
			{"output directory missing", "package consumer; type Input struct{}", "Input", "bindInput", "output=missing/generated.go", "missing"},
			{"invalid boolean", "package consumer; type Input struct{}", "Input", "bindInput", "routes=maybe", "invalid"},
			{"empty boolean", "package consumer; type Input struct{}", "Input", "bindInput", "routes=", "invalid"},
			{"unknown method", "package consumer; type Input struct{}", "Input", "bindInput", "method=HEAD", "flag"},
			{"unknown target", "package consumer; type Input struct{}", "Input", "bindInput", "target=router", "flag"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				work := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-"))
				if err := os.Mkdir(work, 0755); err != nil {
					t.Fatal(err)
				}
				if tc.source != "" {
					writeBinderFile(t, filepath.Join(work, "input.go"), []byte(tc.source))
				}
				prior := []byte("package consumer\n// previous output\n")
				writeBinderFile(t, filepath.Join(work, "generated.go"), prior)
				for _, standalone := range []bool{false, true} {
					command, prefix, start := bin, "--", []string{"generate", "binder"}
					if standalone {
						command, prefix, start = "go", "-", []string{"run", "github.com/go-kruda/kruda/cmd/bindgen"}
					}
					args := append(start, prefix+"input=input.go", prefix+"type="+tc.typ, prefix+"output=generated.go", prefix+"func="+tc.binder)
					if strings.HasPrefix(tc.flag, "routes=") {
						args = append(args, prefix+tc.flag)
					} else {
						args = append(args, prefix+"routes")
						if tc.flag != "" {
							args = append(args, prefix+tc.flag)
						}
					}
					out := runBinderCommand(t, work, false, command, args...)
					if !strings.Contains(out, tc.want) {
						t.Fatalf("missing %q in %s", tc.want, out)
					}
					if !bytes.Equal(readBinderFile(t, filepath.Join(work, "generated.go")), prior) {
						t.Fatal("failure changed output")
					}
				}
			})
		}
	})
	t.Run("required route values", func(t *testing.T) {
		work := filepath.Join(dir, "required")
		if err := os.Mkdir(work, 0755); err != nil {
			t.Fatal(err)
		}
		prior := []byte("package consumer\n// previous output\n")
		writeBinderFile(t, filepath.Join(work, "generated.go"), prior)
		for _, missing := range []string{"input", "type", "output", "func"} {
			for _, empty := range []bool{false, true} {
				for _, standalone := range []bool{false, true} {
					command, prefix, args := bin, "--", []string{"generate", "binder"}
					if standalone {
						command, prefix, args = "go", "-", []string{"run", "github.com/go-kruda/kruda/cmd/bindgen"}
					}
					for _, flag := range []struct{ name, value string }{{"input", "input.go"}, {"type", "Input"}, {"output", "generated.go"}, {"func", "bindInput"}} {
						if flag.name == missing {
							if !empty {
								continue
							}
							flag.value = ""
						}
						args = append(args, prefix+flag.name+"="+flag.value)
					}
					out := runBinderCommand(t, work, false, command, append(args, prefix+"routes")...)
					want := "required"
					if !standalone && empty {
						want = "must not be empty"
					}
					if !strings.Contains(out, want) || !bytes.Equal(readBinderFile(t, filepath.Join(work, "generated.go")), prior) {
						t.Fatalf("%s empty=%v standalone=%v: %s", missing, empty, standalone, out)
					}
				}
			}
		}
	})
	t.Run("module selected old generator", func(t *testing.T) {
		old := filepath.Join(dir, "oldcore")
		if err := os.MkdirAll(filepath.Join(old, "cmd", "bindgen"), 0755); err != nil {
			t.Fatal(err)
		}
		writeBinderFile(t, filepath.Join(old, "go.mod"), []byte("module github.com/go-kruda/kruda\ngo 1.26.11\n"))
		writeBinderFile(t, filepath.Join(old, "cmd", "bindgen", "main.go"), []byte(`package main
import("flag"; "os")
func main() {
 flag.String("input", "", "input"); flag.String("type", "", "type"); flag.String("func", "", "func")
 output := flag.String("output", "", "output"); flag.Parse()
 if err := os.WriteFile(*output, []byte("legacy binder"), 0644); err != nil { panic(err) }
}
`))
		consumer := filepath.Join(dir, "oldconsumer")
		if err := os.Mkdir(consumer, 0755); err != nil {
			t.Fatal(err)
		}
		writeBinderFile(t, filepath.Join(consumer, "go.mod"), []byte("module legacy.test\ngo 1.26.11\nrequire github.com/go-kruda/kruda v0.0.0\nreplace github.com/go-kruda/kruda => "+strconv.Quote(filepath.ToSlash(old))+"\n"))
		args := []string{"generate", "binder", "--input=input.go", "--type=Input", "--output=generated.go", "--func=bindInput"}
		for _, extra := range [][]string{nil, {"--routes=false"}} {
			runBinderCommand(t, consumer, true, bin, append(args, extra...)...)
			if got := string(readBinderFile(t, filepath.Join(consumer, "generated.go"))); got != "legacy binder" {
				t.Fatalf("default invocation did not use consumer module: %q", got)
			}
		}
		out := runBinderCommand(t, consumer, false, bin, append(args, "--routes")...)
		if !strings.Contains(out, "flag provided but not defined: -routes") || string(readBinderFile(t, filepath.Join(consumer, "generated.go"))) != "legacy binder" {
			t.Fatalf("old module rejection lost diagnostic or replaced output: %s", out)
		}
	})
}

func runBinderCommand(t *testing.T, dir string, success bool, program string, args ...string) string {
	t.Helper()
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("%s %v: %v\n%s", program, args, err, out)
	}
	if len(out) > 0 {
		t.Logf("%s %v\n%s", program, args, out)
	}
	return string(out)
}

func writeBinderFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func readBinderFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
