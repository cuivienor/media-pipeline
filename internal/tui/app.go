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
	ViewItemList View = iota  // NEW: Main view
	ViewItemDetail            // NEW: Movie or TV show detail
	ViewSeasonDetail          // NEW: Season detail for TV
	ViewOrganize
	ViewNewItem // Renamed from ViewNewRip
	// Removed: ViewOverview, ViewStageList, ViewActionNeeded
)

// App is the main application model
type App struct {
	config *config.Config
	repo   db.Repository
	state  *AppState
	err    error

	// Navigation state
	currentView   View
	selectedStage model.Stage
	selectedItem  *model.MediaItem
	cursor        int

	// Window size
	width  int
	height int

	// Form state
	newRipForm *NewRipForm

	// Organize view state
	organizeView *OrganizeView
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, repo db.Repository) *App {
	return &App{
		config:      cfg,
		repo:        repo,
		currentView: ViewItemList,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return a.loadState
}

// stateMsg is sent when state loading completes
type stateMsg struct {
	state *AppState
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

	case ripCompleteMsg:
		if msg.err != nil {
			a.err = msg.err
		}
		a.currentView = ViewItemList
		return a, a.loadState

	case organizeLoadedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.organizeView = &OrganizeView{
			item:  msg.item,
			path:  msg.path,
			files: msg.files,
		}
		a.currentView = ViewOrganize
		return a, nil

	case validateMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		if a.organizeView != nil {
			a.organizeView.validation = msg.result
		}
		return a, nil

	case organizeCompleteMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		// Return to item list and refresh
		a.currentView = ViewItemList
		a.organizeView = nil
		return a, a.loadState
	}

	return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Route to form handler if in NewItem view
	if a.currentView == ViewNewItem {
		return a.handleNewRipKey(msg)
	}

	// Route to organize handler if in Organize view
	if a.currentView == ViewOrganize {
		return a.handleOrganizeKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "r":
		// Refresh
		return a, a.loadState

	case "n":
		// New item (only from item list view)
		if a.currentView == ViewItemList {
			a.currentView = ViewNewItem
			a.newRipForm = &NewRipForm{
				Type:     "movie",
				DiscPath: "disc:0",
			}
			return a, nil
		}

	case "o":
		// Organize (only from item detail view when at rip stage and completed)
		if a.currentView == ViewItemDetail && a.selectedItem != nil {
			if a.selectedItem.Current == model.StageRip && a.selectedItem.Status == model.StatusCompleted {
				return a, a.loadOrganizeView(a.selectedItem)
			}
		}

	case "esc":
		// Go back
		switch a.currentView {
		case ViewItemDetail:
			a.currentView = ViewItemList
			a.cursor = 0
		case ViewSeasonDetail:
			a.currentView = ViewItemDetail
			a.cursor = 0
		case ViewNewItem:
			a.currentView = ViewItemList
			a.cursor = 0
		case ViewOrganize:
			a.currentView = ViewItemDetail
			a.organizeView = nil
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
	case ViewItemList:
		if len(a.state.Items) == 0 {
			return 0
		}
		return len(a.state.Items) - 1
	case ViewItemDetail:
		// For TV shows showing seasons
		if a.selectedItem != nil && a.selectedItem.Type == model.MediaTypeTV {
			if len(a.selectedItem.Seasons) == 0 {
				return 0
			}
			return len(a.selectedItem.Seasons) - 1
		}
		return 0
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
	case ViewItemList:
		// Select item for detail
		if a.cursor < len(a.state.Items) {
			a.selectedItem = &a.state.Items[a.cursor]
			a.currentView = ViewItemDetail
			a.cursor = 0
		}
		return a, nil

	case ViewItemDetail:
		// For TV shows, drill into season detail
		if a.selectedItem != nil && a.selectedItem.Type == model.MediaTypeTV {
			if a.cursor < len(a.selectedItem.Seasons) {
				// Note: selectedSeason not yet added to App struct in this task
				a.currentView = ViewSeasonDetail
				a.cursor = 0
			}
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
	case ViewItemList:
		return a.renderItemList()
	case ViewItemDetail:
		return a.renderItemDetail()
	case ViewSeasonDetail:
		// Season detail not yet implemented in this task
		return "Season detail view - not yet implemented"
	case ViewNewItem:
		return a.renderNewRipForm()
	case ViewOrganize:
		return a.renderOrganizeView()
	default:
		return "Unknown view"
	}
}
