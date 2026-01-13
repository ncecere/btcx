package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const readDescription = `Reads a file from the local filesystem.
You can access any file directly by using this tool.
By default, it reads up to 2000 lines starting from the beginning of the file.
You can optionally specify a line offset and limit for long files.
Any lines longer than 2000 characters will be truncated.
Results are returned with line numbers starting at 1.`

const (
	defaultReadLimit = 2000
	maxLineLength    = 2000
	maxBytes         = 50 * 1024 // 50KB
)

// ReadTool reads file contents
type ReadTool struct {
	workingDir string
}

// NewReadTool creates a new read tool
func NewReadTool(workingDir string) *ReadTool {
	return &ReadTool{workingDir: workingDir}
}

// Name returns the tool name
func (t *ReadTool) Name() string {
	return "read"
}

// Description returns the tool description
func (t *ReadTool) Description() string {
	return readDescription
}

// Parameters returns the JSON schema for the tool parameters
func (t *ReadTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filePath": map[string]interface{}{
				"type":        "string",
				"description": "The path to the file to read",
			},
			"offset": map[string]interface{}{
				"type":        "number",
				"description": "The line number to start reading from (0-based)",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "The number of lines to read (defaults to 2000)",
			},
		},
		"required": []string{"filePath"},
	}
}

// readArgs are the arguments for the read tool
type readArgs struct {
	FilePath string `json:"filePath"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

// Execute runs the read tool
func (t *ReadTool) Execute(ctx context.Context, args json.RawMessage) (*Result, error) {
	var a readArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if a.FilePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}

	// Resolve file path
	filePath := a.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workingDir, filePath)
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to suggest similar files
			suggestions := SuggestSimilarFiles(filePath, 3)
			if len(suggestions) > 0 {
				suggestionList := strings.Join(suggestions, "\n  ")
				return nil, fmt.Errorf("file not found: %s\n\nDid you mean one of these?\n  %s", filePath, suggestionList)
			}
			return nil, fmt.Errorf("file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Check for binary file by extension first
	if isBinaryExtension(filePath) {
		return nil, fmt.Errorf("cannot read binary file: %s", filePath)
	}

	// Check file content for binary data
	isBinary, _ := IsBinaryContent(filePath)
	if isBinary {
		return nil, fmt.Errorf("cannot read binary file: %s", filePath)
	}

	// Set defaults
	limit := a.Limit
	if limit == 0 {
		limit = defaultReadLimit
	}
	offset := a.Offset

	// Read file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read lines
	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0
	bytesRead := 0
	truncatedByBytes := false

	for scanner.Scan() {
		lineNum++

		// Skip lines before offset
		if lineNum <= offset {
			continue
		}

		// Check if we've read enough lines
		if len(lines) >= limit {
			break
		}

		line := scanner.Text()

		// Truncate long lines
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + "..."
		}

		// Check bytes limit
		lineBytes := len(line) + 1 // +1 for newline
		if bytesRead+lineBytes > maxBytes {
			truncatedByBytes = true
			break
		}

		lines = append(lines, line)
		bytesRead += lineBytes
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Format output with line numbers
	var output strings.Builder
	output.WriteString("<file>\n")

	for i, line := range lines {
		lineNumber := offset + i + 1
		output.WriteString(fmt.Sprintf("%05d| %s\n", lineNumber, line))
	}

	// Add truncation message
	lastReadLine := offset + len(lines)
	hasMoreLines := lineNum > lastReadLine

	if truncatedByBytes {
		output.WriteString(fmt.Sprintf("\n(Output truncated at %d bytes. Use 'offset' parameter to read beyond line %d)", maxBytes, lastReadLine))
	} else if hasMoreLines {
		output.WriteString(fmt.Sprintf("\n(File has more lines. Use 'offset' parameter to read beyond line %d)", lastReadLine))
	} else {
		output.WriteString(fmt.Sprintf("\n(End of file - total %d lines)", lineNum))
	}
	output.WriteString("\n</file>")

	relPath, _ := filepath.Rel(t.workingDir, filePath)
	if relPath == "" {
		relPath = filePath
	}

	return &Result{
		Title:  relPath,
		Output: output.String(),
		Metadata: map[string]interface{}{
			"truncated": truncatedByBytes || hasMoreLines,
		},
	}, nil
}

// isBinaryExtension checks if a file has a binary extension
func isBinaryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExtensions := map[string]bool{
		".zip": true, ".tar": true, ".gz": true, ".exe": true,
		".dll": true, ".so": true, ".class": true, ".jar": true,
		".war": true, ".7z": true, ".doc": true, ".docx": true,
		".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".pdf": true, ".png": true, ".jpg": true, ".jpeg": true,
		".gif": true, ".bmp": true, ".ico": true, ".mp3": true,
		".mp4": true, ".avi": true, ".mov": true, ".bin": true,
		".dat": true, ".obj": true, ".o": true, ".a": true,
		".lib": true, ".wasm": true, ".pyc": true, ".pyo": true,
	}
	return binaryExtensions[ext]
}
