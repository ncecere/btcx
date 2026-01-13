package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/nickcecere/btcx/internal/provider"
	"github.com/nickcecere/btcx/internal/ui"
)

// ansiEscapeRegex matches ANSI escape sequences and terminal garbage
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07?|\x1b\][^\x1b]*|\[\d+;\d+R|\]11;[^\]]*\\?|rgb:[0-9a-fA-F/]+\\?|\\\d+;\d+;\d+M|[0-9]+/[0-9]+/[0-9]+\\`)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	resourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	userStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33"))

	assistantStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))
)

// Messages for Bubble Tea
type streamChunkMsg string
type streamDoneMsg struct {
	content string
	err     error
}
type streamToolMsg string
type streamToolDoneMsg struct{}
type spinnerTickMsg struct{}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Clear any pending input that might contain escape sequences
	return tea.Batch(
		textarea.Blink,
		tea.ClearScreen,
	)
}

// spinnerTick returns a command that ticks the spinner
func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Filter out escape sequences that appear as runes
		if msg.Type == tea.KeyRunes {
			s := string(msg.Runes)
			if strings.Contains(s, "\x1b") || strings.Contains(s, "]11;") || strings.Contains(s, "rgb:") || strings.HasPrefix(s, "[") {
				// Ignore escape sequences
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if !msg.Alt && !m.streaming {
				// Submit the input - clean ANSI escape sequences
				question := strings.TrimSpace(m.input.Value())
				question = cleanInput(question)
				if question != "" {
					m.input.Reset()
					m.messages = append(m.messages, Message{
						Role:    "user",
						Content: question,
					})
					m.streaming = true
					m.currentChunk = ""
					m.currentTool = ""
					m.err = nil
					// Start spinner tick and ask question
					return m, tea.Batch(spinnerTick(), m.askQuestion(question))
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-8)
			m.viewport.YPosition = 3
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 8
		}
		m.input.SetWidth(msg.Width - 2)
		m.updateViewport()

	case spinnerTickMsg:
		if m.streaming {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(ui.SpinnerFrames())
			return m, spinnerTick()
		}

	case streamChunkMsg:
		m.currentChunk += string(msg)
		m.updateViewport()

	case streamToolMsg:
		m.currentTool = string(msg)

	case streamToolDoneMsg:
		m.currentTool = ""

	case streamDoneMsg:
		m.streaming = false
		m.currentTool = ""
		if msg.err != nil {
			m.err = msg.err
		} else {
			// Save the complete assistant message
			if msg.content != "" {
				m.messages = append(m.messages, Message{
					Role:    "assistant",
					Content: msg.content,
				})
			} else if m.currentChunk != "" {
				m.messages = append(m.messages, Message{
					Role:    "assistant",
					Content: m.currentChunk,
				})
			}
			m.currentChunk = ""
		}
		m.updateViewport()
	}

	// Update input
	if !m.streaming {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing..."
	}

	// Build the view
	var s strings.Builder

	// Header
	header := titleStyle.Render("btcx")
	resources := resourceStyle.Render(fmt.Sprintf(" [%s]", m.resourceNames()))
	s.WriteString(header + resources + "\n")
	s.WriteString(strings.Repeat("─", m.width) + "\n")

	// Messages viewport
	s.WriteString(m.viewport.View() + "\n")

	// Separator
	s.WriteString(strings.Repeat("─", m.width) + "\n")

	// Input or status
	if m.streaming {
		frames := ui.SpinnerFrames()
		frame := spinnerStyle.Render(frames[m.spinnerFrame])
		if m.currentTool != "" {
			s.WriteString(fmt.Sprintf("%s Using %s...\n", frame, m.currentTool))
		} else {
			s.WriteString(fmt.Sprintf("%s Thinking...\n", frame))
		}
	} else {
		// Clean the input view to remove escape sequences
		inputView := m.input.View()
		inputView = cleanInput(inputView)
		// Restore the box drawing if it got stripped
		if !strings.Contains(inputView, "Ask") && inputView == "" {
			inputView = "Ask a question..."
		}
		s.WriteString(inputView + "\n")
	}

	// Help
	help := helpStyle.Render("Enter: send | Ctrl+C: quit")
	if m.err != nil {
		help = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	s.WriteString(help)

	return s.String()
}

// updateViewport updates the viewport content
func (m *Model) updateViewport() {
	var content strings.Builder

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.width-4),
	)

	for i, msg := range m.messages {
		switch msg.Role {
		case "user":
			content.WriteString(userStyle.Render("You: "))
			content.WriteString(msg.Content)
			content.WriteString("\n\n")

		case "assistant":
			content.WriteString(assistantStyle.Render("Assistant: "))
			content.WriteString("\n")
			rendered, err := renderer.Render(msg.Content)
			if err != nil {
				content.WriteString(msg.Content)
			} else {
				content.WriteString(rendered)
			}
			content.WriteString("\n")

			// Add separator between Q&A pairs (but not after the last one)
			if i < len(m.messages)-1 {
				content.WriteString("\n")
				content.WriteString(strings.Repeat("─", m.width-4))
				content.WriteString("\n\n")
			}
		}
	}

	// Add streaming content
	if m.streaming && m.currentChunk != "" {
		content.WriteString(assistantStyle.Render("Assistant: "))
		content.WriteString("\n")
		rendered, err := renderer.Render(m.currentChunk)
		if err != nil {
			content.WriteString(m.currentChunk)
		} else {
			content.WriteString(rendered)
		}
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// askQuestion sends a question to the agent
func (m *Model) askQuestion(question string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var fullContent strings.Builder

		callback := func(event provider.StreamEvent) {
			switch event.Type {
			case provider.StreamEventText:
				fullContent.WriteString(event.Delta)
			case provider.StreamEventToolCall:
				if event.ToolCall != nil {
					// Note: Can't send tea.Msg from here directly
					// The tool name will be tracked via the model
				}
			case provider.StreamEventToolResult:
				// Tool finished
			}
		}

		resp, err := m.Agent.AskWithCallback(ctx, question, callback)
		if err != nil {
			return streamDoneMsg{err: err}
		}

		// Use response content if callback didn't capture anything
		// (happens with non-streaming providers like openai-compatible)
		content := fullContent.String()
		if content == "" && resp != nil {
			content = resp.Content
		}

		return streamDoneMsg{content: content}
	}
}

// resourceNames returns a comma-separated list of resource names
func (m *Model) resourceNames() string {
	var names []string
	for _, r := range m.Collection.Resources {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

// cleanInput removes ANSI escape sequences and terminal garbage from input
func cleanInput(s string) string {
	// Remove ANSI escape sequences
	s = ansiEscapeRegex.ReplaceAllString(s, "")
	// Remove common terminal garbage patterns
	s = regexp.MustCompile(`\d+/\d+/\d+`).ReplaceAllString(s, "")    // rgb values like 0000/0000/0000
	s = regexp.MustCompile(`\\+`).ReplaceAllString(s, "")            // backslashes
	s = regexp.MustCompile(`\[\d+;\d+;\d+M`).ReplaceAllString(s, "") // mouse sequences
	// Remove any remaining escape characters and control sequences
	s = strings.Map(func(r rune) rune {
		if r == '\x1b' || (r < 32 && r != '\n' && r != '\r' && r != '\t') {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// Run starts the TUI
func Run(m Model) error {
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Better mouse handling
	)
	_, err := p.Run()
	return err
}
