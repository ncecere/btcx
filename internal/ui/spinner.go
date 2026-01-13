package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner frames (braille pattern)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner manages an animated spinner for terminal output
type Spinner struct {
	message string
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start begins the spinner animation in a goroutine
func (s *Spinner) Start() {
	go s.run()
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

// UpdateMessage changes the spinner message (thread-safe)
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

func (s *Spinner) run() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	defer close(s.doneCh)

	i := 0
	for {
		select {
		case <-s.stopCh:
			// Clear the spinner line
			fmt.Print("\r\033[2K")
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.message
			s.mu.Unlock()
			fmt.Printf("\r%s %s", Highlight.Render(spinnerFrames[i]), msg)
			i = (i + 1) % len(spinnerFrames)
		}
	}
}

// SpinnerFrames returns the spinner frames for use in TUI components
func SpinnerFrames() []string {
	return spinnerFrames
}
