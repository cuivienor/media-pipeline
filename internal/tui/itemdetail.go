package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemDetail renders the detail view for a movie or TV show
func (a *App) renderItemDetail() string {
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
	if item.StageStatus == model.StatusCompleted && item.CurrentStage == model.StageRip {
		// Special case: after rip, use [o] for organize
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString("  Press [o] to organize files\n")
		b.WriteString("\n")
	} else if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		nextStage := item.CurrentStage.NextStage()
		b.WriteString(fmt.Sprintf("  Press [s] to start %s\n", nextStage.String()))
		b.WriteString("\n")
	} else if item.StageStatus == model.StatusPending {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Press [s] to start %s\n", item.CurrentStage.String()))
		b.WriteString("\n")
	} else if item.StageStatus == model.StatusFailed {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Press [s] to retry %s\n", item.CurrentStage.String()))
		b.WriteString("\n")
	}

	// Job History (collapsible in future)
	jobs := a.state.MovieJobs[item.ID]
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
	var helpText string
	if item.StageStatus == model.StatusInProgress {
		helpText = "[r] Refresh  [Esc] Back  [q] Quit"
	} else if item.CurrentStage == model.StageRip && item.StageStatus == model.StatusCompleted {
		helpText = "[o] Organize  [r] Refresh  [Esc] Back  [q] Quit"
	} else if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
		// Ready for next stage (remux, transcode, or publish)
		nextStage := item.CurrentStage.NextStage()
		helpText = fmt.Sprintf("[s] Start %s  [r] Refresh  [Esc] Back  [q] Quit", nextStage.String())
	} else if item.StageStatus == model.StatusPending || item.StageStatus == model.StatusFailed {
		helpText = fmt.Sprintf("[s] Start %s  [r] Refresh  [Esc] Back  [q] Quit", item.CurrentStage.String())
	} else {
		helpText = "[r] Refresh  [Esc] Back  [q] Quit"
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

			// Status icon with color
			var statusIcon string
			var iconStyle lipgloss.Style
			switch season.StageStatus {
			case model.StatusCompleted:
				if season.CurrentStage == model.StagePublish {
					statusIcon = "✓"
					iconStyle = lipgloss.NewStyle().Foreground(colorSuccess)
				} else {
					statusIcon = "●"
					iconStyle = lipgloss.NewStyle().Foreground(colorSuccess)
				}
			case model.StatusInProgress:
				statusIcon = "◐"
				iconStyle = lipgloss.NewStyle().Foreground(colorWarning)
			case model.StatusFailed:
				statusIcon = "✗"
				iconStyle = lipgloss.NewStyle().Foreground(colorError)
			default:
				statusIcon = "○"
				iconStyle = lipgloss.NewStyle().Foreground(colorMuted)
			}

			// Season name and stage
			seasonName := fmt.Sprintf("Season %d", season.Number)
			stageName := season.CurrentStage.DisplayName()

			// Status text with color
			var statusStyle lipgloss.Style
			switch season.StageStatus {
			case model.StatusCompleted:
				statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
			case model.StatusInProgress:
				statusStyle = lipgloss.NewStyle().Foreground(colorWarning)
			case model.StatusFailed:
				statusStyle = lipgloss.NewStyle().Foreground(colorError)
			default:
				statusStyle = lipgloss.NewStyle().Foreground(colorMuted)
			}

			// Next action hint for this season
			var actionHint string
			if season.StageStatus == model.StatusCompleted && season.CurrentStage != model.StagePublish {
				actionHint = mutedItemStyle.Render(fmt.Sprintf(" → %s", season.CurrentStage.NextAction()))
			} else if season.StageStatus == model.StatusPending {
				actionHint = mutedItemStyle.Render(" → start rip")
			}

			b.WriteString(fmt.Sprintf("%s%s %s - %s %s%s\n",
				prefix,
				iconStyle.Render(statusIcon),
				seasonName,
				stageName,
				statusStyle.Render(string(season.StageStatus)),
				actionHint))
		}
	}
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("[Enter] View Season  [a] Add Season  [r] Refresh  [Esc] Back  [q] Quit"))

	return b.String()
}
