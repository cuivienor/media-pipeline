package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/organize"
)

// OrganizeView holds state for the organize validation view
type OrganizeView struct {
	item       *model.MediaItem
	season     *model.Season // nil for movies, set for TV seasons
	files      []fileInfo
	discFiles  map[string][]fileInfo // files grouped by disc (for TV seasons)
	validation *organize.ValidationResult
	path       string   // base path (season directory for TV)
	discPaths  []string // disc directories within season (for TV)
}

type fileInfo struct {
	name  string
	size  string
	isDir bool
}

// renderOrganizeView renders the organize validation view
func (a *App) renderOrganizeView() string {
	if a.organizeView == nil {
		return "No item selected for organization"
	}

	ov := a.organizeView
	var b strings.Builder

	// Title
	title := fmt.Sprintf("Organize: %s", ov.item.Name)
	if ov.season != nil {
		title = fmt.Sprintf("Organize: %s S%02d", ov.item.Name, ov.season.Number)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Path
	b.WriteString(sectionHeaderStyle.Render("PATH"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", ov.path))

	// Files - different display for TV seasons with multiple discs
	if ov.season != nil && len(ov.discFiles) > 0 {
		// Show season-level files first (like _episodes, _extras)
		b.WriteString(sectionHeaderStyle.Render("SEASON FILES"))
		b.WriteString("\n")
		for _, f := range ov.files {
			icon := "  "
			if f.isDir {
				icon = "📁"
			}
			sizeStr := ""
			if f.size != "" {
				sizeStr = " " + mutedItemStyle.Render(f.size)
			}
			b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, f.name, sizeStr))
		}
		b.WriteString("\n")

		// Show disc files grouped by disc
		discNames := make([]string, 0, len(ov.discFiles))
		for name := range ov.discFiles {
			discNames = append(discNames, name)
		}
		sort.Strings(discNames)

		for _, discName := range discNames {
			files := ov.discFiles[discName]
			b.WriteString(sectionHeaderStyle.Render(discName))
			b.WriteString("\n")
			for _, f := range files {
				icon := "  "
				if f.isDir {
					icon = "📁"
				}
				sizeStr := ""
				if f.size != "" {
					sizeStr = " " + mutedItemStyle.Render(f.size)
				}
				b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, f.name, sizeStr))
			}
			b.WriteString("\n")
		}
	} else {
		// Movie view - single directory
		b.WriteString(sectionHeaderStyle.Render("FILES"))
		b.WriteString("\n")
		for _, f := range ov.files {
			icon := "  "
			if f.isDir {
				icon = "📁"
			}
			sizeStr := ""
			if f.size != "" {
				sizeStr = " " + mutedItemStyle.Render(f.size)
			}
			b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, f.name, sizeStr))
		}
		b.WriteString("\n")
	}

	// Validation result
	if ov.validation != nil {
		if ov.validation.Valid {
			b.WriteString(statusCompleted.String() + " ")
			b.WriteString(lipgloss.NewStyle().Foreground(colorSuccess).Render("Organization valid"))
			b.WriteString("\n")
		} else {
			b.WriteString(statusFailed.String() + " ")
			b.WriteString(errorStyle.Render("Organization invalid"))
			b.WriteString("\n")
			for _, err := range ov.validation.Errors {
				b.WriteString(fmt.Sprintf("  • %s\n", err))
			}
		}
		b.WriteString("\n")
	}

	// Instructions
	b.WriteString(sectionHeaderStyle.Render("INSTRUCTIONS"))
	b.WriteString("\n")
	if ov.item.Type == model.MediaTypeMovie {
		b.WriteString("  1. Move main feature to _main/\n")
		b.WriteString("  2. Move extras to _extras/ (optional)\n")
		b.WriteString("  3. Delete unwanted files from root\n")
	} else if len(ov.discPaths) > 0 {
		// Multi-disc TV season
		b.WriteString("  For each disc folder:\n")
		b.WriteString("  1. Create _episodes/ inside the disc folder\n")
		b.WriteString("  2. Move episodes to _episodes/\n")
		b.WriteString("  3. Name files: 01.mkv, 02.mkv, etc.\n")
		b.WriteString("  4. Move extras to _extras/ (optional)\n")
		b.WriteString("  5. Delete unwanted files from disc root\n")
	} else {
		// Single disc
		b.WriteString("  1. Create _episodes/ in season folder\n")
		b.WriteString("  2. Move episodes to _episodes/\n")
		b.WriteString("  3. Name files: 01.mkv, 02.mkv, etc.\n")
		b.WriteString("  4. Move extras to _extras/ (optional)\n")
		b.WriteString("  5. Delete unwanted files from root\n")
	}
	b.WriteString("\n")

	// Help
	helpText := "[v] Validate  [r] Refresh  [Esc] Back"
	if ov.validation != nil && ov.validation.Valid {
		helpText = "[c] Mark Complete  [v] Re-validate  [r] Refresh  [Esc] Back"
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// handleOrganizeKey handles key presses in the organize view
func (a *App) handleOrganizeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit

	case "esc":
		// Go back to appropriate detail view
		if a.organizeView != nil && a.organizeView.season != nil {
			a.currentView = ViewSeasonDetail
		} else {
			a.currentView = ViewItemDetail
		}
		a.organizeView = nil
		return a, nil

	case "v":
		// Validate organization
		return a, a.validateOrganization()

	case "c":
		// Mark complete (only if validated)
		if a.organizeView != nil && a.organizeView.validation != nil && a.organizeView.validation.Valid {
			return a, a.markOrganizeComplete()
		}
		return a, nil

	case "r":
		// Refresh file list
		if a.organizeView != nil && a.organizeView.item != nil {
			if a.organizeView.season != nil {
				return a, a.loadOrganizeViewForSeason(a.organizeView.item, a.organizeView.season)
			}
			return a, a.loadOrganizeView(a.organizeView.item)
		}
		return a, nil
	}

	return a, nil
}

