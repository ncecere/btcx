package main

import (
	"fmt"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View and modify btcx configuration.`,
	}

	cmd.AddCommand(configShowCmd())
	cmd.AddCommand(configSetCmd())
	cmd.AddCommand(configPathCmd())

	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		Example: `  btcx config set provider openai
  btcx config set model gpt-4o
  btcx config set provider ollama
  btcx config set model llama3.2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			switch key {
			case "provider":
				cfg.Provider = config.ProviderType(value)
			case "model":
				cfg.Model = value
			case "baseUrl":
				cfg.BaseURL = value
			case "apiKey":
				cfg.APIKey = value
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Set %s = %s\n", key, value)
			return nil
		},
	}
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return fmt.Errorf("failed to resolve paths: %w", err)
			}

			fmt.Printf("Global config:  %s\n", paths.GlobalConfig)
			fmt.Printf("Project config: %s\n", paths.ProjectConfig)
			fmt.Printf("Cache dir:      %s\n", paths.CacheDir)
			fmt.Printf("Data dir:       %s\n", paths.DataDir)
			return nil
		},
	}
}
