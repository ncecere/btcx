package ui

import (
	"github.com/charmbracelet/glamour"
)

// RenderMarkdown renders markdown content for terminal display
func RenderMarkdown(content string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return content, err
	}
	return renderer.Render(content)
}

// RenderMarkdownWidth renders markdown with a custom width
func RenderMarkdownWidth(content string, width int) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content, err
	}
	return renderer.Render(content)
}