type organizeLoadedMsg struct {
	item      *model.MediaItem
	season    *model.Season
	path      string
	files     []fileInfo
	discFiles map[string][]fileInfo
	discPaths []string
	err       error
}

// loadOrganizeView loads file list for organize view (movies)
func (a *App) loadOrganizeView(item *model.MediaItem) tea.Cmd {
	return func() tea.Msg {
		// Find the rip output directory
		path := a.findItemPath(item, model.StageRip)
		if path == "" {
			return organizeLoadedMsg{err: fmt.Errorf("could not find rip output for %s", item.Name)}
		}

		files, err := listDirectory(path)
		if err != nil {
			return organizeLoadedMsg{err: err}
		}

		return organizeLoadedMsg{
			item:  item,
			path:  path,
			files: files,
		}
	}
}

// loadOrganizeViewForSeason loads file list for a TV season (multiple discs)
func (a *App) loadOrganizeViewForSeason(item *model.MediaItem, season *model.Season) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Find all completed rip jobs for this season
		jobs, err := a.repo.ListJobsForMedia(ctx, item.ID)
		if err != nil {
			return organizeLoadedMsg{err: fmt.Errorf("failed to list jobs: %w", err)}
		}

		// Collect disc paths from completed rip jobs
		var discPaths []string
		discFiles := make(map[string][]fileInfo)

		for _, job := range jobs {
			if job.Stage == model.StageRip && job.Status == model.JobStatusCompleted {
				if job.SeasonID != nil && *job.SeasonID == season.ID && job.OutputDir != "" {
					discPaths = append(discPaths, job.OutputDir)
					files, err := listDirectory(job.OutputDir)
					if err == nil {
						discName := filepath.Base(job.OutputDir)
						discFiles[discName] = files
					}
				}
			}
		}

		if len(discPaths) == 0 {
			return organizeLoadedMsg{err: fmt.Errorf("no completed rip jobs found for %s Season %d", item.Name, season.Number)}
		}

		// Season base path is the parent of the disc directories
		seasonPath := filepath.Dir(discPaths[0])

		// Also list the season directory itself (for _episodes, _extras that user creates)
		seasonFiles, _ := listDirectory(seasonPath)

		return organizeLoadedMsg{
			item:      item,
			season:    season,
			path:      seasonPath,
			files:     seasonFiles,
			discFiles: discFiles,
			discPaths: discPaths,
		}
	}
}

