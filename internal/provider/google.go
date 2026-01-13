package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GoogleProvider implements the Provider interface for Google AI
type GoogleProvider struct {
	client *genai.Client
	model  string
}

// NewGoogleProvider creates a new Google AI provider
func NewGoogleProvider(apiKey, model string) (*GoogleProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google AI client: %w", err)
	}

	return &GoogleProvider{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name
func (p *GoogleProvider) Name() string {
	return "google"
}

// Chat sends a chat request to Google AI
func (p *GoogleProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	model := p.client.GenerativeModel(req.Model)
	if req.Model == "" {
		model = p.client.GenerativeModel(p.model)
	}

	// Configure model
	if req.System != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(req.System)},
		}
	}

	// Add tools
	if len(req.Tools) > 0 {
		model.Tools = p.convertTools(req.Tools)
	}

	// Start chat session
	cs := model.StartChat()
	cs.History = p.convertHistory(req.Messages)

	// Get last user message
	var lastContent *genai.Content
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastContent = &genai.Content{
				Parts: []genai.Part{genai.Text(req.Messages[i].Content)},
				Role:  "user",
			}
			break
		}
		// Handle tool results
		if req.Messages[i].Role == "tool" {
			lastContent = &genai.Content{
				Parts: []genai.Part{
					genai.FunctionResponse{
						Name: req.Messages[i].ToolCallID, // Use tool call ID as function name
						Response: map[string]any{
							"result": req.Messages[i].Content,
						},
					},
				},
				Role: "user",
			}
			break
		}
	}

	if lastContent == nil {
		return nil, fmt.Errorf("no user message found")
	}

	// Send message
	resp, err := cs.SendMessage(ctx, lastContent.Parts...)
	if err != nil {
		return nil, fmt.Errorf("google ai request failed: %w", err)
	}

	// Convert response
	return p.convertResponse(resp), nil
}

// StreamChat streams a chat response from Google AI
func (p *GoogleProvider) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	model := p.client.GenerativeModel(req.Model)
	if req.Model == "" {
		model = p.client.GenerativeModel(p.model)
	}

	// Configure model
	if req.System != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(req.System)},
		}
	}

	// Add tools
	if len(req.Tools) > 0 {
		model.Tools = p.convertTools(req.Tools)
	}

	// Start chat session
	cs := model.StartChat()
	cs.History = p.convertHistory(req.Messages)

	// Get last user message
	var lastContent *genai.Content
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastContent = &genai.Content{
				Parts: []genai.Part{genai.Text(req.Messages[i].Content)},
				Role:  "user",
			}
			break
		}
		// Handle tool results
		if req.Messages[i].Role == "tool" {
			lastContent = &genai.Content{
				Parts: []genai.Part{
					genai.FunctionResponse{
						Name: req.Messages[i].ToolCallID,
						Response: map[string]any{
							"result": req.Messages[i].Content,
						},
					},
				},
				Role: "user",
			}
			break
		}
	}

	if lastContent == nil {
		return nil, fmt.Errorf("no user message found")
	}

	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		iter := cs.SendMessageStream(ctx, lastContent.Parts...)

		for {
			resp, err := iter.Next()
			if err != nil {
				if err.Error() == "iterator done" {
					events <- StreamEvent{
						Type:       StreamEventDone,
						StopReason: "stop",
					}
					break
				}
				events <- StreamEvent{
					Type:  StreamEventError,
					Error: err,
				}
				break
			}

			// Process response
			for _, cand := range resp.Candidates {
				if cand.Content == nil {
					continue
				}

				for _, part := range cand.Content.Parts {
					switch v := part.(type) {
					case genai.Text:
						events <- StreamEvent{
							Type:  StreamEventText,
							Delta: string(v),
						}
					case genai.FunctionCall:
						args, _ := json.Marshal(v.Args)
						events <- StreamEvent{
							Type: StreamEventToolCall,
							ToolCall: &ToolCall{
								ID:        v.Name, // Google uses function name as ID
								Name:      v.Name,
								Arguments: args,
							},
						}
					}
				}

				// Check for stop reason
				if cand.FinishReason != genai.FinishReasonUnspecified {
					events <- StreamEvent{
						Type:       StreamEventDone,
						StopReason: string(cand.FinishReason),
					}
				}
			}
		}
	}()

	return events, nil
}

// convertHistory converts our messages to Google AI format for chat history
func (p *GoogleProvider) convertHistory(messages []Message) []*genai.Content {
	var history []*genai.Content

	for i := 0; i < len(messages)-1; i++ { // Exclude last message as it will be sent
		msg := messages[i]
		switch msg.Role {
		case "user":
			history = append(history, &genai.Content{
				Parts: []genai.Part{genai.Text(msg.Content)},
				Role:  "user",
			})

		case "assistant":
			var parts []genai.Part
			if msg.Content != "" {
				parts = append(parts, genai.Text(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				json.Unmarshal(tc.Arguments, &args)
				parts = append(parts, genai.FunctionCall{
					Name: tc.Name,
					Args: args,
				})
			}
			history = append(history, &genai.Content{
				Parts: parts,
				Role:  "model",
			})

		case "tool":
			history = append(history, &genai.Content{
				Parts: []genai.Part{
					genai.FunctionResponse{
						Name: msg.ToolCallID,
						Response: map[string]any{
							"result": msg.Content,
						},
					},
				},
				Role: "user",
			})
		}
	}

	return history
}

// convertTools converts our tools to Google AI format
func (p *GoogleProvider) convertTools(tools []Tool) []*genai.Tool {
	var funcs []*genai.FunctionDeclaration

	for _, tool := range tools {
		// Convert parameters to genai.Schema
		schema := p.convertSchema(tool.Parameters)

		funcs = append(funcs, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  schema,
		})
	}

	return []*genai.Tool{{FunctionDeclarations: funcs}}
}

// convertSchema converts a JSON schema map to genai.Schema
func (p *GoogleProvider) convertSchema(params map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{
		Type: genai.TypeObject,
	}

	if props, ok := params["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range props {
			if propMap, ok := prop.(map[string]interface{}); ok {
				schema.Properties[name] = p.convertPropertySchema(propMap)
			}
		}
	}

	if required, ok := params["required"].([]interface{}); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	return schema
}

// convertPropertySchema converts a property schema
func (p *GoogleProvider) convertPropertySchema(prop map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{}

	if t, ok := prop["type"].(string); ok {
		switch t {
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		case "array":
			schema.Type = genai.TypeArray
		case "object":
			schema.Type = genai.TypeObject
		}
	}

	if desc, ok := prop["description"].(string); ok {
		schema.Description = desc
	}

	return schema
}

// convertResponse converts a Google AI response to our format
func (p *GoogleProvider) convertResponse(resp *genai.GenerateContentResponse) *ChatResponse {
	result := &ChatResponse{}

	if resp.UsageMetadata != nil {
		result.Usage = Usage{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:  int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}

		result.StopReason = string(cand.FinishReason)

		for _, part := range cand.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				result.Content += string(v)
			case genai.FunctionCall:
				args, _ := json.Marshal(v.Args)
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					ID:        v.Name,
					Name:      v.Name,
					Arguments: args,
				})
			}
		}
	}

	return result
}
