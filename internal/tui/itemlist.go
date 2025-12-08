package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemList renders the main item list view
func (a *App) renderItemList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Media Pipeline"))
	b.WriteString("\n\n")

	if a.state == nil || len(a.state.Items) == 0 {
		b.WriteString(mutedItemStyle.Render("No active items. Press [n] to add one."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("[n] New Item  [h] History  [q] Quit"))
		return b.String()
	}

	// Group items by status
	needsAction := a.filterItemsByStatus(model.StatusCompleted)
	inProgress := a.filterItemsByStatus(model.StatusInProgress)
	failed := a.filterItemsByStatus(model.StatusFailed)
	notStarted := a.filterItemsByItemStatus(model.ItemStatusNotStarted)

	cursorIndex := 0

	// Needs Action section
	if len(needsAction) > 0 {
		b.WriteString(sectionHeaderStyle.Render("NEEDS ACTION"))
		b.WriteString("\n")
		for _, item := range needsAction {
			selected := cursorIndex == a.cursor
			b.WriteString(a.renderItemRow(item, selected))
			b.WriteString("\n")
			cursorIndex++
		}
		b.WriteString("\n")
	}

	// In Progress section
	if len(inProgress) > 0 {
		b.WriteString(sectionHeaderStyle.Render("IN PROGRESS"))
		b.WriteString("\n")
		for _, item := range inProgress {
			selected := cursorIndex == a.cursor
			b.WriteString(a.renderItemRow(item, selected))
			b.WriteString("\n")
			cursorIndex++
		}
		b.WriteString("\n")
	}

	// Failed section
	if len(failed) > 0 {
		b.WriteString(sectionHeaderStyle.Render("FAILED"))
		b.WriteString("\n")
		for _, item := range failed {
			selected := cursorIndex == a.cursor
			b.WriteString(a.renderItemRow(item, selected))
			b.WriteString("\n")
			cursorIndex++
		}
		b.WriteString("\n")
	}

	// Not Started section
	if len(notStarted) > 0 {
		b.WriteString(sectionHeaderStyle.Render("NOT STARTED"))
		b.WriteString("\n")
		for _, item := range notStarted {
			selected := cursorIndex == a.cursor
			b.WriteString(a.renderItemRow(item, selected))
			b.WriteString("\n")
			cursorIndex++
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("[Enter] View  [n] New Item  [r] Refresh  [h] History  [q] Quit"))

	return b.String()
}

// renderItemRow renders a single item row
func (a *App) renderItemRow(item model.MediaItem, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}

	// Status indicator
	var statusIcon string
	var statusStyle lipgloss.Style
	switch item.StageStatus {
	case model.StatusCompleted:
		statusIcon = "●"
		statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	case model.StatusInProgress:
		statusIcon = "◐"
		statusStyle = lipgloss.NewStyle().Foreground(colorWarning)
	case model.StatusFailed:
		statusIcon = "✗"
		statusStyle = lipgloss.NewStyle().Foreground(colorError)
	default:
		statusIcon = "○"
		statusStyle = lipgloss.NewStyle().Foreground(colorMuted)
	}

	// Type badge
	typeBadge := "[M]"
	if item.Type == model.MediaTypeTV {
		typeBadge = "[TV]"
	}

	// Build row
	name := item.Name
	if item.Type == model.MediaTypeTV && len(item.Seasons) > 0 {
		name = fmt.Sprintf("%s (%d seasons)", item.Name, len(item.Seasons))
	}

	// Next action hint
	var actionHint string
	if item.Type == model.MediaTypeMovie {
		if item.StageStatus == model.StatusCompleted && item.CurrentStage != model.StagePublish {
			actionHint = mutedItemStyle.Render(fmt.Sprintf(" → %s", item.CurrentStage.NextAction()))
		} else if item.StageStatus == model.StatusInProgress {
			actionHint = mutedItemStyle.Render(fmt.Sprintf(" [%s]", item.CurrentStage.String()))
		}
	}

	return fmt.Sprintf("%s%s %s %s%s",
		prefix,
		statusStyle.Render(statusIcon),
		typeBadge,
		name,
		actionHint,
	)
}

// filterItemsByStatus returns items with the given stage status (for movies)
func (a *App) filterItemsByStatus(status model.Status) []model.MediaItem {
	var result []model.MediaItem
	for _, item := range a.state.Items {
		if item.Type == model.MediaTypeMovie && item.StageStatus == status {
			result = append(result, item)
		}
		// For TV shows, check if any season matches
		if item.Type == model.MediaTypeTV {
			for _, season := range item.Seasons {
				if season.StageStatus == status {
					result = append(result, item)
					break
				}
			}
		}
	}
	return result
}

// filterItemsByItemStatus returns items with the given item status
func (a *App) filterItemsByItemStatus(status model.ItemStatus) []model.MediaItem {
	var result []model.MediaItem
	for _, item := range a.state.Items {
		if item.ItemStatus == status {
			result = append(result, item)
		}
	}
	return result
}
