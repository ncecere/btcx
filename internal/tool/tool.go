package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is the interface that all tools must implement
type Tool interface {
	// Name returns the tool's unique identifier
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// Parameters returns the JSON schema for the tool's parameters
	Parameters() map[string]interface{}

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args json.RawMessage) (*Result, error)
}

// Result is the result of a tool execution
type Result struct {
	// Title is a short title for the result
	Title string `json:"title"`

	// Output is the main text output
	Output string `json:"output"`

	// Metadata contains additional structured data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Registry holds all available tools
type Registry struct {
	tools     map[string]Tool
	outputDir string
	threadID  string
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// SetOutputDir sets the directory for truncated outputs
func (r *Registry) SetOutputDir(dir string) {
	r.outputDir = dir
}

// SetThreadID sets the current thread ID for organizing outputs
func (r *Registry) SetThreadID(threadID string) {
	r.threadID = threadID
}

// GetTruncationConfig returns the truncation configuration
func (r *Registry) GetTruncationConfig(toolName string) TruncationConfig {
	return TruncationConfig{
		OutputDir: r.outputDir,
		ThreadID:  r.threadID,
		ToolName:  toolName,
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Execute runs a tool by name
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (*Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool %q not found. Available tools: grep, glob, read, list", name)
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		return nil, err
	}

	// Apply truncation if output is too large
	if r.outputDir != "" && result != nil {
		truncCfg := r.GetTruncationConfig(name)
		truncResult, truncErr := TruncateOutput(result.Output, truncCfg)
		if truncErr == nil && truncResult.Truncated {
			result.Output = truncResult.Content
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["truncated"] = true
			if truncResult.OutputPath != "" {
				result.Metadata["outputPath"] = truncResult.OutputPath
			}
		}
	}

	return result, nil
}

// ToOpenAITools converts the registry to OpenAI-compatible tool definitions
func (r *Registry) ToOpenAITools() []map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Parameters(),
			},
		})
	}
	return tools
}

// ToAnthropicTools converts the registry to Anthropic-compatible tool definitions
func (r *Registry) ToAnthropicTools() []map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, map[string]interface{}{
			"name":         tool.Name(),
			"description":  tool.Description(),
			"input_schema": tool.Parameters(),
		})
	}
	return tools
}

// DefaultRegistry creates a registry with all default tools
func DefaultRegistry(workingDir string) *Registry {
	registry := NewRegistry()
	registry.Register(NewGrepTool(workingDir))
	registry.Register(NewGlobTool(workingDir))
	registry.Register(NewReadTool(workingDir))
	registry.Register(NewListTool(workingDir))
	return registry
}
