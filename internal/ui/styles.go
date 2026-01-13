// Package ui provides terminal UI components and styling for btcx.
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	ColorPrimary   = lipgloss.Color("39")  // Cyan
	ColorSecondary = lipgloss.Color("212") // Pink
	ColorSuccess   = lipgloss.Color("82")  // Green
	ColorWarning   = lipgloss.Color("214") // Orange
	ColorError     = lipgloss.Color("196") // Red
	ColorMuted     = lipgloss.Color("245") // Gray
	ColorHighlight = lipgloss.Color("226") // Yellow
)

// Text styles
var (
	Bold      = lipgloss.NewStyle().Bold(true)
	Italic    = lipgloss.NewStyle().Italic(true)
	Dim       = lipgloss.NewStyle().Foreground(ColorMuted)
	Highlight = lipgloss.NewStyle().Foreground(ColorHighlight)
	Header    = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
)

// Status styles
var (
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)
	Error   = lipgloss.NewStyle().Foreground(ColorError)
)

// Tool styles
var (
	Tool     = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	ToolName = lipgloss.NewStyle().Foreground(ColorSecondary)
)

// Usage styles
var (
	Usage = lipgloss.NewStyle().Foreground(ColorMuted)
)
