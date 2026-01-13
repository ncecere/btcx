package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "v0.1.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "btcx",
		Short:   "A documentation search agent powered by AI",
		Long:    `btcx helps you search and understand codebases by asking questions about libraries and frameworks.`,
		Version: version,
	}

	// Add commands
	rootCmd.AddCommand(askCmd())
	rootCmd.AddCommand(tuiCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(resourcesCmd())
	rootCmd.AddCommand(cacheCmd())
	rootCmd.AddCommand(threadsCmd())
	rootCmd.AddCommand(modelsCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
