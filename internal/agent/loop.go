package agent

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nickcecere/btcx/internal/provider"
	"github.com/nickcecere/btcx/internal/storage"
)

// loopState tracks state during the agentic loop to detect stuck patterns
type loopState struct {
	// searchHistory tracks tool calls by hash to detect repetition
	searchHistory map[string]int

	// emptyResultCount tracks consecutive empty/no-result tool calls
	emptyResultCount int

	// totalSearches tracks total number of searches performed
	totalSearches int

	// hintInjected tracks if we've already injected a hint
	hintInjected bool
}

// newLoopState creates a new loop state tracker
func newLoopState() *loopState {
	return &loopState{
		searchHistory: make(map[string]int),
	}
}

// hashToolCall creates a hash of a tool call for deduplication
func hashToolCall(name string, args json.RawMessage) string {
	h := md5.New()
	h.Write([]byte(name))
	h.Write(args)
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// isEmptyResult checks if a tool result indicates no matches found
func isEmptyResult(result string) bool {
	result = strings.ToLower(result)
	return strings.Contains(result, "no files found") ||
		strings.Contains(result, "no matches") ||
		strings.Contains(result, "not found") ||
		len(result) < 30
}

// Response represents a response from the agent
type Response struct {
	// Content is the text response
	Content string

	// ToolCalls are any tool calls that were made
	ToolCalls []storage.ToolCall

	// Usage is the token usage
	Usage provider.Usage
}

// StreamCallback is called for each streaming event
type StreamCallback func(event provider.StreamEvent)

// Ask sends a question to the agent and returns the response
func (a *Agent) Ask(ctx context.Context, question string) (*Response, error) {
	return a.AskWithCallback(ctx, question, nil)
}

// AskWithCallback sends a question to the agent and streams the response
func (a *Agent) AskWithCallback(ctx context.Context, question string, callback StreamCallback) (*Response, error) {
	// Initialize thread if needed
	if a.Thread == nil {
		threadID := generateID()
		a.Thread = &storage.Thread{
			ID:        threadID,
			Title:     truncateTitle(question),
			Created:   time.Now(),
			Updated:   time.Now(),
			Resources: a.getResourceNames(),
			Provider:  string(a.ModelConfig.Provider),
			Model:     a.ModelConfig.Model,
			Messages:  []storage.Message{},
		}
		// Set thread ID for truncation output organization
		a.Tools.SetThreadID(threadID)
	}

	// Add user message
	userMsg := storage.Message{
		Role:      "user",
		Content:   question,
		Timestamp: time.Now(),
	}
	a.Thread.Messages = append(a.Thread.Messages, userMsg)

	// Run the agentic loop
	response, err := a.runLoop(ctx, callback)
	if err != nil {
		return nil, err
	}

	// Save thread
	if err := a.Storage.SaveThread(a.Thread); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save thread: %v\n", err)
	}

	return response, nil
}

