package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateBinderCLI(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kruda")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	run := func(workdir string, args ...string) (string, string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = workdir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	t.Run("help and aliases", func(t *testing.T) {
		out, stderr, err := run(dir, "generate", "--help")
		if err != nil || !strings.Contains(out, "binder") {
			t.Fatalf("generate help: %v\n%s\n%s", err, out, stderr)
		}
		for _, alias := range []string{"generate", "gen", "g"} {
			out, stderr, err := run(dir, alias, "binder", "--help")
			if err != nil {
				t.Fatalf("binder help: %v\n%s", err, stderr)
			}
			for _, text := range []string{"--input", "--type", "--output", "--func", "Go", "scalar", "replaces", "WithGeneratedPlan"} {
				if !strings.Contains(out, text) {
					t.Errorf("help missing %q:\n%s", text, out)
				}
			}
		}
	})

	t.Run("required values", func(t *testing.T) {
		for _, flag := range []string{"input", "type", "output", "func"} {
			for _, empty := range []bool{false, true} {
				args := []string{"generate", "binder"}
				for name, value := range map[string]string{"input": "input.go", "type": "SearchInput", "output": "generated.go", "func": "bindSearchInput"} {
					if name == flag {
						if !empty {
							continue
						}
						value = ""
					}
					args = append(args, "--"+name+"="+value)
				}
				out, stderr, err := run(dir, args...)
				want := "required flag(s)"
				if empty {
					want = "must not be empty"
				}
				if err == nil || !strings.Contains(stderr, want) || strings.Count(stderr, "Error:") != 1 || out != "" {
					t.Errorf("%s empty=%v: err=%v stdout=%q stderr=%q", flag, empty, err, out, stderr)
				}
			}
		}
		_, stderr, err := run(dir, "generate", "binder", "extra", "--input=input.go", "--type=SearchInput", "--output=generated.go", "--func=bindSearchInput")
		if err == nil || !strings.Contains(stderr, "extra") || strings.Count(stderr, "Error:") != 1 {
			t.Fatalf("positional argument: %v\n%s", err, stderr)
		}
	})

	t.Run("source integration", func(t *testing.T) {
		coreMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if os.IsNotExist(err) || (err == nil && !bytes.HasPrefix(coreMod, []byte("module github.com/go-kruda/kruda\n"))) {
			t.Skip("generator integration requires the parent Kruda core source checkout")
		}
		if err != nil {
			t.Fatal(err)
		}
		mod := []byte("module binder.test\n\ngo 1.26.9\n\nrequire github.com/go-kruda/kruda v0.0.0\n\nreplace github.com/go-kruda/kruda => " + strconv.Quote(filepath.ToSlash(root)) + "\n")
		sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"go.mod": mod, "go.sum": sum} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Run("generator parity replacement and argument boundaries", func(t *testing.T) {
			input := "input ; $value.go"
			output := "generated ; $value.go"
			src := []byte("package consumer\ntype SearchInput struct { Q string `query:\"q\" default:\"all\" validate:\"min=1,max=64\"`; Page int `query:\"page\" default:\"1\" validate:\"min=1,max=1000\"` }\n")
			if err := os.WriteFile(filepath.Join(dir, input), src, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, output), []byte("package consumer\n// old output\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"generate", "binder", "--input", input, "--type", "SearchInput", "--output", output, "--func", "bindSearchInput"}
			var first []byte
			for i := 0; i < 2; i++ {
				stdout, stderr, err := run(dir, args...)
				if err != nil || stdout != "" {
					t.Fatalf("generation %d: %v\nstdout=%s\nstderr=%s", i, err, stdout, stderr)
				}
				got, err := os.ReadFile(filepath.Join(dir, output))
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					first = got
				} else if !bytes.Equal(got, first) {
					t.Fatal("repeated generation changed output")
				}
				for _, text := range []string{"Code generated", "func SearchInputPlan()", "kruda.AttestBinderShape[SearchInput]", "ValidatorFactory:"} {
					if !bytes.Contains(got, []byte(text)) {
						t.Errorf("output missing %q", text)
					}
				}
				standalone := exec.Command("go", "run", "github.com/go-kruda/kruda/cmd/bindgen", "-input", input, "-type", "SearchInput", "-output", "standalone.go", "-func", "bindSearchInput")
				standalone.Dir = dir
				standalone.Env = append(os.Environ(), "GOWORK=off")
				if out, err := standalone.CombinedOutput(); err != nil {
					t.Fatalf("standalone generation: %v\n%s", err, out)
				}
				want, err := os.ReadFile(filepath.Join(dir, "standalone.go"))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("CLI and standalone output differ: %v", err)
				}
			}
			for name, want := range map[string][]byte{"go.mod": mod, "go.sum": sum, input: src} {
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("generation changed %s: %v", name, err)
				}
			}
		})

		t.Run("successful Go diagnostics", func(t *testing.T) {
			cmd := exec.Command(bin, "generate", "binder", "--input", "input ; $value.go", "--type=SearchInput", "--output=diagnostics.go", "--func=bindSearchInput")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-x")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "WORK=") || strings.Contains(stderr.String(), "Error:") {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			got, err := os.ReadFile(filepath.Join(dir, "diagnostics.go"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(dir, "standalone.go"))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("diagnostics changed generator output: %v", err)
			}
		})

		t.Run("generator failures preserve output", func(t *testing.T) {
			for _, tc := range []struct {
				name, source, typeName, funcName, output, want string
			}{
				{"unreadable input", "", "Input", "bindInput", "generated.go", "input.go"},
				{"invalid type", "package consumer", "bad-type", "bindInput", "generated.go", "must be identifiers"},
				{"invalid function", "package consumer", "Input", "bad-func", "generated.go", "must be identifiers"},
				{"missing type", "package consumer", "Input", "bindInput", "generated.go", "input struct Input not found"},
				{"nonstruct type", "package consumer; type Input string", "Input", "bindInput", "generated.go", "input must be a struct"},
				{"body", "package consumer; type Input struct { Q string `json:\"q\"` }", "Input", "bindInput", "generated.go", "body and form fields are unsupported"},
				{"form", "package consumer; type Input struct { Q string `form:\"q\"` }", "Input", "bindInput", "generated.go", "body and form fields are unsupported"},
				{"slice", "package consumer; type Input struct { Q []string `query:\"q\"` }", "Input", "bindInput", "generated.go", "only builtin scalar types are supported"},
				{"default", "package consumer; type Input struct { Page int `query:\"page\" default:\"bad\"` }", "Input", "bindInput", "generated.go", "invalid default"},
				{"output directory missing", "package consumer; type Input struct { Q string `query:\"q\"` }", "Input", "bindInput", "missing/generated.go", "missing"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					workdir, err := os.MkdirTemp(dir, "failure-")
					if err != nil {
						t.Fatal(err)
					}
					if tc.source != "" {
						if err := os.WriteFile(filepath.Join(workdir, "input.go"), []byte(tc.source), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					prior := []byte("package consumer\n// existing output\n")
					if tc.output == "generated.go" {
						if err := os.WriteFile(filepath.Join(workdir, tc.output), prior, 0o644); err != nil {
							t.Fatal(err)
						}
					}
					out, stderr, err := run(workdir, "generate", "binder", "--input=input.go", "--type="+tc.typeName, "--output="+tc.output, "--func="+tc.funcName)
					if err == nil || out != "" || !strings.Contains(stderr, tc.want) || strings.Count(stderr, "Error:") != 1 {
						t.Fatalf("err=%v stdout=%q stderr=%q, want %q", err, out, stderr, tc.want)
					}
					if tc.output == "generated.go" {
						got, err := os.ReadFile(filepath.Join(workdir, tc.output))
						if err != nil || !bytes.Equal(got, prior) {
							t.Fatalf("failure changed existing output: %v", err)
						}
					}
				})
			}
		})
	})

	t.Run("Go unavailable", func(t *testing.T) {
		cmd := exec.Command(bin, "generate", "binder", "--input=input.go", "--type=Input", "--output=generated.go", "--func=bindInput")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+t.TempDir())
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "running bindgen:") || strings.Count(stderr.String(), "Error:") != 1 {
			t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
	})
}

func TestRunBindgenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := runBindgen(cmd, "input.go", "Input", "generated.go", "bindInput")
	if !errors.Is(err, context.Canceled) || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}
