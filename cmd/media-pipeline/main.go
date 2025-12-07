package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cuivienor/media-pipeline/internal/config"
	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/tui"
)

func main() {
	// Load configuration
	cfg, err := config.LoadFromMediaBase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Expected config at: $MEDIA_BASE/pipeline/config.yaml\n")
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(cfg.DatabasePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create repository
	repo := db.NewSQLiteRepository(database)

	// Create the app
	app := tui.NewApp(cfg, repo)

	// Create and run the Bubbletea program
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
