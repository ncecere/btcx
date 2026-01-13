package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nickcecere/btcx/internal/search"
)

const globDescription = `Fast file pattern matching tool that works with any codebase size.
Supports glob patterns like "**/*.js" or "src/**/*.ts".
Returns matching file paths sorted by modification time.
Use this tool when you need to find files by name patterns.`

// GlobTool finds files matching a pattern
type GlobTool struct {
	workingDir string
}

// NewGlobTool creates a new glob tool
func NewGlobTool(workingDir string) *GlobTool {
	return &GlobTool{workingDir: workingDir}
}

// Name returns the tool name
func (t *GlobTool) Name() string {
	return "glob"
}

// Description returns the tool description
func (t *GlobTool) Description() string {
	return globDescription
}

// Parameters returns the JSON schema for the tool parameters
func (t *GlobTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The glob pattern to match files against",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory to search in. Defaults to the current working directory.",
			},
		},
		"required": []string{"pattern"},
	}
}

// globArgs are the arguments for the glob tool
type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// Execute runs the glob tool
func (t *GlobTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a globArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if a.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	// Resolve search path
	searchPath := t.workingDir
	if a.Path != "" {
		if filepath.IsAbs(a.Path) {
			searchPath = a.Path
		} else {
			searchPath = filepath.Join(t.workingDir, a.Path)
		}
	}

	// Run search
	opts := search.GlobOptions{
		MaxFiles: 100,
	}

	files, err := search.Glob(searchPath, a.Pattern, opts)
	if err != nil {
		return nil, fmt.Errorf("glob failed: %w", err)
	}

	if len(files) == 0 {
		return &Result{
			Title:  filepath.Base(searchPath),
			Output: "No files found",
			Metadata: map[string]interface{}{
				"count":     0,
				"truncated": false,
			},
		}, nil
	}

	// Format output
	var output strings.Builder
	for _, file := range files {
		relPath, _ := filepath.Rel(t.workingDir, file.Path)
		if relPath == "" {
			relPath = file.Path
		}
		output.WriteString(relPath)
		output.WriteString("\n")
	}

	truncated := len(files) >= opts.MaxFiles
	if truncated {
		output.WriteString("\n(Results are truncated. Consider using a more specific path or pattern.)")
	}

	return &Result{
		Title:  filepath.Base(searchPath),
		Output: output.String(),
		Metadata: map[string]interface{}{
			"count":     len(files),
			"truncated": truncated,
		},
	}, nil
}
