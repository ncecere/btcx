package search

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ripgrepAvailable caches whether ripgrep is available
var ripgrepAvailable *bool

// RipgrepAvailable checks if ripgrep (rg) is available in PATH
func RipgrepAvailable() bool {
	if ripgrepAvailable != nil {
		return *ripgrepAvailable
	}

	_, err := exec.LookPath("rg")
	available := err == nil
	ripgrepAvailable = &available
	return available
}

// RipgrepGrep searches for a pattern using ripgrep
func RipgrepGrep(root, pattern string, opts GrepOptions) ([]Match, error) {
	if opts.MaxMatches == 0 {
		opts.MaxMatches = DefaultGrepOptions().MaxMatches
	}
	if opts.MaxLineLength == 0 {
		opts.MaxLineLength = DefaultGrepOptions().MaxLineLength
	}

	// Build ripgrep command
	args := []string{
		"-n",                        // Line numbers
		"-H",                        // Include filename
		"--hidden",                  // Search hidden files
		"--follow",                  // Follow symlinks
		"--field-match-separator=|", // Use | as separator for easy parsing
		"--no-heading",              // Don't group by file
		"--color=never",             // No color codes
		"--regexp", pattern,
	}

	// Add include pattern if specified
	if opts.Include != "" {
		args = append(args, "--glob", opts.Include)
	}

	// Add root path
	args = append(args, root)

	cmd := exec.Command("rg", args...)
	cmd.Dir = root

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var matches []Match
	scanner := bufio.NewScanner(stdout)

	// Parse ripgrep output: filepath|linenum|content
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}

		filePath := parts[0]
		lineNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		lineText := parts[2]

		// Truncate long lines
		if len(lineText) > opts.MaxLineLength {
			lineText = lineText[:opts.MaxLineLength] + "..."
		}

		// Get file modification time
		var modTime time.Time
		if info, err := os.Stat(filePath); err == nil {
			modTime = info.ModTime()
		}

		matches = append(matches, Match{
			Path:     filePath,
			LineNum:  lineNum,
			LineText: lineText,
			ModTime:  modTime,
		})

		// Check if we've reached max matches
		if len(matches) >= opts.MaxMatches {
			break
		}
	}

	// Wait for command to finish (ignore exit code - rg returns 1 for no matches)
	cmd.Wait()

	// Sort by modification time (newest first)
	sortMatchesByTime(matches)

	return matches, nil
}

// RipgrepGlob finds files matching a pattern using ripgrep --files
func RipgrepGlob(root, pattern string, opts GlobOptions) ([]FileInfo, error) {
	if opts.MaxFiles == 0 {
		opts.MaxFiles = DefaultGlobOptions().MaxFiles
	}

	// Build ripgrep command for listing files
	args := []string{
		"--files",  // List files only
		"--hidden", // Include hidden files
		"--follow", // Follow symlinks
		"--glob", pattern,
	}

	// Add root path
	args = append(args, root)

	cmd := exec.Command("rg", args...)
	cmd.Dir = root

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var files []FileInfo
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		filePath := scanner.Text()
		if filePath == "" {
			continue
		}

		// Make path absolute if not already
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(root, filePath)
		}

		// Get file modification time
		var modTime time.Time
		if info, err := os.Stat(filePath); err == nil {
			modTime = info.ModTime()
		}

		files = append(files, FileInfo{
			Path:    filePath,
			ModTime: modTime,
		})

		// Check if we've reached max files
		if len(files) >= opts.MaxFiles {
			break
		}
	}

	// Wait for command to finish
	cmd.Wait()

	// Sort by modification time (newest first)
	sortFilesByTime(files)

	return files, nil
}

// sortMatchesByTime sorts matches by modification time (newest first)
func sortMatchesByTime(matches []Match) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].ModTime.After(matches[i].ModTime) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// sortFilesByTime sorts files by modification time (newest first)
func sortFilesByTime(files []FileInfo) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].ModTime.After(files[i].ModTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}
