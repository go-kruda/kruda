// Package main provides the kruda CLI tool for project scaffolding,
// development server, code generation, and configuration validation.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags by the release build.
var version = "dev"

// resolveVersion falls back to the version the module was installed at, so a
// binary from `go install github.com/go-kruda/kruda/cmd/kruda@latest` reports
// that version instead of "dev" — otherwise "which version are you on?" has no
// answer for anyone who did not build from source.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return version
	}
	return info.Main.Version
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "kruda",
		Short:   "Kruda — Type-safe Go web framework CLI",
		Long:    "CLI tool for the Kruda web framework. Scaffold projects, run dev servers, generate code, and validate configuration.",
		Version: resolveVersion(),
	}

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(mcpCmd)

	// PGO commands: kruda pgo, kruda pgo info, kruda pgo strip
	pgoCmd.AddCommand(pgoInfoCmd)
	pgoCmd.AddCommand(pgoStripCmd)
	rootCmd.AddCommand(pgoCmd)

	// Cobra already prints the error; printing it again here just doubled every
	// failure message.
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
