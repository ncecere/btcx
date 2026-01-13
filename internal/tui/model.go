package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/nickcecere/btcx/internal/agent"
	"github.com/nickcecere/btcx/internal/config"
	"github.com/nickcecere/btcx/internal/resource"
)

// Model is the TUI application model
type Model struct {
	// Config is the application configuration
	Config *config.Config

	// Paths contains resolved paths
	Paths *config.Paths

	// Collection is the current resource collection
	Collection *resource.Collection

	// Agent is the AI agent
	Agent *agent.Agent

	// UI components
	input    textarea.Model
	viewport viewport.Model

	// State
	messages     []Message
	streaming    bool
	currentChunk string
	err          error
	width        int
	height       int
	ready        bool
	quitting     bool

	// Spinner state
	spinnerFrame int
	currentTool  string
}

// Message represents a chat message in the TUI
type Message struct {
	Role    string
	Content string
}

// NewModel creates a new TUI model
func NewModel(cfg *config.Config, paths *config.Paths, collection *resource.Collection, a *agent.Agent) Model {
	// Create textarea for input
	ta := textarea.New()
	ta.Placeholder = "Ask a question..."
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	// Disable some features that might cause escape sequence issues
	ta.Prompt = ""

	return Model{
		Config:     cfg,
		Paths:      paths,
		Collection: collection,
		Agent:      a,
		input:      ta,
		messages:   []Message{},
	}
}
