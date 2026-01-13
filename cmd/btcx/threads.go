package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/storage"
	"github.com/spf13/cobra"
)

func threadsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "threads",
		Short: "Manage conversation threads",
		Long:  `View, continue, and delete conversation threads.`,
	}

	cmd.AddCommand(threadsListCmd())
	cmd.AddCommand(threadsShowCmd())
	cmd.AddCommand(threadsDeleteCmd())
	cmd.AddCommand(threadsClearCmd())

	return cmd
}

func threadsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store := storage.NewStorage(paths.DataDir)

			threads, err := store.ListThreads()
			if err != nil {
				return fmt.Errorf("failed to list threads: %w", err)
			}

			if len(threads) == 0 {
				fmt.Println("No threads found.")
				return nil
			}

			fmt.Printf("Threads (%d):\n\n", len(threads))
			for _, t := range threads {
				age := formatAge(t.Updated)
				resources := strings.Join(t.Resources, ", ")
				fmt.Printf("  %s\n", t.ID)
				fmt.Printf("    Title:     %s\n", t.Title)
				fmt.Printf("    Updated:   %s\n", age)
				fmt.Printf("    Resources: %s\n", resources)
				fmt.Printf("    Messages:  %d\n", len(t.Messages))
				fmt.Println()
			}

			return nil
		},
	}
}

func threadsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show thread details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			_, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store := storage.NewStorage(paths.DataDir)

			thread, err := store.LoadThread(id)
			if err != nil {
				return err
			}

			fmt.Printf("Thread: %s\n", thread.ID)
			fmt.Printf("Title: %s\n", thread.Title)
			fmt.Printf("Created: %s\n", thread.Created.Format(time.RFC3339))
			fmt.Printf("Updated: %s\n", thread.Updated.Format(time.RFC3339))
			fmt.Printf("Resources: %s\n", strings.Join(thread.Resources, ", "))
			fmt.Printf("Provider: %s\n", thread.Provider)
			fmt.Printf("Model: %s\n", thread.Model)
			fmt.Printf("\nMessages (%d):\n\n", len(thread.Messages))

			for i, msg := range thread.Messages {
				fmt.Printf("--- Message %d (%s) ---\n", i+1, msg.Role)
				if msg.Content != "" {
					// Truncate long messages
					content := msg.Content
					if len(content) > 500 {
						content = content[:500] + "..."
					}
					fmt.Printf("%s\n", content)
				}
				if len(msg.ToolCalls) > 0 {
					fmt.Printf("Tool calls: ")
					for _, tc := range msg.ToolCalls {
						fmt.Printf("%s ", tc.Name)
					}
					fmt.Println()
				}
				fmt.Println()
			}

			return nil
		},
	}
}

func threadsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			_, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store := storage.NewStorage(paths.DataDir)

			if err := store.DeleteThread(id); err != nil {
				return err
			}

			fmt.Printf("Deleted thread: %s\n", id)
			return nil
		},
	}
}

func threadsClearCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete all threads",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("use --confirm to delete all threads")
			}

			_, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store := storage.NewStorage(paths.DataDir)

			if err := store.ClearThreads(); err != nil {
				return fmt.Errorf("failed to clear threads: %w", err)
			}

			fmt.Println("All threads deleted.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm deletion of all threads")

	return cmd
}

// formatAge formats a time as a human-readable age
func formatAge(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
