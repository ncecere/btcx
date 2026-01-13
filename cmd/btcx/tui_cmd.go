package main

import (
	"context"
	"fmt"

	"github.com/nickcecere/btcx/internal/agent"
	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/resource"
	"github.com/nickcecere/btcx/internal/tui"
	"github.com/spf13/cobra"
)

func tuiCmd() *cobra.Command {
	var resources []string
	var modelName string

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Start interactive TUI mode",
		Long:  `Start an interactive terminal UI for chatting with the AI about resources.`,
		Example: `  btcx tui -r svelte
  btcx tui -r svelte -r react
  btcx tui -r cobra -m claude`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(resources) == 0 {
				return fmt.Errorf("at least one resource is required (-r flag)")
			}

			// Get model config
			modelCfg, err := cfg.GetModelConfig(modelName)
			if err != nil {
				return fmt.Errorf("failed to get model: %w", err)
			}

			// Resolve resources
			var configResources []*config.Resource
			for _, name := range resources {
				r, ok := cfg.GetResource(name)
				if !ok {
					return fmt.Errorf("resource %q not found in config", name)
				}
				configResources = append(configResources, r)
			}

			// Create resource manager
			mgr := resource.NewManager(cfg.Cache.ResolvedPath)

			// Ensure collection
			fmt.Printf("Preparing resources...\n")
			collection, err := mgr.EnsureCollection(context.Background(), configResources)
			if err != nil {
				return fmt.Errorf("failed to prepare resources: %w", err)
			}

			// Create agent with model config
			agentOpts := agent.Options{
				Config:      cfg,
				ModelConfig: modelCfg,
				Collection:  collection,
				DataDir:     paths.DataDir,
			}

			a, err := agent.New(agentOpts)
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			// Create and run TUI
			model := tui.NewModel(cfg, paths, collection, a)
			return tui.Run(model)
		},
	}

	cmd.Flags().StringArrayVarP(&resources, "resource", "r", nil, "Resource(s) to search")
	cmd.Flags().StringVarP(&modelName, "model", "m", "", "Model to use (from config)")

	return cmd
}
