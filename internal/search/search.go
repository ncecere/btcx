package search

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	gitignore "github.com/monochromegane/go-gitignore"
)

// Match represents a grep match
type Match struct {
	Path     string
	LineNum  int
	LineText string
	ModTime  time.Time
}

// FileInfo represents a file with its modification time
type FileInfo struct {
	Path    string
	ModTime time.Time
}

// GrepOptions are options for the Grep function
type GrepOptions struct {
	// Include is a glob pattern to filter files (e.g., "*.go", "*.{ts,tsx}")
	Include string

	// MaxMatches is the maximum number of matches to return
	MaxMatches int

	// MaxLineLength is the maximum line length before truncation
	MaxLineLength int
}

// DefaultGrepOptions returns the default grep options
func DefaultGrepOptions() GrepOptions {
	return GrepOptions{
		MaxMatches:    100,
		MaxLineLength: 2000,
	}
}

// Grep searches for a pattern in files under the given root directory
// Uses ripgrep if available, otherwise falls back to Go implementation
func Grep(root, pattern string, opts GrepOptions) ([]Match, error) {
	// Try ripgrep first
	if RipgrepAvailable() {
		return RipgrepGrep(root, pattern, opts)
	}

	// Fall back to Go implementation
	return goGrep(root, pattern, opts)
}

// goGrep is the pure Go implementation of grep
func goGrep(root, pattern string, opts GrepOptions) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	if opts.MaxMatches == 0 {
		opts.MaxMatches = DefaultGrepOptions().MaxMatches
	}
	if opts.MaxLineLength == 0 {
		opts.MaxLineLength = DefaultGrepOptions().MaxLineLength
	}

	// Load gitignore patterns
	ignorer := loadGitignore(root)

	var matches []Match

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path for gitignore matching
		relPath, _ := filepath.Rel(root, path)

		// Skip hidden directories (except root)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			// Check gitignore for directories
			if ignorer != nil && ignorer.Match(relPath+"/", true) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.Match(relPath, false) {
			return nil
		}

		// Apply include filter
		if opts.Include != "" {
			matched, err := doublestar.Match(opts.Include, d.Name())
			if err != nil || !matched {
				// Also try matching against relative path for patterns like "src/**/*.go"
				matched, _ = doublestar.Match(opts.Include, relPath)
				if !matched {
					return nil
				}
			}
		}

		// Skip binary files
		if isBinaryFile(path) {
			return nil
		}

		// Search file
		fileMatches, err := grepFile(path, re, opts.MaxLineLength)
		if err != nil {
			return nil // Skip errors
		}

		matches = append(matches, fileMatches...)

		// Check if we've reached max matches
		if len(matches) >= opts.MaxMatches {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, err
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ModTime.After(matches[j].ModTime)
	})

	// Truncate to max matches
	if len(matches) > opts.MaxMatches {
		matches = matches[:opts.MaxMatches]
	}

	return matches, nil
}

// grepFile searches for a pattern in a single file
func grepFile(path string, re *regexp.Regexp, maxLineLength int) ([]Match, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var matches []Match
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			// Truncate long lines
			displayLine := line
			if len(displayLine) > maxLineLength {
				displayLine = displayLine[:maxLineLength] + "..."
			}

			matches = append(matches, Match{
				Path:     path,
				LineNum:  lineNum,
				LineText: displayLine,
				ModTime:  info.ModTime(),
			})
		}
	}

	return matches, scanner.Err()
}

// GlobOptions are options for the Glob function
type GlobOptions struct {
	// MaxFiles is the maximum number of files to return
	MaxFiles int
}

// DefaultGlobOptions returns the default glob options
func DefaultGlobOptions() GlobOptions {
	return GlobOptions{
		MaxFiles: 100,
	}
}

// Glob finds files matching a pattern under the given root directory
// Uses ripgrep if available, otherwise falls back to Go implementation
func Glob(root, pattern string, opts GlobOptions) ([]FileInfo, error) {
	// Try ripgrep first
	if RipgrepAvailable() {
		return RipgrepGlob(root, pattern, opts)
	}

	// Fall back to Go implementation
	return goGlob(root, pattern, opts)
}

// goGlob is the pure Go implementation of glob
func goGlob(root, pattern string, opts GlobOptions) ([]FileInfo, error) {
	if opts.MaxFiles == 0 {
		opts.MaxFiles = DefaultGlobOptions().MaxFiles
	}

	// Load gitignore patterns
	ignorer := loadGitignore(root)

	var files []FileInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path for pattern matching
		relPath, _ := filepath.Rel(root, path)

		// Skip hidden directories (except root)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			// Check gitignore for directories
			if ignorer != nil && ignorer.Match(relPath+"/", true) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.Match(relPath, false) {
			return nil
		}

		// Match pattern against both filename and relative path
		matched, err := doublestar.Match(pattern, d.Name())
		if err != nil {
			return nil
		}
		if !matched {
			matched, _ = doublestar.Match(pattern, relPath)
		}

		if matched {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			files = append(files, FileInfo{
				Path:    path,
				ModTime: info.ModTime(),
			})
		}

		// Check if we've reached max files
		if len(files) >= opts.MaxFiles {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, err
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	// Truncate to max files
	if len(files) > opts.MaxFiles {
		files = files[:opts.MaxFiles]
	}

	return files, nil
}

// loadGitignore loads .gitignore patterns from the root directory
func loadGitignore(root string) gitignore.IgnoreMatcher {
	gitignorePath := filepath.Join(root, ".gitignore")
	ignorer, err := gitignore.NewGitIgnore(gitignorePath)
	if err != nil {
		return nil
	}
	return ignorer
}

// isBinaryFile checks if a file is likely binary
func isBinaryFile(path string) bool {
	// Check extension first
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

	if binaryExtensions[ext] {
		return true
	}

	// Check file content for null bytes
	file, err := os.Open(path)
	if err != nil {
		return true // Assume binary if can't open
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return true // Assume binary if can't read
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	return false
}
