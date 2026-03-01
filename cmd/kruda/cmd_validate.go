package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// validateCmd validates the Kruda project configuration and environment.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Kruda project configuration",
	Long: `Check Go version compatibility, required dependencies, and project setup.

Validates:
  • Go version is 1.24 or higher
  • go.mod exists and contains the kruda dependency
  • Project structure is correct

Exit code 0 on success, 1 on failure (CI-friendly).`,
	RunE: runValidate,
}

// validationResult tracks a single check.
type validationResult struct {
	name    string
	passed  bool
	message string
	suggest string
}

// runValidate performs all validation checks and prints a summary.
func runValidate(cmd *cobra.Command, args []string) error {
	var results []validationResult

	results = append(results, checkGoVersion())
	results = append(results, checkGoMod())
	results = append(results, checkKrudaDependency())

	// Print results.
	fmt.Println()
	fmt.Println("  Kruda Project Validation")
	fmt.Println("  ========================")
	fmt.Println()

	allPassed := true
	for _, r := range results {
		if r.passed {
			fmt.Printf("  ✅ %s — %s\n", r.name, r.message)
		} else {
			fmt.Printf("  ❌ %s — %s\n", r.name, r.message)
			if r.suggest != "" {
				fmt.Printf("     💡 %s\n", r.suggest)
			}
			allPassed = false
		}
	}

	fmt.Println()

	if allPassed {
		fmt.Println("  All checks passed! Your project is ready.")
		return nil
	}

	// Return an error to trigger non-zero exit code via Cobra.
	return fmt.Errorf("validation failed: %d issue(s) found", countFailed(results))
}

// checkGoVersion verifies Go >= 1.24 is installed.
func checkGoVersion() validationResult {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		return validationResult{
			name:    "Go Version",
			passed:  false,
			message: "could not run 'go version'",
			suggest: "Ensure Go is installed and available in your PATH",
		}
	}

	versionStr := string(out)
	major, minor, ok := parseGoVersion(versionStr)
	if !ok {
		return validationResult{
			name:    "Go Version",
			passed:  false,
			message: fmt.Sprintf("could not parse Go version from: %s", strings.TrimSpace(versionStr)),
			suggest: "Expected format: go version go1.XX.Y ...",
		}
	}

	if major < 1 || (major == 1 && minor < 24) {
		return validationResult{
			name:    "Go Version",
			passed:  false,
			message: fmt.Sprintf("Go %d.%d found, but Kruda requires Go 1.24+", major, minor),
			suggest: "Upgrade Go: https://go.dev/dl/",
		}
	}

	return validationResult{
		name:    "Go Version",
		passed:  true,
		message: fmt.Sprintf("Go %d.%d detected (>= 1.24 required)", major, minor),
	}
}

// goVersionRe matches "go1.XX" or "go1.XX.Y" in `go version` output.
var goVersionRe = regexp.MustCompile(`go(\d+)\.(\d+)`)

// parseGoVersion extracts major and minor version from `go version` output.
func parseGoVersion(output string) (major, minor int, ok bool) {
	matches := goVersionRe.FindStringSubmatch(output)
	if len(matches) < 3 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(matches[1])
	minor, err2 := strconv.Atoi(matches[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// checkGoMod verifies that go.mod exists in the current directory.
func checkGoMod() validationResult {
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return validationResult{
			name:    "go.mod",
			passed:  false,
			message: "go.mod not found in current directory",
			suggest: "Run 'go mod init <module>' or 'kruda new <project>' to create a project",
		}
	}

	return validationResult{
		name:    "go.mod",
		passed:  true,
		message: "go.mod found",
	}
}

// checkKrudaDependency verifies that go.mod contains the kruda dependency.
func checkKrudaDependency() validationResult {
	f, err := os.Open("go.mod")
	if err != nil {
		return validationResult{
			name:    "Kruda Dependency",
			passed:  false,
			message: "could not read go.mod",
			suggest: "Ensure go.mod exists and is readable",
		}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "github.com/go-kruda/kruda") {
			return validationResult{
				name:    "Kruda Dependency",
				passed:  true,
				message: "github.com/go-kruda/kruda found in go.mod",
			}
		}
	}

	return validationResult{
		name:    "Kruda Dependency",
		passed:  false,
		message: "github.com/go-kruda/kruda not found in go.mod",
		suggest: "Run 'go get github.com/go-kruda/kruda' to add the dependency",
	}
}

// countFailed returns the number of failed checks.
func countFailed(results []validationResult) int {
	n := 0
	for _, r := range results {
		if !r.passed {
			n++
		}
	}
	return n
}
