package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var binderCmd = &cobra.Command{
	Use:   "binder",
	Short: "Generate a typed query/path input binder",
	Long: `Generate a binder for scalar query/path fields using the existing cmd/bindgen tool.

Requires Go on PATH and a project dependency on a Kruda core version containing
cmd/bindgen. The project's module graph selects the generator. Input and output
paths are relative to the working directory; the output directory must exist.
Successful generation replaces the requested output file.

Register the emitted plan manually with kruda.WithGeneratedPlan(InputPlan()).

Example:
  kruda generate binder --input input.go --type SearchInput --output search_generated.go --func bindSearchInput`,
	Args: cobra.NoArgs,
	RunE: runGenerateBinder,
}

func init() {
	binderCmd.Flags().String("input", "", "Go source containing the input struct")
	binderCmd.Flags().String("type", "", "input struct name")
	binderCmd.Flags().String("output", "", "generated Go file (replaces existing output)")
	binderCmd.Flags().String("func", "", "generated binder name")
	for _, name := range []string{"input", "type", "output", "func"} {
		binderCmd.MarkFlagRequired(name)
	}
}

func runGenerateBinder(cmd *cobra.Command, args []string) error {
	input, _ := cmd.Flags().GetString("input")
	typeName, _ := cmd.Flags().GetString("type")
	output, _ := cmd.Flags().GetString("output")
	funcName, _ := cmd.Flags().GetString("func")
	if input == "" || typeName == "" || output == "" || funcName == "" {
		return fmt.Errorf("--input, --type, --output, and --func must not be empty")
	}
	return runBindgen(cmd, input, typeName, output, funcName)
}

func runBindgen(cmd *cobra.Command, input, typeName, output, funcName string) error {
	process := exec.CommandContext(cmd.Context(), "go", "run", "github.com/go-kruda/kruda/cmd/bindgen",
		"-input", input, "-type", typeName, "-output", output, "-func", funcName)
	var stderr bytes.Buffer
	process.Stdout = cmd.OutOrStdout()
	process.Stderr = &stderr
	if err := process.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("bindgen: %s", message)
		}
		return fmt.Errorf("running bindgen: %w", err)
	}
	fmt.Fprint(cmd.ErrOrStderr(), stderr.String())
	return nil
}
