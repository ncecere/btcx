package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nickcecere/btcx/internal/config"
)

// Manager handles resource operations
type Manager struct {
	cacheDir string
}

// NewManager creates a new resource manager
func NewManager(cacheDir string) *Manager {
	return &Manager{
		cacheDir: cacheDir,
	}
}

// ResourcesDir returns the directory where resources are cached
func (m *Manager) ResourcesDir() string {
	return filepath.Join(m.cacheDir, "resources")
}

// CollectionsDir returns the directory where collections are stored
func (m *Manager) CollectionsDir() string {
	return filepath.Join(m.cacheDir, "collections")
}

// ResourcePath returns the path to a specific resource
func (m *Manager) ResourcePath(name string) string {
	return filepath.Join(m.ResourcesDir(), name)
}

// Ensure ensures a resource is available locally
// For git resources, it clones or pulls the repository
// For local resources, it validates the path exists
func (m *Manager) Ensure(ctx context.Context, r *config.Resource) (string, error) {
	switch r.Type {
	case config.ResourceTypeGit:
		return m.ensureGit(ctx, r)
	case config.ResourceTypeLocal:
		return m.ensureLocal(r)
	default:
		return "", fmt.Errorf("unknown resource type: %s", r.Type)
	}
}

// EnsureAll ensures all resources are available locally
func (m *Manager) EnsureAll(ctx context.Context, resources []config.Resource) error {
	for _, r := range resources {
		if _, err := m.Ensure(ctx, &r); err != nil {
			return fmt.Errorf("failed to ensure resource %q: %w", r.Name, err)
		}
	}
	return nil
}

// GetWorkingPath returns the working path for a resource
// This is the searchPath if specified, otherwise the root of the resource
func (m *Manager) GetWorkingPath(r *config.Resource) (string, error) {
	var basePath string

	switch r.Type {
	case config.ResourceTypeGit:
		basePath = m.ResourcePath(r.Name)
	case config.ResourceTypeLocal:
		basePath = r.Path
	default:
		return "", fmt.Errorf("unknown resource type: %s", r.Type)
	}

	// Expand ~ in path
	if len(basePath) > 0 && basePath[0] == '~' {
		homeDir, _ := os.UserHomeDir()
		basePath = filepath.Join(homeDir, basePath[1:])
	}

	// If searchPath is specified, append it
	if r.SearchPath != "" {
		basePath = filepath.Join(basePath, r.SearchPath)
	}

	// Verify path exists
	if _, err := os.Stat(basePath); err != nil {
		return "", fmt.Errorf("path does not exist: %s", basePath)
	}

	return basePath, nil
}

// Clear removes a cached resource
func (m *Manager) Clear(name string) error {
	path := m.ResourcePath(name)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove resource: %w", err)
	}
	return nil
}

// ClearAll removes all cached resources
func (m *Manager) ClearAll() error {
	if err := os.RemoveAll(m.ResourcesDir()); err != nil {
		return fmt.Errorf("failed to remove resources directory: %w", err)
	}
	if err := os.RemoveAll(m.CollectionsDir()); err != nil {
		return fmt.Errorf("failed to remove collections directory: %w", err)
	}
	return nil
}

// List returns the names of all cached resources
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.ResourcesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read resources directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
