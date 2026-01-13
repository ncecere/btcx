package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/nickcecere/btcx/internal/config"
	"github.com/sashabaranov/go-openai"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	client  *openai.Client
	model   string
	baseURL string
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(model, baseURL string) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = config.DefaultOllamaBaseURL
	}

	// Ollama doesn't require an API key, but the OpenAI client needs something
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = baseURL

	client := openai.NewClientWithConfig(cfg)

	return &OllamaProvider{
		client:  client,
		model:   model,
		baseURL: baseURL,
	}, nil
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Chat sends a chat request to Ollama
func (p *OllamaProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := p.convertMessages(req)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request
	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	if len(tools) > 0 {
		ollamaReq.Tools = tools
	}

	// Make request
	resp, err := p.client.CreateChatCompletion(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}

	// Convert response
	return p.convertResponse(&resp), nil
}

// StreamChat streams a chat response from Ollama
func (p *OllamaProvider) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	// Convert messages
	messages := p.convertMessages(req)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request
	model := req.Model
	if model == "" {
		model = p.model
	}

	ollamaReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	if len(tools) > 0 {
		ollamaReq.Tools = tools
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream request failed: %w", err)
	}

	events := make(chan StreamEvent)

	go func() {
		defer close(events)
		defer stream.Close()

		toolCalls := make(map[int]*ToolCall)
		var stopReason string

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// Send done event
				events <- StreamEvent{
					Type:       StreamEventDone,
					StopReason: stopReason,
				}
				break
			}
			if err != nil {
				events <- StreamEvent{
					Type:  StreamEventError,
					Error: err,
				}
				break
			}

			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]

			// Check finish reason
			if choice.FinishReason != "" {
				stopReason = string(choice.FinishReason)
			}

			// Handle content delta
			if choice.Delta.Content != "" {
				events <- StreamEvent{
					Type:  StreamEventText,
					Delta: choice.Delta.Content,
				}
			}

			// Handle tool calls
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index
				if idx == nil {
					continue
				}

				if _, exists := toolCalls[*idx]; !exists {
					toolCalls[*idx] = &ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: json.RawMessage(""),
					}
				}

				// Accumulate arguments
				if tc.Function.Arguments != "" {
					existing := string(toolCalls[*idx].Arguments)
					toolCalls[*idx].Arguments = json.RawMessage(existing + tc.Function.Arguments)
				}
			}
		}

		// Emit tool calls at the end
		for _, tc := range toolCalls {
			if tc.ID != "" && tc.Name != "" {
				events <- StreamEvent{
					Type:     StreamEventToolCall,
					ToolCall: tc,
				}
			}
		}
	}()

	return events, nil
}

// convertMessages converts our messages to OpenAI format (Ollama uses OpenAI-compatible API)
func (p *OllamaProvider) convertMessages(req *ChatRequest) []openai.ChatCompletionMessage {
	var result []openai.ChatCompletionMessage

	// Add system message if provided
	if req.System != "" {
		result = append(result, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.System,
		})
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			result = append(result, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: msg.Content,
			})

		case "assistant":
			oaiMsg := openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg.Content,
			}
			for _, tc := range msg.ToolCalls {
				oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				})
			}
			result = append(result, oaiMsg)

		case "tool":
			result = append(result, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
			})
		}
	}

	return result
}

// convertTools converts our tools to OpenAI format
func (p *OllamaProvider) convertTools(tools []Tool) []openai.Tool {
	var result []openai.Tool

	for _, tool := range tools {
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return result
}

// convertResponse converts an OpenAI response to our format
func (p *OllamaProvider) convertResponse(resp *openai.ChatCompletionResponse) *ChatResponse {
	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Content = choice.Message.Content
		result.StopReason = string(choice.FinishReason)

		for _, tc := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	return result
}
