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
	ViewItemList     View = iota // Main view showing all items
	ViewItemDetail               // Movie or TV show detail
	ViewSeasonDetail             // Season detail for TV
	ViewOrganize                 // File organization view
	ViewNewItem                  // Create new item form
)

// App is the main application model
type App struct {
	config *config.Config
	repo   db.Repository
	state  *AppState
	err    error

	// Navigation state
	currentView    View
	selectedItem   *model.MediaItem
	selectedSeason *model.Season
	cursor         int

	// Window size
	width  int
	height int

	// Form state
	newItemForm *NewItemForm

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
		// Re-sync selectedItem and selectedSeason to point to new state objects
		a.syncSelectedFromState()
		return a, nil

	case itemCreatedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.currentView = ViewItemList
		a.newItemForm = nil
		return a, a.loadState

	case organizeLoadedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.organizeView = &OrganizeView{
			item:      msg.item,
			season:    msg.season,
			path:      msg.path,
			files:     msg.files,
			discFiles: msg.discFiles,
			discPaths: msg.discPaths,
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

	case ripStartedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		// Stay on current view but refresh state
		return a, a.loadState

	case seasonAddedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		// Stay on item detail but refresh state
		return a, a.loadState

	case seasonRipsDoneMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		// Stay on season detail but refresh state
		return a, a.loadState

	case stageStartedMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		// Stay on current view but refresh state
		return a, a.loadState
	}

	return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Route to form handler if in NewItem view
	if a.currentView == ViewNewItem && a.newItemForm != nil {
		return a.handleNewItemKey(msg)
	}

	// Route to organize handler if in Organize view
	if a.currentView == ViewOrganize {
		return a.handleOrganizeKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "r":
		// Refresh state
		return a, a.loadState

	case "n":
		// New item (only from item list view)
		if a.currentView == ViewItemList {
			a.currentView = ViewNewItem
			a.newItemForm = &NewItemForm{
				Type: "movie",
			}
			return a, nil
		}

	case "s":
		// Start next stage - works for movies (item detail) and TV seasons (season detail)
		if a.currentView == ViewItemDetail && a.selectedItem != nil {
			item := a.selectedItem
			// Can start if pending, failed, or completed (ready for next stage)
			if item.StageStatus == model.StatusPending || item.StageStatus == model.StatusFailed {
				return a, a.startStageForItem(item, item.CurrentStage)
			} else if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
				// Skip organize - that has its own flow with [o]
				if item.CurrentStage == model.StageRip {
					return a, nil // Use [o] for organize
				}
				return a, a.startStageForItem(item, item.CurrentStage.NextStage())
			}
		}
		if a.currentView == ViewSeasonDetail && a.selectedSeason != nil {
			season := a.selectedSeason
			// For TV seasons, allow ripping while pending OR in_progress (multi-disc)
			if season.CurrentStage == model.StageRip &&
				(season.StageStatus == model.StatusPending || season.StageStatus == model.StatusInProgress) {
				return a, a.startRipForSeason(a.selectedItem, season)
			}
			// Other stages
			if season.StageStatus == model.StatusPending || season.StageStatus == model.StatusFailed {
				return a, a.startStageForSeason(a.selectedItem, season, season.CurrentStage)
			} else if season.StageStatus == model.StatusCompleted && season.CurrentStage != model.StagePublish {
				if season.CurrentStage == model.StageRip {
					return a, nil // Use [o] for organize
				}
				return a, a.startStageForSeason(a.selectedItem, season, season.CurrentStage.NextStage())
			}
		}

	case "o":
		// Organize - works for movies (item detail) and TV seasons (season detail)
		if a.currentView == ViewItemDetail && a.selectedItem != nil {
			if a.selectedItem.Type == model.MediaTypeMovie {
				if a.selectedItem.CurrentStage == model.StageRip && a.selectedItem.StageStatus == model.StatusCompleted {
					return a, a.loadOrganizeView(a.selectedItem)
				}
			}
		}
		if a.currentView == ViewSeasonDetail && a.selectedSeason != nil {
			// Allow organize when rip stage is completed OR when there are completed rip jobs
			if a.selectedSeason.CurrentStage == model.StageRip && a.selectedSeason.StageStatus == model.StatusCompleted {
				return a, a.loadOrganizeViewForSeason(a.selectedItem, a.selectedSeason)
			}
		}

	case "a":
		// Add season (only from TV show item detail view)
		if a.currentView == ViewItemDetail && a.selectedItem != nil {
			if a.selectedItem.Type == model.MediaTypeTV {
				return a, a.addSeasonToItem(a.selectedItem)
			}
		}

	case "d":
		// Mark ripping done for season (only from season detail when ripping and not already completed)
		if a.currentView == ViewSeasonDetail && a.selectedSeason != nil {
			if a.selectedSeason.CurrentStage == model.StageRip && a.selectedSeason.StageStatus != model.StatusCompleted {
				return a, a.markSeasonRipsDone(a.selectedItem, a.selectedSeason)
			}
		}

	case "esc":
		// Go back
		switch a.currentView {
		case ViewItemDetail:
			a.currentView = ViewItemList
			a.selectedItem = nil
			a.cursor = 0
		case ViewSeasonDetail:
			a.currentView = ViewItemDetail
			a.selectedSeason = nil
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
		displayItems := a.getDisplayOrderItems()
		if len(displayItems) == 0 {
			return 0
		}
		return len(displayItems) - 1
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
		// Select item for detail - use display order, not raw items order
		displayItems := a.getDisplayOrderItems()
		if a.cursor < len(displayItems) {
			item := displayItems[a.cursor]
			a.selectedItem = &item
			a.currentView = ViewItemDetail
			a.cursor = 0
		}
		return a, nil

	case ViewItemDetail:
		// For TV shows, drill into season detail
		if a.selectedItem != nil && a.selectedItem.Type == model.MediaTypeTV {
			if a.cursor < len(a.selectedItem.Seasons) {
				a.selectedSeason = &a.selectedItem.Seasons[a.cursor]
				a.currentView = ViewSeasonDetail
				a.cursor = 0
			}
		}
		return a, nil
	}

	return a, nil
}

// syncSelectedFromState updates selectedItem and selectedSeason to reference
// the corresponding objects in the current state (after state reload)
func (a *App) syncSelectedFromState() {
	if a.state == nil {
		return
	}

	// Re-sync selectedItem
	if a.selectedItem != nil {
		for i := range a.state.Items {
			if a.state.Items[i].ID == a.selectedItem.ID {
				a.selectedItem = &a.state.Items[i]
				break
			}
		}
	}

	// Re-sync selectedSeason (requires selectedItem to be synced first)
	if a.selectedSeason != nil && a.selectedItem != nil {
		for i := range a.selectedItem.Seasons {
			if a.selectedItem.Seasons[i].ID == a.selectedSeason.ID {
				a.selectedSeason = &a.selectedItem.Seasons[i]
				break
			}
		}
	}
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
		return a.renderSeasonDetail()
	case ViewNewItem:
		return a.renderNewItemForm()
	case ViewOrganize:
		return a.renderOrganizeView()
	default:
		return "Unknown view"
	}
}
