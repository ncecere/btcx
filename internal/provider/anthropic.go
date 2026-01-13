package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/liushuangls/go-anthropic/v2"
)

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required")
	}

	client := anthropic.NewClient(apiKey)

	return &AnthropicProvider{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Chat sends a chat request to Anthropic
func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := p.convertMessages(req.Messages)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	anthropicReq := anthropic.MessagesRequest{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	if req.System != "" {
		anthropicReq.System = req.System
	}

	if len(tools) > 0 {
		anthropicReq.Tools = tools
	}

	// Make request
	resp, err := p.client.CreateMessages(ctx, anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}

	// Convert response
	return p.convertResponse(&resp), nil
}

// StreamChat streams a chat response from Anthropic
func (p *AnthropicProvider) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	// Convert messages
	messages := p.convertMessages(req.Messages)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		var currentToolCall *ToolCall
		var toolInput string
		var usage Usage
		var stopReason string

		streamReq := anthropic.MessagesStreamRequest{
			MessagesRequest: anthropic.MessagesRequest{
				Model:     anthropic.Model(model),
				MaxTokens: maxTokens,
				Messages:  messages,
			},
			OnContentBlockStart: func(data anthropic.MessagesEventContentBlockStartData) {
				if data.ContentBlock.Type == anthropic.MessagesContentTypeToolUse {
					currentToolCall = &ToolCall{
						ID:   data.ContentBlock.ID,
						Name: data.ContentBlock.Name,
					}
					toolInput = ""
				}
			},
			OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
				switch data.Delta.Type {
				case anthropic.MessagesContentTypeText:
					if data.Delta.Text != nil && *data.Delta.Text != "" {
						events <- StreamEvent{
							Type:  StreamEventText,
							Delta: *data.Delta.Text,
						}
					}
				case "input_json_delta":
					if data.Delta.PartialJson != nil {
						toolInput += *data.Delta.PartialJson
					}
				}
			},
			OnContentBlockStop: func(data anthropic.MessagesEventContentBlockStopData, content anthropic.MessageContent) {
				if currentToolCall != nil {
					currentToolCall.Arguments = json.RawMessage(toolInput)
					events <- StreamEvent{
						Type:     StreamEventToolCall,
						ToolCall: currentToolCall,
					}
					currentToolCall = nil
					toolInput = ""
				}
			},
			OnMessageDelta: func(data anthropic.MessagesEventMessageDeltaData) {
				usage.OutputTokens = data.Usage.OutputTokens
				stopReason = string(data.Delta.StopReason)
			},
			OnMessageStop: func(data anthropic.MessagesEventMessageStopData) {
				events <- StreamEvent{
					Type:       StreamEventDone,
					Usage:      &usage,
					StopReason: stopReason,
				}
			},
			OnError: func(err anthropic.ErrorResponse) {
				events <- StreamEvent{
					Type:  StreamEventError,
					Error: fmt.Errorf("anthropic error: %s", err.Error.Message),
				}
			},
		}

		if req.System != "" {
			streamReq.System = req.System
		}

		if len(tools) > 0 {
			streamReq.Tools = tools
		}

		_, err := p.client.CreateMessagesStream(ctx, streamReq)
		if err != nil {
			events <- StreamEvent{
				Type:  StreamEventError,
				Error: err,
			}
		}
	}()

	return events, nil
}

// convertMessages converts our messages to Anthropic format
func (p *AnthropicProvider) convertMessages(messages []Message) []anthropic.Message {
	var result []anthropic.Message

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			result = append(result, anthropic.NewUserTextMessage(msg.Content))

		case "assistant":
			var content []anthropic.MessageContent
			if msg.Content != "" {
				content = append(content, anthropic.NewTextMessageContent(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				content = append(content, anthropic.NewToolUseMessageContent(tc.ID, tc.Name, tc.Arguments))
			}
			result = append(result, anthropic.Message{
				Role:    anthropic.RoleAssistant,
				Content: content,
			})

		case "tool":
			result = append(result, anthropic.NewToolResultsMessage(msg.ToolCallID, msg.Content, false))
		}
	}

	return result
}

// convertTools converts our tools to Anthropic format
func (p *AnthropicProvider) convertTools(tools []Tool) []anthropic.ToolDefinition {
	var result []anthropic.ToolDefinition

	for _, tool := range tools {
		inputSchema, _ := json.Marshal(tool.Parameters)
		result = append(result, anthropic.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}

	return result
}

// convertResponse converts an Anthropic response to our format
func (p *AnthropicProvider) convertResponse(resp *anthropic.MessagesResponse) *ChatResponse {
	result := &ChatResponse{
		StopReason: string(resp.StopReason),
		Usage: Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case anthropic.MessagesContentTypeText:
			if block.Text != nil {
				result.Content += *block.Text
			}
		case anthropic.MessagesContentTypeToolUse:
			if block.ID != "" && block.Name != "" {
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					ID:        block.ID,
					Name:      block.Name,
					Arguments: block.Input,
				})
			}
		}
	}

	return result
}
