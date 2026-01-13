package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIProvider implements the Provider interface for OpenAI and compatible APIs
type OpenAIProvider struct {
	client  openai.Client
	model   string
	baseURL string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, model, baseURL string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &OpenAIProvider{
		client:  client,
		model:   model,
		baseURL: baseURL,
	}, nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	if p.baseURL != "" {
		return "openai-compatible"
	}
	return "openai"
}

// Chat sends a chat request to OpenAI
func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert messages
	messages := p.convertMessages(req)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request params
	model := req.Model
	if model == "" {
		model = p.model
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	// Make request
	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}

	// Convert response
	return p.convertResponse(resp), nil
}

// StreamChat streams a chat response from OpenAI
func (p *OpenAIProvider) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	// Convert messages
	messages := p.convertMessages(req)

	// Convert tools
	tools := p.convertTools(req.Tools)

	// Build request params
	model := req.Model
	if model == "" {
		model = p.model
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	// Create streaming request
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			// Handle content deltas
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				events <- StreamEvent{
					Type:  StreamEventText,
					Delta: chunk.Choices[0].Delta.Content,
				}
			}

			// Handle completed tool calls
			if tool, ok := acc.JustFinishedToolCall(); ok {
				events <- StreamEvent{
					Type: StreamEventToolCall,
					ToolCall: &ToolCall{
						ID:        tool.ID,
						Name:      tool.Name,
						Arguments: json.RawMessage(tool.Arguments),
					},
				}
			}
		}

		if err := stream.Err(); err != nil {
			events <- StreamEvent{
				Type:  StreamEventError,
				Error: err,
			}
			return
		}

		// Send done event with usage info
		usage := &Usage{}
		if acc.Usage.PromptTokens > 0 || acc.Usage.CompletionTokens > 0 {
			usage.InputTokens = int(acc.Usage.PromptTokens)
			usage.OutputTokens = int(acc.Usage.CompletionTokens)
			usage.TotalTokens = int(acc.Usage.TotalTokens)
		}

		stopReason := ""
		if len(acc.Choices) > 0 {
			stopReason = string(acc.Choices[0].FinishReason)
		}

		events <- StreamEvent{
			Type:       StreamEventDone,
			StopReason: stopReason,
			Usage:      usage,
		}
	}()

	return events, nil
}

// convertMessages converts our messages to OpenAI format
func (p *OpenAIProvider) convertMessages(req *ChatRequest) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion

	// Add system message if provided
	if req.System != "" {
		result = append(result, openai.SystemMessage(req.System))
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			result = append(result, openai.UserMessage(msg.Content))

		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(msg.ToolCalls))
				for i, tc := range msg.ToolCalls {
					toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(tc.Arguments),
							},
						},
					}
				}
				result = append(result, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Content:   openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(msg.Content)},
						ToolCalls: toolCalls,
					},
				})
			} else {
				result = append(result, openai.AssistantMessage(msg.Content))
			}

		case "tool":
			result = append(result, openai.ToolMessage(msg.Content, msg.ToolCallID))
		}
	}

	return result
}

// convertTools converts our tools to OpenAI format
func (p *OpenAIProvider) convertTools(tools []Tool) []openai.ChatCompletionToolUnionParam {
	var result []openai.ChatCompletionToolUnionParam

	for _, tool := range tools {
		result = append(result, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  shared.FunctionParameters(tool.Parameters),
		}))
	}

	return result
}

// convertResponse converts an OpenAI response to our format
func (p *OpenAIProvider) convertResponse(resp *openai.ChatCompletion) *ChatResponse {
	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
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
