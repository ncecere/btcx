package main

import (
	"fmt"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/ui"
	"github.com/spf13/cobra"
)

func modelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage model configurations",
		Long:  `List and manage model configurations.`,
	}

	cmd.AddCommand(modelsListCmd())

	return cmd
}

func modelsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured models",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check if we have models configured
			if len(cfg.Models) == 0 {
				// Check for legacy config
				if cfg.Provider != "" && cfg.Model != "" {
					fmt.Println("Configured models (legacy format):")
					fmt.Println()
					fmt.Printf("  * %s\n", ui.Bold.Render("default"))
					fmt.Printf("      Provider: %s\n", cfg.Provider)
					fmt.Printf("      Model:    %s\n", cfg.Model)
					if cfg.BaseURL != "" {
						fmt.Printf("      Base URL: %s\n", cfg.BaseURL)
					}
					fmt.Println()
					fmt.Println(ui.Dim.Render("Tip: Consider migrating to the new 'models' format in your config."))
					return nil
				}

				fmt.Println("No models configured.")
				fmt.Println()
				fmt.Println("Add models to your config file (~/.config/btcx/config.yaml):")
				fmt.Println()
				fmt.Println("  models:")
				fmt.Println("    - name: devstral")
				fmt.Println("      provider: ollama")
				fmt.Println("      model: devstral-small-2:latest")
				fmt.Println()
				fmt.Println("  defaultModel: devstral")
				return nil
			}

			fmt.Printf("Configured models (%d):\n", len(cfg.Models))
			fmt.Println()

			for _, m := range cfg.Models {
				// Mark default model with asterisk
				prefix := "  "
				name := m.Name
				if m.Name == cfg.DefaultModel {
					prefix = "* "
					name = ui.Bold.Render(m.Name) + " " + ui.Dim.Render("(default)")
				}

				fmt.Printf("%s%s\n", prefix, name)
				fmt.Printf("      Provider: %s\n", m.Provider)
				fmt.Printf("      Model:    %s\n", m.Model)
				if m.BaseURL != "" {
					fmt.Printf("      Base URL: %s\n", m.BaseURL)
				}
				if m.APIKey != "" {
					// Show masked API key
					masked := m.APIKey
					if len(masked) > 8 {
						masked = masked[:4] + "..." + masked[len(masked)-4:]
					} else {
						masked = "****"
					}
					fmt.Printf("      API Key:  %s\n", masked)
				}
				fmt.Println()
			}

			return nil
		},
	}
}
