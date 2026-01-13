package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// GlobalConfigDir is the XDG-compliant config directory
	GlobalConfigDir = ".config/btcx"
	// GlobalConfigFile is the default config filename
	GlobalConfigFile = "config.yaml"
	// ProjectConfigFile is the project-local config filename
	ProjectConfigFile = "btcx.config.yaml"
	// DefaultCacheDir is the default cache directory
	DefaultCacheDir = ".cache/btcx"
	// DefaultDataDir is the default data directory for threads
	DefaultDataDir = ".local/share/btcx"
)

// Paths contains resolved paths for btcx
type Paths struct {
	GlobalConfig  string
	ProjectConfig string
	CacheDir      string
	DataDir       string
}

// ResolvePaths resolves all paths based on the current environment
func ResolvePaths() (*Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	paths := &Paths{
		GlobalConfig:  filepath.Join(homeDir, GlobalConfigDir, GlobalConfigFile),
		ProjectConfig: filepath.Join(cwd, ProjectConfigFile),
		CacheDir:      filepath.Join(homeDir, DefaultCacheDir),
		DataDir:       filepath.Join(homeDir, DefaultDataDir),
	}

	// Allow override via environment variable
	if configPath := os.Getenv("BTCX_CONFIG"); configPath != "" {
		paths.GlobalConfig = configPath
	}

	return paths, nil
}

// Load loads and merges configuration from global and project config files
func Load() (*Config, *Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, nil, err
	}

	// Start with defaults
	cfg := Defaults()

	// Load global config if it exists
	if err := loadYAML(paths.GlobalConfig, &cfg); err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Load project config if it exists (overrides global)
	if err := loadYAML(paths.ProjectConfig, &cfg); err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to load project config: %w", err)
	}

	// Resolve cache path - keep original in Path, put resolved in ResolvedPath
	if cfg.Cache.Path == "" {
		cfg.Cache.ResolvedPath = paths.CacheDir
	} else {
		resolved := cfg.Cache.Path
		// Expand ~ in cache path first
		if len(resolved) > 0 && resolved[0] == '~' {
			homeDir, _ := os.UserHomeDir()
			resolved = filepath.Join(homeDir, resolved[1:])
		} else if !filepath.IsAbs(resolved) {
			// Relative paths are relative to current directory
			cwd, _ := os.Getwd()
			resolved = filepath.Join(cwd, resolved)
		}
		cfg.Cache.ResolvedPath = resolved
	}

	// Resolve output directory for truncated outputs
	if cfg.Output.OutputDir == "" {
		cfg.Output.ResolvedOutputDir = filepath.Join(paths.DataDir, "outputs")
	} else {
		resolved := cfg.Output.OutputDir
		// Expand ~ in output path first
		if len(resolved) > 0 && resolved[0] == '~' {
			homeDir, _ := os.UserHomeDir()
			resolved = filepath.Join(homeDir, resolved[1:])
		} else if !filepath.IsAbs(resolved) {
			// Relative paths are relative to data directory
			resolved = filepath.Join(paths.DataDir, resolved)
		}
		cfg.Output.ResolvedOutputDir = resolved
	}

	// Resolve API keys for all models
	for i := range cfg.Models {
		cfg.Models[i].APIKey = resolveModelAPIKey(&cfg.Models[i])
	}

	// Load API key for legacy config
	cfg.APIKey = resolveAPIKey(cfg.Provider, cfg.APIKey)

	return &cfg, paths, nil
}

