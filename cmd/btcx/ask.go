package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nickcecere/btcx/internal/agent"
	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/provider"
	"github.com/nickcecere/btcx/internal/resource"
	"github.com/nickcecere/btcx/internal/ui"
	"github.com/spf13/cobra"
)

// JSONOutput represents the JSON output format
type JSONOutput struct {
	Answer    string      `json:"answer"`
	ToolsUsed []ToolUsage `json:"tools_used"`
	Usage     *UsageInfo  `json:"usage,omitempty"`
	Model     *ModelInfo  `json:"model"`
	Resources []string    `json:"resources"`
}

// ToolUsage represents tool usage in JSON output
type ToolUsage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// UsageInfo represents token usage in JSON output
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ModelInfo represents model info in JSON output
type ModelInfo struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func askCmd() *cobra.Command {
	var resources []string
	var question string
	var continueThread bool
	var modelName string
	var noSpinner bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Ask a question about resources",
		Long:  `Ask a question about the specified resources. The AI will search the codebases to answer.`,
		Example: `  btcx ask -r svelte -q "How does the $state rune work?"
  btcx ask -r svelte -r typescript -q "How do I type reactive state?"
  btcx ask --continue -q "Can you explain more?"
  btcx ask -r cobra -q "What is Cobra?" -m claude
  btcx ask -r cobra -q "What is Cobra?" --no-spinner
  btcx ask -r cobra -q "What is Cobra?" --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, paths, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(resources) == 0 {
				return fmt.Errorf("at least one resource is required (-r flag)")
			}

			if question == "" {
				return fmt.Errorf("question is required (-q flag)")
			}

			// Get model config
			modelCfg, err := cfg.GetModelConfig(modelName)
			if err != nil {
				return fmt.Errorf("failed to get model: %w", err)
			}

			// Resolve resources
			var configResources []*config.Resource
			var resourceNames []string
			for _, name := range resources {
				r, ok := cfg.GetResource(name)
				if !ok {
					return fmt.Errorf("resource %q not found in config", name)
				}
				configResources = append(configResources, r)
				resourceNames = append(resourceNames, name)
			}

			// Create resource manager
			mgr := resource.NewManager(cfg.Cache.ResolvedPath)

			// Determine if we should show spinner
			// JSON output implies no spinner
			isJSON := outputFormat == "json"
			showSpinner := cfg.Output.Spinner && !noSpinner && !isJSON

			if !isJSON {
				fmt.Fprintf(os.Stderr, "Preparing resources...\n")
			}
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

			// Continue previous thread if requested
			if continueThread {
				thread, err := a.Storage.GetLatestThread()
				if err == nil {
					a.ContinueThread(thread)
					if !isJSON {
						fmt.Fprintf(os.Stderr, "Continuing thread: %s\n", thread.Title)
					}
				}
			}

			// Start spinner if enabled
			var spinner *ui.Spinner
			if showSpinner {
				spinner = ui.NewSpinner("Thinking...")
				spinner.Start()
			}

			// Collect response (buffered mode)
			var content strings.Builder
			var totalUsage *provider.Usage
			toolCounts := make(map[string]int)

			callback := func(event provider.StreamEvent) {
				switch event.Type {
				case provider.StreamEventText:
					content.WriteString(event.Delta)
				case provider.StreamEventToolCall:
					if event.ToolCall != nil {
						toolCounts[event.ToolCall.Name]++
						if spinner != nil {
							spinner.UpdateMessage(fmt.Sprintf("Using %s...", event.ToolCall.Name))
						}
					}
				case provider.StreamEventToolResult:
					// Tool finished, back to thinking
					if spinner != nil {
						spinner.UpdateMessage("Thinking...")
					}
				case provider.StreamEventDone:
					if event.Usage != nil {
						totalUsage = event.Usage
					}
				case provider.StreamEventError:
					// Will be handled by the error return
				}
			}

			resp, err := a.AskWithCallback(context.Background(), question, callback)

			// Stop spinner
			if spinner != nil {
				spinner.Stop()
			}

			if err != nil {
				return fmt.Errorf("failed to get response: %w", err)
			}

			// Get final content - prefer response content over streamed content
			// (non-streaming mode returns content in response, streaming collects via callback)
			finalContent := content.String()
			if finalContent == "" && resp != nil {
				finalContent = resp.Content
			}

			// Get usage from response if not from stream
			if totalUsage == nil && resp != nil {
				totalUsage = &provider.Usage{
					InputTokens:  resp.Usage.InputTokens,
					OutputTokens: resp.Usage.OutputTokens,
					TotalTokens:  resp.Usage.TotalTokens,
				}
			}

			// Output based on format
			if isJSON {
				return outputJSON(finalContent, toolCounts, totalUsage, modelCfg, resourceNames)
			}

			return outputHuman(cfg, finalContent, totalUsage)
		},
	}

	cmd.Flags().StringArrayVarP(&resources, "resource", "r", nil, "Resource(s) to search")
	cmd.Flags().StringVarP(&question, "question", "q", "", "Question to ask")
	cmd.Flags().BoolVarP(&continueThread, "continue", "c", false, "Continue the last conversation thread")
	cmd.Flags().StringVarP(&modelName, "model", "m", "", "Model to use (from config)")
	cmd.Flags().BoolVar(&noSpinner, "no-spinner", false, "Disable the animated spinner")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json)")

	return cmd
}

// outputHuman outputs the response in human-readable format
func outputHuman(cfg *config.Config, content string, usage *provider.Usage) error {
	// Render and display the answer
	fmt.Println(ui.Header.Render("Answer"))
	fmt.Println()

	if cfg.Output.Markdown {
		rendered, renderErr := ui.RenderMarkdown(content)
		if renderErr != nil {
			// Fallback to raw output if rendering fails
			fmt.Println(content)
		} else {
			fmt.Print(rendered)
		}
	} else {
		fmt.Println(content)
	}

	// Show token usage
	if cfg.Output.ShowUsage && usage != nil {
		fmt.Println()
		fmt.Println(ui.Usage.Render(fmt.Sprintf("[Tokens: %d in, %d out]",
			usage.InputTokens, usage.OutputTokens)))
	}

	return nil
}

// outputJSON outputs the response in JSON format
func outputJSON(content string, toolCounts map[string]int, usage *provider.Usage, modelCfg *config.ModelConfig, resourceNames []string) error {
	output := JSONOutput{
		Answer:    content,
		ToolsUsed: []ToolUsage{},
		Model: &ModelInfo{
			Name:     modelCfg.Name,
			Provider: string(modelCfg.Provider),
			Model:    modelCfg.Model,
		},
		Resources: resourceNames,
	}

	// Convert tool counts to array
	for name, count := range toolCounts {
		output.ToolsUsed = append(output.ToolsUsed, ToolUsage{
			Name:  name,
			Count: count,
		})
	}

	// Add usage if available
	if usage != nil {
		output.Usage = &UsageInfo{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
		}
	}

	// Marshal and print
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
