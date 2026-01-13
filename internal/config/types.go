package config

// ProviderType represents the type of AI provider
type ProviderType string

const (
	ProviderAnthropic        ProviderType = "anthropic"
	ProviderOpenAI           ProviderType = "openai"
	ProviderOpenAICompatible ProviderType = "openai-compatible"
	ProviderGoogle           ProviderType = "google"
	ProviderOllama           ProviderType = "ollama"
)

// Default Ollama base URL
const DefaultOllamaBaseURL = "http://localhost:11434/v1"

// Config represents the main configuration for btcx
type Config struct {
	// DefaultModel is the name of the default model to use (from Models list)
	DefaultModel string `yaml:"defaultModel,omitempty"`

	// Models is the list of named model configurations
	Models []ModelConfig `yaml:"models,omitempty"`

	// Output controls CLI output behavior
	Output OutputConfig `yaml:"output,omitempty"`

	// Legacy fields (for backward compatibility with flat config)
	Provider ProviderType `yaml:"provider,omitempty"`
	Model    string       `yaml:"model,omitempty"`
	BaseURL  string       `yaml:"baseUrl,omitempty"`
	APIKey   string       `yaml:"apiKey,omitempty"`

	// Cache configuration
	Cache CacheConfig `yaml:"cache,omitempty"`

	// Resources is the list of configured resources
	Resources []Resource `yaml:"resources,omitempty"`
}

// ModelConfig represents a named AI model configuration
type ModelConfig struct {
	// Name is the unique identifier for this model config
	Name string `yaml:"name"`

	// Provider is the AI provider type
	Provider ProviderType `yaml:"provider"`

	// Model is the model ID to use
	Model string `yaml:"model"`

	// BaseURL is the custom base URL (optional, for ollama or openai-compatible)
	BaseURL string `yaml:"baseUrl,omitempty"`

	// APIKey is an optional API key (prefer environment variables)
	APIKey string `yaml:"apiKey,omitempty"`
}

// OutputConfig controls CLI output behavior
type OutputConfig struct {
	// Spinner enables the animated spinner during processing (default: true)
	Spinner bool `yaml:"spinner"`

	// Markdown enables markdown rendering of output (default: true)
	Markdown bool `yaml:"markdown"`

	// ShowUsage shows token usage after response (default: true)
	ShowUsage bool `yaml:"showUsage"`

	// OutputDir is the directory for truncated tool outputs
	// Default: ~/.local/share/btcx/outputs
	OutputDir string `yaml:"outputDir,omitempty"`

	// ResolvedOutputDir is the absolute path after expanding ~ and relative paths
	// This is not saved to the config file
	ResolvedOutputDir string `yaml:"-"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	// Path is the directory to store cached resources
	// Default: ~/.cache/btcx
	Path string `yaml:"path,omitempty"`

	// ResolvedPath is the absolute path after expanding ~ and relative paths
	// This is not saved to the config file
	ResolvedPath string `yaml:"-"`
}

// ResourceType represents the type of resource
type ResourceType string

const (
	ResourceTypeGit   ResourceType = "git"
	ResourceTypeLocal ResourceType = "local"
)

// Resource represents a documentation resource
type Resource struct {
	// Name is the unique identifier for this resource
	Name string `yaml:"name"`

	// Type is the resource type (git or local)
	Type ResourceType `yaml:"type"`

	// URL is the git repository URL (for git resources)
	URL string `yaml:"url,omitempty"`

	// Branch is the git branch to use (for git resources)
	Branch string `yaml:"branch,omitempty"`

	// Path is the local filesystem path (for local resources)
	Path string `yaml:"path,omitempty"`

	// SearchPath is the subdirectory to focus on within the resource
	SearchPath string `yaml:"searchPath,omitempty"`

	// Notes are hints for the AI about this resource
	Notes string `yaml:"notes,omitempty"`
}

// Defaults returns a Config with default values
func Defaults() Config {
	return Config{
		Output: OutputConfig{
			Spinner:   true,
			Markdown:  true,
			ShowUsage: true,
		},
		Cache: CacheConfig{
			Path: "", // Will be resolved to ~/.cache/btcx
		},
		Resources: []Resource{},
	}
}