// runLoop runs the agentic loop until completion
func (a *Agent) runLoop(ctx context.Context, callback StreamCallback) (*Response, error) {
	maxIterations := 10 // Prevent infinite loops
	totalUsage := provider.Usage{}
	var allToolCalls []storage.ToolCall
	state := newLoopState()

	for i := 0; i < maxIterations; i++ {
		// Build messages for the provider
		messages := a.buildMessages()

		// Build system prompt, adding hint if stuck
		systemPrompt := a.GetSystemPrompt()
		if state.hintInjected {
			systemPrompt += StuckLoopHint()
		}

		// Create chat request
		req := &provider.ChatRequest{
			Model:     a.ModelConfig.Model,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     a.GetTools(),
			MaxTokens: 8192,
		}

		var resp *provider.ChatResponse
		var err error

		// Use streaming mode unless provider is openai-compatible (may have non-standard streaming)
		useStreaming := callback != nil && a.ModelConfig.Provider != "openai-compatible"

		if useStreaming {
			// Streaming mode
			resp, err = a.streamChat(ctx, req, callback)
		} else {
			// Non-streaming mode
			resp, err = a.Provider.Chat(ctx, req)
		}

		if err != nil {
			return nil, fmt.Errorf("chat request failed: %w", err)
		}

		// Accumulate usage
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		// Add assistant message to thread
		assistantMsg := storage.Message{
			Role:      "assistant",
			Content:   resp.Content,
			Timestamp: time.Now(),
		}

		// Convert tool calls
		for _, tc := range resp.ToolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, storage.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
			allToolCalls = append(allToolCalls, storage.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}

		a.Thread.Messages = append(a.Thread.Messages, assistantMsg)

		// Check if we're done (no tool calls)
		if len(resp.ToolCalls) == 0 {
			content := resp.Content
			// If the model returns empty content, try to find any previous content
			if content == "" {
				for i := len(a.Thread.Messages) - 1; i >= 0; i-- {
					msg := a.Thread.Messages[i]
					if msg.Role == "assistant" && msg.Content != "" {
						content = msg.Content
						break
					}
				}
				// If still empty, provide a fallback
				if content == "" {
					content = "I was unable to generate a response for this query."
				}
			}
			return &Response{
				Content:   content,
				ToolCalls: allToolCalls,
				Usage:     totalUsage,
			}, nil
		}

		// Execute tool calls and track patterns
		hasUsefulResult := false
		hasRepeatedSearch := false

		for _, tc := range resp.ToolCalls {
			// Track this tool call
			hash := hashToolCall(tc.Name, tc.Arguments)
			state.searchHistory[hash]++
			state.totalSearches++

			// Check for repeated searches
			if state.searchHistory[hash] > 1 {
				hasRepeatedSearch = true
			}

			result, err := a.executeTool(ctx, tc, callback)

			// Add tool result message
			toolMsg := storage.Message{
				Role:       "tool",
				Content:    result,
				Timestamp:  time.Now(),
				ToolCallID: tc.ID,
			}

			if err != nil {
				toolMsg.ToolResults = []storage.ToolResult{{
					ToolCallID: tc.ID,
					Output:     "",
					Error:      err.Error(),
				}}
			} else {
				toolMsg.ToolResults = []storage.ToolResult{{
					ToolCallID: tc.ID,
					Output:     result,
				}}
				// Check if result has useful content
				if !isEmptyResult(result) {
					hasUsefulResult = true
				}
			}

			a.Thread.Messages = append(a.Thread.Messages, toolMsg)
		}

		// Track consecutive empty results to detect stuck loops
		if hasUsefulResult {
			state.emptyResultCount = 0
		} else {
			state.emptyResultCount++
		}

		// Inject hidden system hint if model is stuck
		// This is added to the system prompt context, not as a visible message
		if (state.emptyResultCount >= 2 || hasRepeatedSearch) && !state.hintInjected {
			state.hintInjected = true
			// We'll handle this by modifying the system prompt in the next iteration
		}

		// If we've had too many consecutive empty results, force completion
		if state.emptyResultCount >= 3 {
			return a.forceCompletion(allToolCalls, totalUsage)
		}

		// If we've done many searches without progress, force completion
		if state.totalSearches >= 8 && state.emptyResultCount >= 2 {
			return a.forceCompletion(allToolCalls, totalUsage)
		}
	}

	// Save thread even on failure for debugging
	if a.Thread != nil && len(a.Thread.Messages) > 0 {
		_ = a.Storage.SaveThread(a.Thread)
	}

	// Return what we have if there's any content
	if len(a.Thread.Messages) > 0 {
		for i := len(a.Thread.Messages) - 1; i >= 0; i-- {
			msg := a.Thread.Messages[i]
			if msg.Role == "assistant" && msg.Content != "" {
				return &Response{
					Content:   msg.Content + "\n\n[Note: Response may be incomplete due to iteration limit]",
					ToolCalls: allToolCalls,
					Usage:     totalUsage,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("max iterations reached")
}

// streamChat streams the chat response
func (a *Agent) streamChat(ctx context.Context, req *provider.ChatRequest, callback StreamCallback) (*provider.ChatResponse, error) {
	events, err := a.Provider.StreamChat(ctx, req)
	if err != nil {
		return nil, err
	}

	var content string
	var toolCalls []provider.ToolCall
	var usage provider.Usage
	var stopReason string

	for event := range events {
		// Forward event to callback
		if callback != nil {
			callback(event)
		}

		switch event.Type {
		case provider.StreamEventText:
			content += event.Delta
		case provider.StreamEventToolCall:
			if event.ToolCall != nil {
				toolCalls = append(toolCalls, *event.ToolCall)
			}
		case provider.StreamEventDone:
			if event.Usage != nil {
				usage = *event.Usage
			}
			stopReason = event.StopReason
		case provider.StreamEventError:
			return nil, event.Error
		}
	}

	return &provider.ChatResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage:      usage,
	}, nil
}

// executeTool executes a tool call
func (a *Agent) executeTool(ctx context.Context, tc provider.ToolCall, callback StreamCallback) (string, error) {
	// Notify callback about tool execution starting
	if callback != nil {
		callback(provider.StreamEvent{
			Type:     provider.StreamEventToolCall,
			ToolCall: &tc,
		})
	}

	result, err := a.Tools.Execute(ctx, tc.Name, tc.Arguments)

	// Notify callback about tool execution completing
	if callback != nil {
		callback(provider.StreamEvent{
			Type:     provider.StreamEventToolResult,
			ToolCall: &tc,
		})
	}

	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error()), nil // Return error as content, not as Go error
	}

	return result.Output, nil
}

// buildMessages builds the message list for the provider
func (a *Agent) buildMessages() []provider.Message {
	var messages []provider.Message

	for _, msg := range a.Thread.Messages {
		switch msg.Role {
		case "user":
			messages = append(messages, provider.Message{
				Role:    "user",
				Content: msg.Content,
			})

		case "assistant":
			providerMsg := provider.Message{
				Role:    "assistant",
				Content: msg.Content,
			}
			for _, tc := range msg.ToolCalls {
				providerMsg.ToolCalls = append(providerMsg.ToolCalls, provider.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			messages = append(messages, providerMsg)

		case "tool":
			content := msg.Content
			if len(msg.ToolResults) > 0 {
				if msg.ToolResults[0].Error != "" {
					content = fmt.Sprintf("Error: %s", msg.ToolResults[0].Error)
				} else {
					content = msg.ToolResults[0].Output
				}
			}
			messages = append(messages, provider.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: msg.ToolCallID,
			})
		}
	}

	return messages
}

// getResourceNames returns the names of resources in the collection
func (a *Agent) getResourceNames() []string {
	var names []string
	for _, r := range a.Collection.Resources {
		names = append(names, r.Name)
	}
	return names
}

// generateID generates a unique ID for a thread
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// truncateTitle truncates a title to a reasonable length
func truncateTitle(s string) string {
	if len(s) > 50 {
		return s[:47] + "..."
	}
	return s
}

// forceCompletion returns a response with whatever content has been accumulated
func (a *Agent) forceCompletion(allToolCalls []storage.ToolCall, totalUsage provider.Usage) (*Response, error) {
	// Find the last assistant message with content
	var lastContent string
	for i := len(a.Thread.Messages) - 1; i >= 0; i-- {
		msg := a.Thread.Messages[i]
		if msg.Role == "assistant" && msg.Content != "" {
			lastContent = msg.Content
			break
		}
	}

	// If no assistant content, try to summarize tool results
	if lastContent == "" {
		// Collect useful tool results
		var usefulResults []string
		for _, msg := range a.Thread.Messages {
			if msg.Role == "tool" && len(msg.Content) > 100 && !isEmptyResult(msg.Content) {
				// Truncate to reasonable size
				content := msg.Content
				if len(content) > 500 {
					content = content[:500] + "..."
				}
				usefulResults = append(usefulResults, content)
			}
		}

		if len(usefulResults) > 0 {
			lastContent = "Based on the search results, here is what I found:\n\n"
			for i, result := range usefulResults {
				if i >= 3 {
					break // Limit to 3 results
				}
				lastContent += result + "\n\n"
			}
			lastContent += "[Note: The model was unable to complete the response. Above are the raw search results.]"
		} else {
			lastContent = "I was unable to find specific information about this topic in the codebase after multiple searches. The search patterns used did not return relevant results. Try rephrasing your question or being more specific about what you're looking for."
		}
	}

	return &Response{
		Content:   lastContent,
		ToolCalls: allToolCalls,
		Usage:     totalUsage,
	}, nil
}

// ContinueThread continues an existing thread
func (a *Agent) ContinueThread(thread *storage.Thread) {
	a.Thread = thread
}

// GetThread returns the current thread
func (a *Agent) GetThread() *storage.Thread {
	return a.Thread
}

// ToolExecutionEvent represents a tool execution for callbacks
type ToolExecutionEvent struct {
	Name      string
	Arguments json.RawMessage
	Result    string
	Error     error
}
