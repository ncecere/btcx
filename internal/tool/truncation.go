package tool

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MaxOutputBytes is the maximum size of tool output before truncation
	MaxOutputBytes = 50 * 1024 // 50KB

	// MaxOutputLines is the maximum number of lines before truncation
	MaxOutputLines = 500

	// OutputFileExpiry is how long truncated output files are kept
	OutputFileExpiry = 24 * time.Hour
)

// TruncationResult contains the result of truncating output
type TruncationResult struct {
	// Content is the (possibly truncated) output content
	Content string

	// Truncated indicates if the content was truncated
	Truncated bool

	// OutputPath is the path to the full output file (if truncated)
	OutputPath string
}

// TruncationConfig holds configuration for truncation
type TruncationConfig struct {
	// OutputDir is the directory to store truncated outputs
	OutputDir string

	// ThreadID is the current thread ID for organizing outputs
	ThreadID string

	// ToolName is the name of the tool that generated the output
	ToolName string
}

// TruncateOutput truncates output if it exceeds size limits
// Returns the truncated output and path to full output file if truncated
func TruncateOutput(output string, cfg TruncationConfig) (*TruncationResult, error) {
	// Check if truncation is needed
	lines := strings.Split(output, "\n")
	bytes := len(output)

	needsTruncation := bytes > MaxOutputBytes || len(lines) > MaxOutputLines

	if !needsTruncation {
		return &TruncationResult{
			Content:   output,
			Truncated: false,
		}, nil
	}

	// Ensure output directory exists
	threadDir := filepath.Join(cfg.OutputDir, cfg.ThreadID)
	if err := os.MkdirAll(threadDir, 0755); err != nil {
		// If we can't create the directory, return truncated content without file
		return truncateInMemory(output, lines), nil
	}

	// Generate unique filename
	filename := fmt.Sprintf("%s-%s.txt", cfg.ToolName, randomID())
	outputPath := filepath.Join(threadDir, filename)

	// Write full output to file
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		// If we can't write the file, return truncated content without file
		return truncateInMemory(output, lines), nil
	}

	// Create truncated preview
	result := truncateInMemory(output, lines)
	result.OutputPath = outputPath
	result.Content += fmt.Sprintf("\n\n[Full output saved to: %s]", outputPath)

	return result, nil
}

// truncateInMemory truncates content in memory without saving to file
func truncateInMemory(output string, lines []string) *TruncationResult {
	var truncatedContent string

	// Truncate by lines first
	if len(lines) > MaxOutputLines {
		truncatedContent = strings.Join(lines[:MaxOutputLines], "\n")
		truncatedContent += fmt.Sprintf("\n\n[Output truncated at %d lines. Total: %d lines]", MaxOutputLines, len(lines))
	} else {
		truncatedContent = output
	}

	// Then truncate by bytes if still too large
	if len(truncatedContent) > MaxOutputBytes {
		truncatedContent = truncatedContent[:MaxOutputBytes]
		truncatedContent += "\n\n[Output truncated at 50KB]"
	}

	return &TruncationResult{
		Content:   truncatedContent,
		Truncated: true,
	}
}

// CleanupOldOutputs removes output files older than OutputFileExpiry
func CleanupOldOutputs(outputDir string) error {
	if outputDir == "" {
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return nil
	}

	cutoff := time.Now().Add(-OutputFileExpiry)

	return filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories on first pass
		if d.IsDir() {
			return nil
		}

		// Check file age
		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}

		return nil
	})
}

// CleanupEmptyThreadDirs removes empty thread directories
func CleanupEmptyThreadDirs(outputDir string) error {
	if outputDir == "" {
		return nil
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		threadDir := filepath.Join(outputDir, entry.Name())
		subEntries, err := os.ReadDir(threadDir)
		if err != nil {
			continue
		}

		// Remove empty directories
		if len(subEntries) == 0 {
			os.Remove(threadDir)
		}
	}

	return nil
}

// randomID generates a short random ID for filenames
func randomID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