// loadYAML loads a YAML file into the given struct
func loadYAML(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// resolveModelAPIKey resolves the API key for a model config
func resolveModelAPIKey(m *ModelConfig) string {
	// Ollama doesn't require an API key
	if m.Provider == ProviderOllama {
		return ""
	}

	// If API key is set in config, use it
	if m.APIKey != "" {
		return m.APIKey
	}

	// Fall back to environment variables
	switch m.Provider {
	case ProviderAnthropic:
		return os.Getenv("ANTHROPIC_API_KEY")
	case ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case ProviderOpenAICompatible:
		if key := os.Getenv("OPENAI_COMPATIBLE_API_KEY"); key != "" {
			return key
		}
		return os.Getenv("OPENAI_API_KEY")
	case ProviderGoogle:
		return os.Getenv("GOOGLE_API_KEY")
	}

	return ""
}

// resolveAPIKey resolves the API key from environment or config (legacy)
func resolveAPIKey(provider ProviderType, configKey string) string {
	// Ollama doesn't require an API key
	if provider == ProviderOllama {
		return ""
	}

	// Environment variables take precedence
	var envKey string
	switch provider {
	case ProviderAnthropic:
		envKey = os.Getenv("ANTHROPIC_API_KEY")
	case ProviderOpenAI:
		envKey = os.Getenv("OPENAI_API_KEY")
	case ProviderOpenAICompatible:
		envKey = os.Getenv("OPENAI_COMPATIBLE_API_KEY")
		if envKey == "" {
			envKey = os.Getenv("OPENAI_API_KEY")
		}
	case ProviderGoogle:
		envKey = os.Getenv("GOOGLE_API_KEY")
	}

	if envKey != "" {
		return envKey
	}
	return configKey
}

// Save saves the configuration to the global config file
func Save(cfg *Config) error {
	paths, err := ResolvePaths()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(paths.GlobalConfig)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(paths.GlobalConfig, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetModelConfig returns a model configuration by name
// If name is empty, returns the default model
// Falls back to legacy flat config if no models are defined
func (c *Config) GetModelConfig(name string) (*ModelConfig, error) {
	// If name specified, look it up in models list
	if name != "" {
		for i := range c.Models {
			if c.Models[i].Name == name {
				return &c.Models[i], nil
			}
		}
		return nil, fmt.Errorf("model %q not found in config", name)
	}

	// Use defaultModel if set
	if c.DefaultModel != "" {
		for i := range c.Models {
			if c.Models[i].Name == c.DefaultModel {
				return &c.Models[i], nil
			}
		}
		return nil, fmt.Errorf("default model %q not found in config", c.DefaultModel)
	}

	// If models list has entries, use the first one
	if len(c.Models) > 0 {
		return &c.Models[0], nil
	}

	// Fall back to legacy flat config (backward compatibility)
	if c.Provider != "" && c.Model != "" {
		return &ModelConfig{
			Name:     "default",
			Provider: c.Provider,
			Model:    c.Model,
			BaseURL:  c.BaseURL,
			APIKey:   c.APIKey,
		}, nil
	}

	return nil, fmt.Errorf("no model configured; add models to config or set provider/model")
}

// GetResource returns a resource by name
func (c *Config) GetResource(name string) (*Resource, bool) {
	for i := range c.Resources {
		if c.Resources[i].Name == name {
			return &c.Resources[i], true
		}
	}
	return nil, false
}

// AddResource adds a resource to the configuration
func (c *Config) AddResource(r Resource) error {
	if _, exists := c.GetResource(r.Name); exists {
		return fmt.Errorf("resource %q already exists", r.Name)
	}
	c.Resources = append(c.Resources, r)
	return nil
}

// RemoveResource removes a resource from the configuration
func (c *Config) RemoveResource(name string) error {
	for i := range c.Resources {
		if c.Resources[i].Name == name {
			c.Resources = append(c.Resources[:i], c.Resources[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("resource %q not found", name)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Check if we have any model configuration
	hasModels := len(c.Models) > 0
	hasLegacy := c.Provider != "" && c.Model != ""

	if !hasModels && !hasLegacy {
		return fmt.Errorf("no model configured; add models to config or set provider/model")
	}

	// Validate models list
	seenModels := make(map[string]bool)
	for _, m := range c.Models {
		if m.Name == "" {
			return fmt.Errorf("model name is required")
		}
		if seenModels[m.Name] {
			return fmt.Errorf("duplicate model name: %s", m.Name)
		}
		seenModels[m.Name] = true

		// Validate provider
		switch m.Provider {
		case ProviderAnthropic, ProviderOpenAI, ProviderOpenAICompatible, ProviderGoogle, ProviderOllama:
			// Valid
		default:
			return fmt.Errorf("model %q: invalid provider: %s", m.Name, m.Provider)
		}

		if m.Model == "" {
			return fmt.Errorf("model %q: model ID is required", m.Name)
		}

		// Validate openai-compatible requires baseUrl
		if m.Provider == ProviderOpenAICompatible && m.BaseURL == "" {
			return fmt.Errorf("model %q: baseUrl is required for openai-compatible provider", m.Name)
		}
	}

	// Validate defaultModel references a valid model
	if c.DefaultModel != "" && len(c.Models) > 0 {
		found := false
		for _, m := range c.Models {
			if m.Name == c.DefaultModel {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("defaultModel %q not found in models list", c.DefaultModel)
		}
	}

	// Validate legacy config if using it
	if hasLegacy && !hasModels {
		switch c.Provider {
		case ProviderAnthropic, ProviderOpenAI, ProviderOpenAICompatible, ProviderGoogle, ProviderOllama:
			// Valid
		default:
			return fmt.Errorf("invalid provider: %s", c.Provider)
		}

		if c.Provider == ProviderOpenAICompatible && c.BaseURL == "" {
			return fmt.Errorf("baseUrl is required for openai-compatible provider")
		}
	}

	// Validate resources
	seen := make(map[string]bool)
	for _, r := range c.Resources {
		if r.Name == "" {
			return fmt.Errorf("resource name is required")
		}
		if seen[r.Name] {
			return fmt.Errorf("duplicate resource name: %s", r.Name)
		}
		seen[r.Name] = true

		switch r.Type {
		case ResourceTypeGit:
			if r.URL == "" {
				return fmt.Errorf("resource %q: url is required for git resources", r.Name)
			}
		case ResourceTypeLocal:
			if r.Path == "" {
				return fmt.Errorf("resource %q: path is required for local resources", r.Name)
			}
		default:
			return fmt.Errorf("resource %q: invalid type: %s", r.Name, r.Type)
		}
	}

	return nil
}
