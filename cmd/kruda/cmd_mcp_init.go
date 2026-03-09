package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var mcpInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate MCP config files for AI coding assistants",
	Long: `Generate .mcp.json (Claude Code) and .cursor/mcp.json (Cursor) in the
current directory so AI assistants can use Kruda MCP tools automatically.

By default, generates config for both Claude Code and Cursor.
Use --claude or --cursor to generate for only one.`,
	RunE: runMCPInit,
}

func init() {
	mcpInitCmd.Flags().Bool("claude", false, "generate .mcp.json only (Claude Code)")
	mcpInitCmd.Flags().Bool("cursor", false, "generate .cursor/mcp.json only (Cursor)")
	mcpInitCmd.Flags().Bool("force", false, "overwrite existing config files")
	mcpCmd.AddCommand(mcpInitCmd)
}

type mcpConfig struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
}

type mcpServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func runMCPInit(cmd *cobra.Command, args []string) error {
	claudeOnly, _ := cmd.Flags().GetBool("claude")
	cursorOnly, _ := cmd.Flags().GetBool("cursor")
	force, _ := cmd.Flags().GetBool("force")

	// Default: both
	writeClaude := !cursorOnly
	writeCursor := !claudeOnly

	cfg := mcpConfig{
		MCPServers: map[string]mcpServerConfig{
			"kruda": {
				Command: "kruda",
				Args:    []string{"mcp"},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	wrote := 0

	if writeClaude {
		path := ".mcp.json"
		if err := writeMCPConfig(path, data, force); err != nil {
			return err
		}
		fmt.Printf("  Created %s (Claude Code)\n", path)
		wrote++
	}

	if writeCursor {
		dir := ".cursor"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		path := filepath.Join(dir, "mcp.json")
		if err := writeMCPConfig(path, data, force); err != nil {
			return err
		}
		fmt.Printf("  Created %s (Cursor)\n", path)
		wrote++
	}

	if wrote > 0 {
		fmt.Println()
		fmt.Println("  Restart your AI assistant to load the Kruda MCP server.")
		fmt.Println()
		fmt.Println("  Available tools:")
		fmt.Println("    kruda_new            Scaffold a new project")
		fmt.Println("    kruda_add_handler    Generate a route handler")
		fmt.Println("    kruda_add_resource   Generate a CRUD resource service")
		fmt.Println("    kruda_list_routes    List all registered routes")
		fmt.Println("    kruda_suggest_wing   Suggest Wing Feather hints")
		fmt.Println("    kruda_docs           Look up Kruda docs and examples")
		fmt.Println()
		fmt.Println("  Verify: kruda mcp --test")
	}

	return nil
}

func writeMCPConfig(path string, data []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return os.WriteFile(path, data, 0o644)
}
