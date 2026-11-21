package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
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

	projectName := filepath.Base(name)
	moduleName := filepath.ToSlash(filepath.Clean(name))
	argument := filepath.ToSlash(name)
	if filepath.IsAbs(name) || moduleName == "." || moduleName == ".." ||
		strings.HasPrefix(argument, "./") || strings.HasPrefix(argument, "../") {
		target, err := filepath.Abs(name)
		if err != nil {
			return fmt.Errorf("resolving project directory: %w", err)
		}
		projectName = filepath.Base(target)
		moduleName = projectName
	}
	if err := validateModulePath(moduleName); err != nil {
		return err
	}
	data := templateData{ProjectName: projectName, ModuleName: moduleName}

	// Scaffold the project from embedded templates
	tmplDir := "templates/" + tmplName
	if err := scaffoldFromFS(templateFS, tmplDir, name, data); err != nil {
		return fmt.Errorf("scaffolding failed: %w", err)
	}

	// Print getting started message
	printGettingStarted(name, tmplName)
	return nil
}

func validateModulePath(moduleName string) error {
	// Like go mod init, allow local main module names without a domain.
	if err := module.CheckImportPath(moduleName); err != nil {
		return fmt.Errorf("invalid module path %q: %w", moduleName, err)
	}
	if moduleName == "go" || moduleName == "toolchain" {
		return fmt.Errorf("module path %q is reserved", moduleName)
	}
	if _, _, ok := module.SplitPathVersion(moduleName); !ok {
		return fmt.Errorf("invalid major version suffix in module path %q", moduleName)
	}
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
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		return err
	}
	defer root.Close()
	return scaffoldInto(fsys, tmplDir, root, ".", data)
}

func scaffoldInto(fsys embed.FS, tmplDir string, root *os.Root, targetDir string, data templateData) error {
	return fs.WalkDir(fsys, tmplDir, func(templatePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if templatePath == tmplDir {
			return nil
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(strings.TrimPrefix(templatePath, tmplDir+"/")))

		if d.IsDir() {
			return root.MkdirAll(targetPath, 0o755)
		}

		// Read template content
		content, err := fsys.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", templatePath, err)
		}

		// Strip .tmpl suffix from target filename
		targetPath = strings.TrimSuffix(targetPath, ".tmpl")

		// Execute template
		tmpl, err := template.New(path.Base(templatePath)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", templatePath, err)
		}

		f, err := root.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			return fmt.Errorf("creating %s: %w", targetPath, err)
		}
		execErr := tmpl.Execute(f, data)
		closeErr := f.Close()
		if execErr != nil {
			return fmt.Errorf("executing template %s: %w", templatePath, execErr)
		}
		return closeErr
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
	fmt.Println("  AI Integration (Claude Code / Cursor):")
	fmt.Println("    .mcp.json included — restart your AI assistant to enable Kruda tools.")
	fmt.Println("    Verify: kruda mcp --test")
	fmt.Println()
}
