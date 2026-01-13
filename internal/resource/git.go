package resource

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nickcecere/btcx/internal/config"
)

// ensureGit ensures a git resource is cloned and up to date
func (m *Manager) ensureGit(ctx context.Context, r *config.Resource) (string, error) {
	path := m.ResourcePath(r.Name)

	// Check if already cloned
	if _, err := os.Stat(path); err == nil {
		// Already exists, try to pull
		return path, m.pullGit(ctx, path, r)
	}

	// Clone the repository
	return path, m.cloneGit(ctx, path, r)
}

// cloneGit clones a git repository
func (m *Manager) cloneGit(ctx context.Context, path string, r *config.Resource) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(m.ResourcesDir(), 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %w", err)
	}

	opts := &git.CloneOptions{
		URL:      r.URL,
		Progress: nil, // TODO: Add progress reporting
		Depth:    1,   // Shallow clone for speed
	}

	// Set branch if specified
	if r.Branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(r.Branch)
		opts.SingleBranch = true
	}

	_, err := git.PlainCloneContext(ctx, path, false, opts)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

// pullGit pulls the latest changes for a git repository
func (m *Manager) pullGit(ctx context.Context, path string, r *config.Resource) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	opts := &git.PullOptions{
		Progress: nil, // TODO: Add progress reporting
	}

	// Set branch if specified
	if r.Branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(r.Branch)
	}

	err = worktree.PullContext(ctx, opts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull repository: %w", err)
	}

	return nil
}
