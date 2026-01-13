package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nickcecere/btcx/internal/search"
)

const grepDescription = `Fast content search tool that works with any codebase size.
Searches file contents using regular expressions.
Supports full regex syntax (e.g., "log.*Error", "function\s+\w+").
Filter files by pattern with the include parameter (e.g., "*.js", "*.{ts,tsx}").
Returns file paths and line numbers with matches, sorted by modification time.
Use this tool when you need to find files containing specific patterns.`

// GrepTool searches file contents using regex
type GrepTool struct {
	workingDir string
}

// NewGrepTool creates a new grep tool
func NewGrepTool(workingDir string) *GrepTool {
	return &GrepTool{workingDir: workingDir}
}

// Name returns the tool name
func (t *GrepTool) Name() string {
	return "grep"
}

// Description returns the tool description
func (t *GrepTool) Description() string {
	return grepDescription
}

// Parameters returns the JSON schema for the tool parameters
func (t *GrepTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The regex pattern to search for in file contents",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory to search in. Defaults to the current working directory.",
			},
			"include": map[string]interface{}{
				"type":        "string",
				"description": `File pattern to include in the search (e.g., "*.js", "*.{ts,tsx}")`,
			},
		},
		"required": []string{"pattern"},
	}
}

// grepArgs are the arguments for the grep tool
type grepArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

// Execute runs the grep tool
func (t *GrepTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a grepArgs
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
	opts := search.GrepOptions{
		Include:       a.Include,
		MaxMatches:    100,
		MaxLineLength: 2000,
	}

	matches, err := search.Grep(searchPath, a.Pattern, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(matches) == 0 {
		return &Result{
			Title:  a.Pattern,
			Output: "No files found",
			Metadata: map[string]interface{}{
				"matches":   0,
				"truncated": false,
			},
		}, nil
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d matches\n", len(matches)))

	currentFile := ""
	for _, match := range matches {
		if currentFile != match.Path {
			if currentFile != "" {
				output.WriteString("\n")
			}
			relPath, _ := filepath.Rel(t.workingDir, match.Path)
			if relPath == "" {
				relPath = match.Path
			}
			currentFile = match.Path
			output.WriteString(fmt.Sprintf("%s:\n", relPath))
		}
		output.WriteString(fmt.Sprintf("  Line %d: %s\n", match.LineNum, match.LineText))
	}

	truncated := len(matches) >= opts.MaxMatches
	if truncated {
		output.WriteString("\n(Results are truncated. Consider using a more specific path or pattern.)")
	}

	return &Result{
		Title:  a.Pattern,
		Output: output.String(),
		Metadata: map[string]interface{}{
			"matches":   len(matches),
			"truncated": truncated,
		},
	}, nil
}