type validateMsg struct {
	result *organize.ValidationResult
	err    error
}

// validateOrganization runs the organization validator
func (a *App) validateOrganization() tea.Cmd {
	return func() tea.Msg {
		if a.organizeView == nil {
			return validateMsg{err: fmt.Errorf("no item selected")}
		}

		validator := &organize.Validator{}
		var result organize.ValidationResult

		if a.organizeView.item.Type == model.MediaTypeMovie {
			result = validator.ValidateMovie(a.organizeView.path)
		} else {
			// For TV seasons, use multi-disc validation if we have disc paths
			if len(a.organizeView.discPaths) > 0 {
				result = validator.ValidateTVSeason(a.organizeView.discPaths)
			} else {
				// Single disc or legacy - validate season directory directly
				result = validator.ValidateTV(a.organizeView.path)
			}
		}

		return validateMsg{result: &result}
	}
}

type organizeCompleteMsg struct {
	err error
}

// markOrganizeComplete creates an organize job and marks it complete
func (a *App) markOrganizeComplete() tea.Cmd {
	return func() tea.Msg {
		if a.organizeView == nil || a.organizeView.validation == nil || !a.organizeView.validation.Valid {
			return organizeCompleteMsg{err: fmt.Errorf("cannot complete: organization not validated")}
		}

		ctx := context.Background()
		ov := a.organizeView

		// Create completed organize job
		now := time.Now()
		job := &model.Job{
			MediaItemID: ov.item.ID,
			Stage:       model.StageOrganize,
			Status:      model.JobStatusCompleted,
			OutputDir:   ov.path,
			StartedAt:   &now,
			CompletedAt: &now,
		}

		// Set SeasonID for TV seasons
		if ov.season != nil {
			job.SeasonID = &ov.season.ID
		}

		if err := a.repo.CreateJob(ctx, job); err != nil {
			return organizeCompleteMsg{err: err}
		}

		// Update stage to organize completed
		if ov.season != nil {
			// TV season - update season stage
			if err := a.repo.UpdateSeasonStage(ctx, ov.season.ID, model.StageOrganize, model.StatusCompleted); err != nil {
				return organizeCompleteMsg{err: fmt.Errorf("failed to update season stage: %w", err)}
			}
		} else {
			// Movie - update item stage
			if err := a.repo.UpdateMediaItemStage(ctx, ov.item.ID, model.StageOrganize, model.StatusCompleted); err != nil {
				return organizeCompleteMsg{err: fmt.Errorf("failed to update item stage: %w", err)}
			}
		}

		return organizeCompleteMsg{}
	}
}

// listDirectory returns files in a directory
func listDirectory(path string) ([]fileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []fileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		sizeStr := ""
		if !entry.IsDir() {
			sizeStr = formatSize(info.Size())
		}

		files = append(files, fileInfo{
			name:  entry.Name(),
			size:  sizeStr,
			isDir: entry.IsDir(),
		})
	}

	return files, nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// findItemPath finds the output path for an item at a given stage
func (a *App) findItemPath(item *model.MediaItem, stage model.Stage) string {
	ctx := context.Background()
	jobs, _ := a.repo.ListJobsForMedia(ctx, item.ID)

	for _, job := range jobs {
		if job.Stage == stage && job.Status == model.JobStatusCompleted {
			return job.OutputDir
		}
	}
	return ""
}
