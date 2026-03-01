// Package main provides the kruda CLI tool for project scaffolding,
// development server, code generation, and configuration validation.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "kruda",
		Short:   "Kruda — Type-safe Go web framework CLI",
		Long:    "CLI tool for the Kruda web framework. Scaffold projects, run dev servers, generate code, and validate configuration.",
		Version: version,
	}

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)

	// PGO commands: kruda pgo, kruda pgo info, kruda pgo strip
	pgoCmd.AddCommand(pgoInfoCmd)
	pgoCmd.AddCommand(pgoStripCmd)
	rootCmd.AddCommand(pgoCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
