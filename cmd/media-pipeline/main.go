package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/petercsiba/media-pipeline/internal/scanner"
	"github.com/petercsiba/media-pipeline/internal/tui"
)

func main() {
	// Use default configuration (production paths)
	config := scanner.DefaultConfig()

	// Create the app
	app := tui.NewApp(config)

	// Create and run the Bubbletea program
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
