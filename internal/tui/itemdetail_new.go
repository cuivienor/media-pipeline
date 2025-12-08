package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemDetailNew renders the detail view for a movie or TV show
func (a *App) renderItemDetailNew() string {
	if a.selectedItem == nil {
		return "No item selected"
	}

	item := a.selectedItem

	if item.Type == model.MediaTypeTV {
		return a.renderTVShowDetail(item)
	}
	return a.renderMovieDetail(item)
}

// renderMovieDetail renders detail view for a movie
func (a *App) renderMovieDetail(item *model.MediaItem) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(item.Name))
	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("Movie"))
	b.WriteString("\n\n")

	// Current State
	b.WriteString(sectionHeaderStyle.Render("CURRENT STATE"))
	b.WriteString("\n")

	stageStyle := lipgloss.NewStyle()
	switch item.StageStatus {
	case model.StatusCompleted:
		stageStyle = stageStyle.Foreground(colorSuccess)
	case model.StatusInProgress:
		stageStyle = stageStyle.Foreground(colorWarning)
	case model.StatusFailed:
		stageStyle = stageStyle.Foreground(colorError)
	}

	b.WriteString(fmt.Sprintf("  Stage: %s\n", item.CurrentStage.DisplayName()))
	b.WriteString(fmt.Sprintf("  Status: %s\n", stageStyle.Render(string(item.StageStatus))))
	b.WriteString("\n")

	// Next Action
	if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		nextStage := item.CurrentStage.NextStage()
		b.WriteString(fmt.Sprintf("  Press [Enter] to %s\n", nextStage.String()))
		b.WriteString("\n")
	} else if item.StageStatus == model.StatusFailed {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString("  Press [Enter] to retry\n")
		b.WriteString("\n")
	}

	// Job History (collapsible in future)
	jobs := a.state.Jobs[item.ID]
	if len(jobs) > 0 {
		b.WriteString(sectionHeaderStyle.Render("HISTORY"))
		b.WriteString("\n")
		for _, job := range jobs {
			statusIcon := "○"
			switch job.Status {
			case model.JobStatusCompleted:
				statusIcon = "✓"
			case model.JobStatusInProgress:
				statusIcon = "◐"
			case model.JobStatusFailed:
				statusIcon = "✗"
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, job.Stage.DisplayName()))
		}
		b.WriteString("\n")
	}

	// Help
	helpText := "[Enter] Next Action  [l] View Logs  [f] View Files  [Esc] Back  [q] Quit"
	if item.CurrentStage == model.StageRip && item.StageStatus == model.StatusCompleted {
		helpText = "[o] Organize  [l] View Logs  [f] View Files  [Esc] Back  [q] Quit"
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// renderTVShowDetail renders detail view for a TV show
func (a *App) renderTVShowDetail(item *model.MediaItem) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(item.Name))
	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("TV Show"))
	b.WriteString("\n\n")

	// Seasons list
	b.WriteString(sectionHeaderStyle.Render("SEASONS"))
	b.WriteString("\n")

	if len(item.Seasons) == 0 {
		b.WriteString(mutedItemStyle.Render("  No seasons. Press [a] to add a season."))
		b.WriteString("\n")
	} else {
		for i, season := range item.Seasons {
			selected := i == a.cursor
			prefix := "  "
			if selected {
				prefix = "> "
			}

			statusIcon := "○"
			switch season.StageStatus {
			case model.StatusCompleted:
				if season.CurrentStage == model.StagePublish {
					statusIcon = "✓"
				} else {
					statusIcon = "●"
				}
			case model.StatusInProgress:
				statusIcon = "◐"
			case model.StatusFailed:
				statusIcon = "✗"
			}

			stageName := season.CurrentStage.DisplayName()
			b.WriteString(fmt.Sprintf("%s%s Season %d - %s (%s)\n",
				prefix, statusIcon, season.Number, stageName, season.StageStatus))
		}
	}
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("[Enter] View Season  [a] Add Season  [r] Start Rip  [Esc] Back  [q] Quit"))

	return b.String()
}
