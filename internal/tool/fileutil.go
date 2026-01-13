package tool

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SuggestSimilarFiles finds files similar to the requested filename
// Returns up to maxSuggestions file paths that are similar to the target
func SuggestSimilarFiles(targetPath string, maxSuggestions int) []string {
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)
	baseLower := strings.ToLower(base)

	// Try to read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type suggestion struct {
		path  string
		score int // Lower is better
	}

	var suggestions []suggestion

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		nameLower := strings.ToLower(name)

		// Calculate similarity score
		score := calculateSimilarity(baseLower, nameLower)

		// Only include if reasonably similar (score < 10)
		if score < 10 {
			suggestions = append(suggestions, suggestion{
				path:  filepath.Join(dir, name),
				score: score,
			})
		}
	}

	// Sort by score (lower is better)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].score < suggestions[j].score
	})

	// Return top suggestions
	result := make([]string, 0, maxSuggestions)
	for i := 0; i < len(suggestions) && i < maxSuggestions; i++ {
		result = append(result, suggestions[i].path)
	}

	return result
}

// calculateSimilarity returns a similarity score between two strings
// Lower score means more similar
func calculateSimilarity(a, b string) int {
	// Exact match
	if a == b {
		return 0
	}

	// One contains the other
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 1
	}

	// Check if base names match (ignoring extension)
	aBase := strings.TrimSuffix(a, filepath.Ext(a))
	bBase := strings.TrimSuffix(b, filepath.Ext(b))

	if aBase == bBase {
		return 2
	}

	if strings.Contains(aBase, bBase) || strings.Contains(bBase, aBase) {
		return 3
	}

	// Levenshtein distance
	dist := levenshteinDistance(a, b)
	if dist <= 3 {
		return 4 + dist
	}

	// Check common prefix
	commonPrefix := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			commonPrefix++
		} else {
			break
		}
	}

	if commonPrefix >= 3 {
		return 8
	}

	return 100 // Not similar
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// IsBinaryContent checks if file content appears to be binary
// by examining the first few kilobytes for null bytes and non-printable characters
func IsBinaryContent(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return true, err // Assume binary if can't open
	}
	defer file.Close()

	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return true, err // Assume binary if can't read
	}

	// Check for null bytes (common in binary files)
	nonPrintable := 0
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil // Null byte = definitely binary
		}
		// Count non-printable characters (excluding common whitespace)
		if buf[i] < 9 || (buf[i] > 13 && buf[i] < 32) {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	return float64(nonPrintable)/float64(n) > 0.3, nil
}
