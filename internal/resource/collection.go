package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nickcecere/btcx/internal/config"
)

// Collection represents a set of resources grouped together for searching
type Collection struct {
	// Name is the unique identifier for this collection (e.g., "svelte+react")
	Name string

	// Path is the directory containing symlinks to resources
	Path string

	// Resources are the resources in this collection
	Resources []CollectionResource
}

// CollectionResource represents a resource within a collection
type CollectionResource struct {
	// Name is the resource name
	Name string

	// Path is the actual path to the resource (resolved from symlink)
	Path string

	// Notes are hints for the AI about this resource
	Notes string
}

// EnsureCollection ensures a collection exists with the given resources
func (m *Manager) EnsureCollection(ctx context.Context, resources []*config.Resource) (*Collection, error) {
	if len(resources) == 0 {
		return nil, fmt.Errorf("at least one resource is required")
	}

	// Generate collection name from sorted resource names
	names := make([]string, len(resources))
	for i, r := range resources {
		names[i] = r.Name
	}
	sort.Strings(names)
	collectionName := strings.Join(names, "+")

	// Collection path
	collectionPath := filepath.Join(m.CollectionsDir(), collectionName)

	// Ensure resources are available and get their paths
	resourcePaths := make(map[string]string)
	for _, r := range resources {
		path, err := m.Ensure(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure resource %q: %w", r.Name, err)
		}

		// Get the working path (with searchPath applied)
		workingPath, err := m.GetWorkingPath(r)
		if err != nil {
			return nil, fmt.Errorf("failed to get working path for %q: %w", r.Name, err)
		}

		resourcePaths[r.Name] = workingPath
		_ = path // Silence unused variable warning
	}

	// Create collection directory
	if err := os.MkdirAll(collectionPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create collection directory: %w", err)
	}

	// Create symlinks for each resource
	collection := &Collection{
		Name:      collectionName,
		Path:      collectionPath,
		Resources: make([]CollectionResource, 0, len(resources)),
	}

	for _, r := range resources {
		linkPath := filepath.Join(collectionPath, r.Name)
		targetPath := resourcePaths[r.Name]

		// Remove existing symlink if it exists
		if _, err := os.Lstat(linkPath); err == nil {
			if err := os.Remove(linkPath); err != nil {
				return nil, fmt.Errorf("failed to remove existing symlink: %w", err)
			}
		}

		// Create symlink
		if err := os.Symlink(targetPath, linkPath); err != nil {
			return nil, fmt.Errorf("failed to create symlink: %w", err)
		}

		collection.Resources = append(collection.Resources, CollectionResource{
			Name:  r.Name,
			Path:  targetPath,
			Notes: r.Notes,
		})
	}

	return collection, nil
}

// GetCollection retrieves an existing collection by name
func (m *Manager) GetCollection(name string) (*Collection, error) {
	collectionPath := filepath.Join(m.CollectionsDir(), name)

	info, err := os.Stat(collectionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("collection %q not found", name)
		}
		return nil, fmt.Errorf("failed to stat collection: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("collection path is not a directory: %s", collectionPath)
	}

	// Read symlinks in collection
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read collection directory: %w", err)
	}

	collection := &Collection{
		Name:      name,
		Path:      collectionPath,
		Resources: make([]CollectionResource, 0, len(entries)),
	}

	for _, entry := range entries {
		linkPath := filepath.Join(collectionPath, entry.Name())

		// Resolve symlink
		targetPath, err := os.Readlink(linkPath)
		if err != nil {
			continue // Skip non-symlinks
		}

		// Make absolute if relative
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(collectionPath, targetPath)
		}

		collection.Resources = append(collection.Resources, CollectionResource{
			Name: entry.Name(),
			Path: targetPath,
		})
	}

	return collection, nil
}

// ListCollections returns the names of all collections
func (m *Manager) ListCollections() ([]string, error) {
	entries, err := os.ReadDir(m.CollectionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read collections directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// RemoveCollection removes a collection
func (m *Manager) RemoveCollection(name string) error {
	collectionPath := filepath.Join(m.CollectionsDir(), name)
	if err := os.RemoveAll(collectionPath); err != nil {
		return fmt.Errorf("failed to remove collection: %w", err)
	}
	return nil
}
