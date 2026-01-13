package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Storage handles persistent data storage
type Storage struct {
	dataDir string
}

// NewStorage creates a new storage instance
func NewStorage(dataDir string) *Storage {
	return &Storage{dataDir: dataDir}
}

// ThreadsDir returns the directory where threads are stored
func (s *Storage) ThreadsDir() string {
	return filepath.Join(s.dataDir, "threads")
}

// EnsureDirs creates all required directories
func (s *Storage) EnsureDirs() error {
	dirs := []string{
		s.dataDir,
		s.ThreadsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Thread represents a conversation thread
type Thread struct {
	// ID is the unique identifier for this thread
	ID string `json:"id"`

	// Title is a human-readable title for the thread
	Title string `json:"title"`

	// Created is when the thread was created
	Created time.Time `json:"created"`

	// Updated is when the thread was last updated
	Updated time.Time `json:"updated"`

	// Resources are the resource names used in this thread
	Resources []string `json:"resources"`

	// Provider is the AI provider used
	Provider string `json:"provider"`

	// Model is the model used
	Model string `json:"model"`

	// Messages are the conversation messages
	Messages []Message `json:"messages"`
}

// Message represents a single message in a conversation
type Message struct {
	// Role is the message role (user, assistant, tool)
	Role string `json:"role"`

	// Content is the message text content
	Content string `json:"content"`

	// ToolCalls are any tool calls made by the assistant
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`

	// ToolResults are the results from tool calls
	ToolResults []ToolResult `json:"toolResults,omitempty"`

	// ToolCallID is the ID of the tool call this message is responding to (for tool role)
	ToolCallID string `json:"toolCallId,omitempty"`

	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	// ID is the unique identifier for this tool call
	ID string `json:"id"`

	// Name is the tool name
	Name string `json:"name"`

	// Arguments is the JSON arguments passed to the tool
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	// ToolCallID is the ID of the tool call this result is for
	ToolCallID string `json:"toolCallId"`

	// Output is the tool output
	Output string `json:"output"`

	// Error is any error that occurred
	Error string `json:"error,omitempty"`
}

// SaveThread saves a thread to disk
func (s *Storage) SaveThread(thread *Thread) error {
	if err := s.EnsureDirs(); err != nil {
		return err
	}

	thread.Updated = time.Now()

	data, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal thread: %w", err)
	}

	path := filepath.Join(s.ThreadsDir(), thread.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write thread: %w", err)
	}

	return nil
}

// LoadThread loads a thread from disk
func (s *Storage) LoadThread(id string) (*Thread, error) {
	path := filepath.Join(s.ThreadsDir(), id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("thread %q not found", id)
		}
		return nil, fmt.Errorf("failed to read thread: %w", err)
	}

	var thread Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thread: %w", err)
	}

	return &thread, nil
}

// DeleteThread deletes a thread from disk
func (s *Storage) DeleteThread(id string) error {
	path := filepath.Join(s.ThreadsDir(), id+".json")

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("thread %q not found", id)
		}
		return fmt.Errorf("failed to delete thread: %w", err)
	}

	return nil
}

// ListThreads returns all threads, sorted by update time (newest first)
func (s *Storage) ListThreads() ([]*Thread, error) {
	entries, err := os.ReadDir(s.ThreadsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []*Thread{}, nil
		}
		return nil, fmt.Errorf("failed to read threads directory: %w", err)
	}

	var threads []*Thread
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		thread, err := s.LoadThread(id)
		if err != nil {
			continue // Skip invalid threads
		}
		threads = append(threads, thread)
	}

	// Sort by update time (newest first)
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].Updated.After(threads[j].Updated)
	})

	return threads, nil
}

// GetLatestThread returns the most recently updated thread
func (s *Storage) GetLatestThread() (*Thread, error) {
	threads, err := s.ListThreads()
	if err != nil {
		return nil, err
	}

	if len(threads) == 0 {
		return nil, fmt.Errorf("no threads found")
	}

	return threads[0], nil
}

// ClearThreads deletes all threads
func (s *Storage) ClearThreads() error {
	if err := os.RemoveAll(s.ThreadsDir()); err != nil {
		return fmt.Errorf("failed to clear threads: %w", err)
	}
	return nil
}
