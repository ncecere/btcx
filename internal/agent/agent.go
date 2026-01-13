package agent

import (
	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/provider"
	"github.com/nickcecere/btcx/internal/resource"
	"github.com/nickcecere/btcx/internal/storage"
	"github.com/nickcecere/btcx/internal/tool"
)

// Agent is the main agent that orchestrates AI interactions
type Agent struct {
	// Config is the application configuration
	Config *config.Config

	// ModelConfig is the active model configuration
	ModelConfig *config.ModelConfig

	// Provider is the AI provider
	Provider provider.Provider

	// Collection is the current resource collection
	Collection *resource.Collection

	// Tools is the tool registry
	Tools *tool.Registry

	// Storage is the storage backend for threads
	Storage *storage.Storage

	// Thread is the current conversation thread
	Thread *storage.Thread
}

// Options are options for creating a new agent
type Options struct {
	Config      *config.Config
	ModelConfig *config.ModelConfig // If nil, uses default from config
	Collection  *resource.Collection
	DataDir     string
	Thread      *storage.Thread
}

// New creates a new agent
func New(opts Options) (*Agent, error) {
	// Get model config
	modelCfg := opts.ModelConfig
	if modelCfg == nil {
		var err error
		modelCfg, err = opts.Config.GetModelConfig("")
		if err != nil {
			return nil, err
		}
	}

	// Create provider from model config
	p, err := provider.NewFromModelConfig(modelCfg)
	if err != nil {
		return nil, err
	}

	// Create tool registry with collection path as working directory
	tools := tool.DefaultRegistry(opts.Collection.Path)

	// Set output directory for truncation
	if opts.Config.Output.ResolvedOutputDir != "" {
		tools.SetOutputDir(opts.Config.Output.ResolvedOutputDir)
	}

	// Create storage
	store := storage.NewStorage(opts.DataDir)

	return &Agent{
		Config:      opts.Config,
		ModelConfig: modelCfg,
		Provider:    p,
		Collection:  opts.Collection,
		Tools:       tools,
		Storage:     store,
		Thread:      opts.Thread,
	}, nil
}

// GetSystemPrompt returns the system prompt for this agent
func (a *Agent) GetSystemPrompt() string {
	return SystemPrompt(a.Collection)
}

// GetTools returns the tools as provider tools
func (a *Agent) GetTools() []provider.Tool {
	var result []provider.Tool
	for _, t := range a.Tools.List() {
		result = append(result, provider.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return result
}
