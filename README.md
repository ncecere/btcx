# btcx

A documentation search agent powered by AI. Ask questions about codebases and get answers with references to the actual source code.

## Features

- **Multi-Provider Support**: Ollama (local), Anthropic, OpenAI, Google, and OpenAI-compatible APIs
- **Multiple Models**: Configure multiple AI models and switch between them with `--model` flag
- **Git & Local Resources**: Search git repositories or local directories
- **Agentic Search**: AI uses tools (grep, glob, read, list) to search codebases before answering
- **Interactive TUI**: Chat interface with markdown rendering
- **JSON Output**: Structured output for programmatic use and AI agent integration
- **Thread History**: Conversations are saved and can be continued

## Installation

### From Source

```bash
go install github.com/nickcecere/btcx/cmd/btcx@latest
```

Or clone and build:

```bash
git clone https://github.com/nickcecere/btcx.git
cd btcx
go build -o /usr/local/bin/btcx ./cmd/btcx/
```

### Prerequisites

- Go 1.21 or later
- One of the supported AI providers:
  - [Ollama](https://ollama.ai/) (free, local)
  - Anthropic API key
  - OpenAI API key
  - Google AI API key

## Quick Start

1. **Create config file**:

```bash
mkdir -p ~/.config/btcx
cp config.example.yaml ~/.config/btcx/config.yaml
```

2. **Edit config** to set your preferred model and add resources:

```yaml
defaultModel: llama

models:
  - name: llama
    provider: ollama
    model: llama3.2

resources:
  - name: cobra
    type: git
    url: https://github.com/spf13/cobra
    branch: main
```

3. **Fetch resources**:

```bash
btcx resources fetch cobra
```

4. **Ask questions**:

```bash
btcx ask -r cobra -q "What is Cobra and how do I create a subcommand?"
```

## Configuration

Config file location: `~/.config/btcx/config.yaml`

### Models

Define multiple AI models and set a default:

```yaml
defaultModel: claude

models:
  # Local Ollama
  - name: llama
    provider: ollama
    model: llama3.2

  # Anthropic Claude
  - name: claude
    provider: anthropic
    model: claude-sonnet-4-20250514
    # apiKey: sk-ant-...  # or set ANTHROPIC_API_KEY env var

  # OpenAI
  - name: gpt4
    provider: openai
    model: gpt-4o
    # apiKey: sk-...  # or set OPENAI_API_KEY env var

  # Google Gemini
  - name: gemini
    provider: google
    model: gemini-2.0-flash

  # OpenAI-compatible (Groq, Together, LM Studio, etc.)
  - name: groq
    provider: openai-compatible
    model: llama-3.1-70b-versatile
    baseUrl: https://api.groq.com/openai/v1
    apiKey: your-api-key
```

### Resources

Resources are the codebases you want to search:

```yaml
resources:
  # Git repository
  - name: react
    type: git
    url: https://github.com/reactjs/react.dev
    branch: main
    searchPath: src/content  # optional: limit search to subdirectory
    notes: React documentation  # optional: hints for the AI

  # Local directory
  - name: myproject
    type: local
    path: ~/Projects/myproject
    searchPath: src
```

### Output Settings

Control CLI output behavior:

```yaml
output:
  spinner: true      # animated spinner (disable for CI/agents)
  markdown: true     # render markdown in output
  showUsage: true    # show token usage after response
```

### Environment Variables

API keys can be set via environment variables:

- `ANTHROPIC_API_KEY` - Anthropic
- `OPENAI_API_KEY` - OpenAI and OpenAI-compatible
- `GOOGLE_API_KEY` - Google AI
- `BTCX_CONFIG` - Override config file path

## Usage

### Ask Questions

```bash
# Basic usage
btcx ask -r <resource> -q "your question"

# Multiple resources
btcx ask -r react -r typescript -q "How do I type React components?"

# Use specific model
btcx ask -r cobra -q "What is Cobra?" -m claude

# Continue previous conversation
btcx ask -r cobra -q "Can you explain more?" --continue
```

### Output Formats

```bash
# Default: styled markdown output with spinner
btcx ask -r cobra -q "What is Cobra?"

# No spinner (for CI/agents/scripts)
btcx ask -r cobra -q "What is Cobra?" --no-spinner

# JSON output (for programmatic use)
btcx ask -r cobra -q "What is Cobra?" --output json
```

JSON output format:

```json
{
  "answer": "Cobra is a Go library for creating CLI applications...",
  "tools_used": [
    {"name": "grep", "count": 2},
    {"name": "read", "count": 1}
  ],
  "usage": {
    "input_tokens": 1523,
    "output_tokens": 456
  },
  "model": {
    "name": "claude",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514"
  },
  "resources": ["cobra"]
}
```

### Interactive TUI

```bash
# Start TUI with resource(s)
btcx tui -r cobra

# Use specific model
btcx tui -r cobra -m gpt4
```

### Manage Resources

```bash
# List configured resources
btcx resources list

# Add a git resource
btcx resources add -n svelte -t git -u https://github.com/sveltejs/svelte.dev --branch main

# Add a local resource
btcx resources add -n myproject -t local -p ~/Projects/myproject

# Fetch/clone a resource
btcx resources fetch svelte

# Remove a resource
btcx resources remove svelte
```

### Manage Models

```bash
# List configured models
btcx models list
```

Output:

```
Configured models (3):

* claude (default)
      Provider: anthropic
      Model:    claude-sonnet-4-20250514

  llama
      Provider: ollama
      Model:    llama3.2

  gpt4
      Provider: openai
      Model:    gpt-4o
```

### Manage Cache

```bash
# Show cache path
btcx cache path

# List cached resources
btcx cache list

# Clear cache for a resource
btcx cache clear -r cobra

# Clear all cache
btcx cache clear
```

### Manage Threads

```bash
# List conversation threads
btcx threads list

# Show a thread
btcx threads show <thread-id>

# Delete a thread
btcx threads delete <thread-id>

# Clear all threads
btcx threads clear
```

### Configuration Commands

```bash
# Show current config
btcx config show

# Show config file path
btcx config path

# Set a config value
btcx config set provider ollama
btcx config set model llama3.2
```

## How It Works

1. **You ask a question** about a codebase
2. **btcx creates an AI agent** with tools for searching:
   - `grep` - Search file contents with regex
   - `glob` - Find files by pattern
   - `read` - Read file contents
   - `list` - List directory contents
3. **The AI searches the codebase** using these tools to find relevant information
4. **The AI synthesizes an answer** based on what it found in the actual source code

This agentic approach means the AI doesn't hallucinate - it bases answers on real code it found.

## Project Structure

```
btcx/
├── cmd/btcx/           # CLI commands
│   ├── main.go         # Entry point
│   ├── ask.go          # Ask command
│   ├── tui_cmd.go      # TUI command
│   ├── config.go       # Config commands
│   ├── resources.go    # Resource commands
│   ├── models.go       # Models commands
│   ├── cache.go        # Cache commands
│   └── threads.go      # Thread commands
├── internal/
│   ├── config/         # Configuration loading
│   ├── provider/       # AI provider implementations
│   ├── agent/          # Agentic loop and system prompt
│   ├── tool/           # Tool implementations (grep, glob, etc.)
│   ├── resource/       # Resource management (git clone, local)
│   ├── storage/        # Thread persistence
│   ├── tui/            # Terminal UI (Bubble Tea)
│   └── ui/             # UI helpers (spinner, styles, markdown)
├── config.example.yaml # Example configuration
└── README.md           # This file
```

## Supported Providers

| Provider | Models | API Key Env Var |
|----------|--------|-----------------|
| Ollama | llama3.2, mistral, codellama, qwen2.5-coder, etc. | (none needed) |
| Anthropic | claude-sonnet-4-20250514, claude-haiku-4-5, claude-opus-4 | `ANTHROPIC_API_KEY` |
| OpenAI | gpt-4o, gpt-4o-mini, gpt-4-turbo | `OPENAI_API_KEY` |
| Google | gemini-2.0-flash, gemini-1.5-pro | `GOOGLE_API_KEY` |
| OpenAI-Compatible | Any (Together, Groq, LM Studio, etc.) | `OPENAI_API_KEY` |

## Tips

### For AI Coding Agents

If using btcx from another AI agent (like Claude Code or Cursor), use these flags:

```bash
btcx ask -r myresource -q "question" --no-spinner --output json
```

This disables the animated spinner (which pollutes output) and returns structured JSON.

### For CI/CD

Set `output.spinner: false` in your config or use `--no-spinner`:

```bash
btcx ask -r docs -q "What are the API endpoints?" --no-spinner
```

### Performance

- Use `searchPath` in resources to limit searches to relevant directories
- Add `notes` to resources to give the AI hints about what to look for
- Smaller models (llama3.2, claude-haiku) are faster but may need more tool calls

## License

MIT
