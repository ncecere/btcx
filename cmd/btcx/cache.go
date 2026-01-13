package main

import (
	"fmt"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/resource"
	"github.com/spf13/cobra"
)

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the resource cache",
		Long:  `View and clear cached resources.`,
	}

	cmd.AddCommand(cacheListCmd())
	cmd.AddCommand(cacheClearCmd())
	cmd.AddCommand(cachePathCmd())

	return cmd
}

func cacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cached resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			mgr := resource.NewManager(cfg.Cache.ResolvedPath)

			resources, err := mgr.List()
			if err != nil {
				return fmt.Errorf("failed to list cache: %w", err)
			}

			if len(resources) == 0 {
				fmt.Println("Cache is empty.")
				return nil
			}

			fmt.Printf("Cached resources (%d):\n", len(resources))
			for _, r := range resources {
				fmt.Printf("  %s\n", r)
			}

			return nil
		},
	}
}

func cacheClearCmd() *cobra.Command {
	var resourceName string
	var all bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the cache",
		Example: `  btcx cache clear --all
  btcx cache clear -r svelte`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			mgr := resource.NewManager(cfg.Cache.ResolvedPath)

			if resourceName != "" {
				// Clear specific resource
				if err := mgr.Clear(resourceName); err != nil {
					return fmt.Errorf("failed to clear %s: %w", resourceName, err)
				}
				fmt.Printf("Cleared: %s\n", resourceName)
			} else if all {
				// Clear all
				if err := mgr.ClearAll(); err != nil {
					return fmt.Errorf("failed to clear cache: %w", err)
				}
				fmt.Println("Cache cleared.")
			} else {
				return fmt.Errorf("specify --all or -r <resource>")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&resourceName, "resource", "r", "", "Resource to clear")
	cmd.Flags().BoolVar(&all, "all", false, "Clear all cached resources")

	return cmd
}

func cachePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show cache directory path",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Println(cfg.Cache.ResolvedPath)
			return nil
		},
	}
}
