package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cuivienor/media-pipeline/internal/config"
	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// View represents the current view
type View int

const (
	ViewOverview View = iota
	ViewStageList
	ViewActionNeeded
	ViewItemDetail
)

// App is the main application model
type App struct {
	config *config.Config
	repo   db.Repository
	state  *PipelineState
	err    error

	// Navigation state
	currentView   View
	selectedStage model.Stage
	selectedItem  *model.MediaItem
	cursor        int

	// Window size
	width  int
	height int
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, repo db.Repository) *App {
	return &App{
		config:      cfg,
		repo:        repo,
		currentView: ViewOverview,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return a.loadState
}

// stateMsg is sent when state loading completes
type stateMsg struct {
	state *PipelineState
	err   error
}

// loadState loads pipeline state from the database
func (a *App) loadState() tea.Msg {
	state, err := LoadState(a.repo)
	return stateMsg{state: state, err: err}
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case stateMsg:
		a.state = msg.state
		a.err = msg.err
		return a, nil
	}

	return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "r":
		// Refresh
		return a, a.loadState

	case "tab":
		// Toggle between overview and action needed
		if a.currentView == ViewOverview {
			a.currentView = ViewActionNeeded
		} else {
			a.currentView = ViewOverview
		}
		a.cursor = 0
		return a, nil

	case "esc":
		// Go back
		switch a.currentView {
		case ViewStageList:
			a.currentView = ViewOverview
			a.cursor = int(a.selectedStage)
		case ViewItemDetail:
			a.currentView = ViewStageList
			a.cursor = 0
		case ViewActionNeeded:
			a.currentView = ViewOverview
			a.cursor = 0
		}
		return a, nil

	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
		return a, nil

	case "down", "j":
		a.cursor++
		// Clamp cursor based on current view
		maxCursor := a.getMaxCursor()
		if a.cursor > maxCursor {
			a.cursor = maxCursor
		}
		return a, nil

	case "enter":
		return a.handleEnter()
	}

	return a, nil
}

// getMaxCursor returns the maximum cursor position for the current view
func (a *App) getMaxCursor() int {
	if a.state == nil {
		return 0
	}

	switch a.currentView {
	case ViewOverview:
		return 3 // 4 stages (0-3)
	case ViewStageList:
		items := a.state.ItemsAtStage(a.selectedStage)
		if len(items) == 0 {
			return 0
		}
		return len(items) - 1
	case ViewActionNeeded:
		ready := a.state.ItemsReadyForNextStage()
		inProgress := a.state.ItemsInProgress()
		failed := a.state.ItemsFailed()
		total := len(ready) + len(inProgress) + len(failed)
		if total == 0 {
			return 0
		}
		return total - 1
	default:
		return 0
	}
}

// handleEnter handles the enter key
func (a *App) handleEnter() (tea.Model, tea.Cmd) {
	if a.state == nil {
		return a, nil
	}

	switch a.currentView {
	case ViewOverview:
		// Drill into stage
		a.selectedStage = model.Stage(a.cursor)
		a.currentView = ViewStageList
		a.cursor = 0
		return a, nil

	case ViewStageList:
		// Select item for detail
		items := a.state.ItemsAtStage(a.selectedStage)
		if a.cursor < len(items) {
			a.selectedItem = &items[a.cursor]
			a.currentView = ViewItemDetail
		}
		return a, nil

	case ViewActionNeeded:
		// Select item for detail
		item := a.getActionNeededItem(a.cursor)
		if item != nil {
			a.selectedItem = item
			a.currentView = ViewItemDetail
		}
		return a, nil
	}

	return a, nil
}

// getActionNeededItem returns the item at the given index in the action needed list
func (a *App) getActionNeededItem(index int) *model.MediaItem {
	if a.state == nil {
		return nil
	}

	ready := a.state.ItemsReadyForNextStage()
	inProgress := a.state.ItemsInProgress()
	failed := a.state.ItemsFailed()

	if index < len(ready) {
		return &ready[index]
	}
	index -= len(ready)

	if index < len(inProgress) {
		return &inProgress[index]
	}
	index -= len(inProgress)

	if index < len(failed) {
		return &failed[index]
	}

	return nil
}

// View implements tea.Model
func (a *App) View() string {
	if a.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit.", a.err)
	}

	if a.state == nil {
		return "Scanning pipeline..."
	}

	switch a.currentView {
	case ViewOverview:
		return a.renderOverview()
	case ViewStageList:
		return a.renderStageList()
	case ViewActionNeeded:
		return a.renderActionNeeded()
	case ViewItemDetail:
		return a.renderItemDetail()
	default:
		return "Unknown view"
	}
}
