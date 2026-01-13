package main

import (
	"context"
	"fmt"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/resource"
	"github.com/spf13/cobra"
)

func resourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resources",
		Aliases: []string{"res"},
		Short:   "Manage resources",
		Long:    `Add, remove, and list resources for searching.`,
	}

	cmd.AddCommand(resourcesListCmd())
	cmd.AddCommand(resourcesAddCmd())
	cmd.AddCommand(resourcesRemoveCmd())
	cmd.AddCommand(resourcesFetchCmd())

	return cmd
}

func resourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Resources) == 0 {
				fmt.Println("No resources configured.")
				fmt.Println("\nAdd a resource with: btcx resources add --help")
				return nil
			}

			fmt.Printf("Configured resources (%d):\n\n", len(cfg.Resources))
			for _, r := range cfg.Resources {
				fmt.Printf("  %s\n", r.Name)
				fmt.Printf("    Type: %s\n", r.Type)
				if r.Type == config.ResourceTypeGit {
					fmt.Printf("    URL:  %s\n", r.URL)
					if r.Branch != "" {
						fmt.Printf("    Branch: %s\n", r.Branch)
					}
				} else {
					fmt.Printf("    Path: %s\n", r.Path)
				}
				if r.SearchPath != "" {
					fmt.Printf("    Search path: %s\n", r.SearchPath)
				}
				if r.Notes != "" {
					fmt.Printf("    Notes: %s\n", r.Notes)
				}
				fmt.Println()
			}

			return nil
		},
	}
}

func resourcesAddCmd() *cobra.Command {
	var name, resType, url, branch, path, searchPath, notes string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new resource",
		Example: `  # Add a git resource
  btcx resources add -n svelte -t git -u https://github.com/sveltejs/svelte.dev --branch main --search-path apps/svelte.dev

  # Add a local resource
  btcx resources add -n myproject -t local -p /path/to/project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("name is required (-n flag)")
			}
			if resType == "" {
				return fmt.Errorf("type is required (-t flag: git or local)")
			}

			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			r := config.Resource{
				Name:       name,
				Type:       config.ResourceType(resType),
				URL:        url,
				Branch:     branch,
				Path:       path,
				SearchPath: searchPath,
				Notes:      notes,
			}

			if err := cfg.AddResource(r); err != nil {
				return err
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid resource: %w", err)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Added resource: %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Resource name")
	cmd.Flags().StringVarP(&resType, "type", "t", "", "Resource type (git or local)")
	cmd.Flags().StringVarP(&url, "url", "u", "", "Git repository URL")
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Local path")
	cmd.Flags().StringVar(&searchPath, "search-path", "", "Subdirectory to search")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes for the AI")

	return cmd
}

func resourcesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := cfg.RemoveResource(name); err != nil {
				return err
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Removed resource: %s\n", name)
			return nil
		},
	}
}

func resourcesFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch [name]",
		Short: "Fetch/update resources",
		Long:  `Fetch or update resources. If no name is specified, all resources are fetched.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			mgr := resource.NewManager(cfg.Cache.ResolvedPath)

			if len(args) > 0 {
				// Fetch specific resource
				name := args[0]
				r, ok := cfg.GetResource(name)
				if !ok {
					return fmt.Errorf("resource %q not found", name)
				}

				fmt.Printf("Fetching %s...\n", name)
				if _, err := mgr.Ensure(context.Background(), r); err != nil {
					return fmt.Errorf("failed to fetch %s: %w", name, err)
				}
				fmt.Printf("Done.\n")
			} else {
				// Fetch all resources
				for _, r := range cfg.Resources {
					fmt.Printf("Fetching %s...\n", r.Name)
					if _, err := mgr.Ensure(context.Background(), &r); err != nil {
						fmt.Printf("  Error: %v\n", err)
						continue
					}
				}
				fmt.Printf("Done.\n")
			}

			return nil
		},
	}
}
