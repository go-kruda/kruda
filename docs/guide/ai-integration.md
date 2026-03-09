# AI Integration

Kruda includes a built-in MCP (Model Context Protocol) server that enables AI coding assistants to scaffold, generate code, and analyze your Kruda project.

## Setup

### Install CLI

```bash
go install github.com/go-kruda/kruda/cmd/kruda@latest
```

### New Projects

`kruda new` includes `.mcp.json` automatically:

```bash
kruda new myapp
cd myapp
# Open in Claude Code or Cursor — MCP tools ready to use
```

### Existing Projects

```bash
kruda mcp init        # generates .mcp.json + .cursor/mcp.json
kruda mcp --test      # verify it works
```

## Available Tools

| Tool | Description |
|------|-------------|
| `kruda_new` | Scaffold a new project (minimal, api, fullstack) |
| `kruda_add_handler` | Generate a route handler with optional typed `C[T]` pattern |
| `kruda_add_resource` | Generate a CRUD `ResourceService[T, string]` implementation |
| `kruda_list_routes` | Scan Go source files and list all registered routes |
| `kruda_suggest_wing` | Suggest Wing Feather hints for routes |
| `kruda_docs` | Look up Kruda docs and code examples by topic |

## How It Works

The MCP server runs locally on your machine as a stdio process. Your AI assistant communicates with it via JSON-RPC:

```
AI Assistant (Claude Code / Cursor)
       ↓ stdio JSON-RPC
   kruda mcp
       ↓
   reads/writes files in your project
```

No data leaves your machine. No remote server required.

## Configuration

`.mcp.json` (Claude Code):

```json
{
  "mcpServers": {
    "kruda": {
      "command": "kruda",
      "args": ["mcp"]
    }
  }
}
```

`.cursor/mcp.json` (Cursor): same format.

## Why AI-Friendly?

Kruda's typed API makes AI code generation more reliable:

- **Typed handlers** — AI generates a struct, gets compile-time validation for free
- **Auto CRUD** — AI says "create CRUD for Product" → one line of code
- **Struct tags** — validation rules are explicit, not hidden in handler logic
- **21 examples** — AI reads patterns and generates consistent code
