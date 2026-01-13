package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nickcecere/btcx/internal/config"
)

// Provider is the interface that all AI providers must implement
type Provider interface {
	// Name returns the provider name
	Name() string

	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat sends a chat request and returns a channel of streaming events
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}

// ChatRequest represents a chat request
type ChatRequest struct {
	// Model is the model to use
	Model string

	// System is the system prompt
	System string

	// Messages is the conversation history
	Messages []Message

	// Tools are the available tools
	Tools []Tool

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int
}

// Message represents a chat message
type Message struct {
	// Role is the message role (system, user, assistant, tool)
	Role string `json:"role"`

	// Content is the text content (for user/assistant messages)
	Content string `json:"content,omitempty"`

	// ToolCalls are tool calls made by the assistant
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID is the ID of the tool call this message is responding to
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation by the assistant
type ToolCall struct {
	// ID is the unique identifier for this tool call
	ID string `json:"id"`

	// Name is the tool name
	Name string `json:"name"`

	// Arguments is the JSON arguments
	Arguments json.RawMessage `json:"arguments"`
}

// Tool represents a tool definition
type Tool struct {
	// Name is the tool name
	Name string

	// Description is the tool description
	Description string

	// Parameters is the JSON schema for the parameters
	Parameters map[string]interface{}
}

// ChatResponse represents a chat response
type ChatResponse struct {
	// Content is the text content of the response
	Content string

	// ToolCalls are any tool calls made by the assistant
	ToolCalls []ToolCall

	// StopReason is why the model stopped generating
	StopReason string

	// Usage contains token usage information
	Usage Usage
}

// Usage contains token usage information
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	// Type is the event type
	Type StreamEventType

	// Delta is the content delta for text events
	Delta string

	// ToolCall is the tool call for tool events
	ToolCall *ToolCall

	// Error is any error that occurred
	Error error

	// Usage is the final usage (sent with Done event)
	Usage *Usage

	// StopReason is why the model stopped (sent with Done event)
	StopReason string
}

// StreamEventType is the type of streaming event
type StreamEventType string

const (
	// StreamEventText is a text delta
	StreamEventText StreamEventType = "text"

	// StreamEventToolCall is a tool call starting
	StreamEventToolCall StreamEventType = "tool_call"

	// StreamEventToolResult is a tool call completing
	StreamEventToolResult StreamEventType = "tool_result"

	// StreamEventDone indicates the stream is complete
	StreamEventDone StreamEventType = "done"

	// StreamEventError indicates an error occurred
	StreamEventError StreamEventType = "error"
)

// New creates a new provider based on the configuration (legacy)
func New(cfg *config.Config) (Provider, error) {
	switch cfg.Provider {
	case config.ProviderAnthropic:
		return NewAnthropicProvider(cfg.APIKey, cfg.Model)
	case config.ProviderOpenAI:
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, "")
	case config.ProviderOpenAICompatible:
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, cfg.BaseURL)
	case config.ProviderGoogle:
		return NewGoogleProvider(cfg.APIKey, cfg.Model)
	case config.ProviderOllama:
		return NewOllamaProvider(cfg.Model, cfg.BaseURL)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// NewFromModelConfig creates a new provider from a ModelConfig
func NewFromModelConfig(m *config.ModelConfig) (Provider, error) {
	switch m.Provider {
	case config.ProviderAnthropic:
		return NewAnthropicProvider(m.APIKey, m.Model)
	case config.ProviderOpenAI:
		return NewOpenAIProvider(m.APIKey, m.Model, "")
	case config.ProviderOpenAICompatible:
		return NewOpenAIProvider(m.APIKey, m.Model, m.BaseURL)
	case config.ProviderGoogle:
		return NewGoogleProvider(m.APIKey, m.Model)
	case config.ProviderOllama:
		return NewOllamaProvider(m.Model, m.BaseURL)
	default:
		return nil, fmt.Errorf("unknown provider: %s", m.Provider)
	}
}
