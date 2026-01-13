package resource

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nickcecere/btcx/internal/config"
)

// ensureLocal ensures a local resource path exists
func (m *Manager) ensureLocal(r *config.Resource) (string, error) {
	path := r.Path

	// Expand ~ in path
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	// Make absolute if relative
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	// Verify path exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("local resource path does not exist: %s", path)
		}
		return "", fmt.Errorf("failed to stat local resource: %w", err)
	}

	// Verify it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("local resource path is not a directory: %s", path)
	}

	return path, nil
}
