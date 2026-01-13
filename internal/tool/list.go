package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const listDescription = `Lists files and directories in a given path.
Returns the contents of a directory with file/folder indicators.
Use this tool to explore the structure of a codebase.`

// ListTool lists directory contents
type ListTool struct {
	workingDir string
}

// NewListTool creates a new list tool
func NewListTool(workingDir string) *ListTool {
	return &ListTool{workingDir: workingDir}
}

// Name returns the tool name
func (t *ListTool) Name() string {
	return "list"
}

// Description returns the tool description
func (t *ListTool) Description() string {
	return listDescription
}

// Parameters returns the JSON schema for the tool parameters
func (t *ListTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory path to list. Defaults to the current working directory.",
			},
		},
		"required": []string{},
	}
}

// listArgs are the arguments for the list tool
type listArgs struct {
	Path string `json:"path"`
}

// Execute runs the list tool
func (t *ListTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a listArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Resolve path
	listPath := t.workingDir
	if a.Path != "" {
		if filepath.IsAbs(a.Path) {
			listPath = a.Path
		} else {
			listPath = filepath.Join(t.workingDir, a.Path)
		}
	}

	// Check if directory exists
	info, err := os.Stat(listPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", listPath)
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", listPath)
	}

	// Read directory contents
	entries, err := os.ReadDir(listPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Separate directories and files
	var dirs []string
	var files []string

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files/directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}

	// Sort alphabetically
	sort.Strings(dirs)
	sort.Strings(files)

	// Format output
	var output strings.Builder

	relPath, _ := filepath.Rel(t.workingDir, listPath)
	if relPath == "" || relPath == "." {
		relPath = filepath.Base(listPath)
	}

	output.WriteString(fmt.Sprintf("%s/\n", relPath))

	// List directories first
	for _, dir := range dirs {
		output.WriteString(fmt.Sprintf("  %s\n", dir))
	}

	// Then files
	for _, file := range files {
		output.WriteString(fmt.Sprintf("  %s\n", file))
	}

	if len(dirs) == 0 && len(files) == 0 {
		output.WriteString("  (empty directory)\n")
	}

	return &Result{
		Title:  relPath,
		Output: output.String(),
		Metadata: map[string]interface{}{
			"directories": len(dirs),
			"files":       len(files),
		},
	}, nil
}
