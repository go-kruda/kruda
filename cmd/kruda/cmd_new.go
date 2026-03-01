package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed all:templates
var templateFS embed.FS

// templateData holds the data passed to template files during scaffolding.
type templateData struct {
	ProjectName string
	ModuleName  string
}

var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "Create a new Kruda project",
	Long: `Scaffold a new Kruda project with the specified template.

Templates:
  minimal    Single main.go with hello world (default)
  api        JSON API with routes, handlers, and models directories
  fullstack  API + static file serving setup`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().StringP("template", "t", "minimal", "project template (minimal, api, fullstack)")
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	tmplName, _ := cmd.Flags().GetString("template")

	// Validate template name
	switch tmplName {
	case "minimal", "api", "fullstack":
	default:
		return fmt.Errorf("unknown template %q: must be minimal, api, or fullstack", tmplName)
	}

	// Check if target directory exists and is non-empty
	if isNonEmptyDir(name) {
		return fmt.Errorf("directory %q already exists and is not empty", name)
	}

	data := templateData{
		ProjectName: name,
		ModuleName:  name,
	}

	// Scaffold the project from embedded templates
	tmplDir := filepath.Join("templates", tmplName)
	if err := scaffoldFromFS(templateFS, tmplDir, name, data); err != nil {
		return fmt.Errorf("scaffolding failed: %w", err)
	}

	// Print getting started message
	printGettingStarted(name, tmplName)
	return nil
}

// isNonEmptyDir returns true if path is an existing non-empty directory.
func isNonEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false // doesn't exist or can't read
	}
	return len(entries) > 0
}

// scaffoldFromFS walks the embedded template directory and creates files
// in the target directory, executing Go templates with the provided data.
func scaffoldFromFS(fsys embed.FS, tmplDir, targetDir string, data templateData) error {
	return fs.WalkDir(fsys, tmplDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path from template root
		relPath, err := filepath.Rel(tmplDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		// Read template content
		content, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", path, err)
		}

		// Strip .tmpl suffix from target filename
		targetPath = strings.TrimSuffix(targetPath, ".tmpl")

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		// Execute template
		tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", path, err)
		}

		f, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", targetPath, err)
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("executing template %s: %w", path, err)
		}

		return nil
	})
}

// printGettingStarted prints next steps after scaffolding.
func printGettingStarted(name, tmplName string) {
	fmt.Println()
	fmt.Printf("  ✅ Created project %q with %s template\n", name, tmplName)
	fmt.Println()
	fmt.Println("  Getting started:")
	fmt.Printf("    cd %s\n", name)
	fmt.Println("    go mod tidy")
	fmt.Println("    go run .")
	fmt.Println()
}
