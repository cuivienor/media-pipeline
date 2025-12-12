package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// statusDone is a special status for fully completed items (publish done)
const statusDone model.Status = "done"

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

	// Group items by category (each item appears in exactly one section)
	needsAction := a.filterItemsByCategory(model.StatusCompleted)
	inProgress := a.filterItemsByCategory(model.StatusInProgress)
	failed := a.filterItemsByCategory(model.StatusFailed)
	notStarted := a.filterItemsByCategory(model.StatusPending)
	done := a.filterItemsByCategory(statusDone)

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

	// Done section (fully completed items)
	if len(done) > 0 {
		b.WriteString(sectionHeaderStyle.Render("DONE"))
		b.WriteString("\n")
		for _, item := range done {
			selected := cursorIndex == a.cursor
			b.WriteString(a.renderItemRow(item, selected))
			b.WriteString("\n")
			cursorIndex++
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("[Enter] View  [n] New Item  [r] Refresh  [q] Quit"))

	return b.String()
}

// renderItemRow renders a single item row
func (a *App) renderItemRow(item model.MediaItem, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}

	// Get the effective status for display (rolled up for TV shows)
	effectiveStatus := a.categorizeItem(item)

	// Status indicator based on effective status
	var statusIcon string
	var statusStyle lipgloss.Style
	switch effectiveStatus {
	case model.StatusCompleted:
		statusIcon = "●"
		statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	case model.StatusInProgress:
		statusIcon = "◐"
		statusStyle = lipgloss.NewStyle().Foreground(colorWarning)
	case model.StatusFailed:
		statusIcon = "✗"
		statusStyle = lipgloss.NewStyle().Foreground(colorError)
	case statusDone:
		statusIcon = "✓"
		statusStyle = lipgloss.NewStyle().Foreground(colorSuccess)
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
		switch effectiveStatus {
		case model.StatusCompleted:
			if item.CurrentStage != model.StagePublish {
				actionHint = mutedItemStyle.Render(fmt.Sprintf(" → %s", item.CurrentStage.NextAction()))
			}
		case model.StatusInProgress:
			actionHint = mutedItemStyle.Render(fmt.Sprintf(" [%s]", item.CurrentStage.String()))
		case model.StatusPending:
			actionHint = mutedItemStyle.Render(fmt.Sprintf(" → start %s", item.CurrentStage.String()))
		case model.StatusFailed:
			actionHint = mutedItemStyle.Render(fmt.Sprintf(" [%s failed]", item.CurrentStage.String()))
		}
	} else if item.Type == model.MediaTypeTV {
		// For TV shows, summarize season states
		actionHint = a.getTVActionHint(item, effectiveStatus)
	}

	return fmt.Sprintf("%s%s %s %s%s",
		prefix,
		statusStyle.Render(statusIcon),
		typeBadge,
		name,
		actionHint,
	)
}

// getTVActionHint returns an action hint for a TV show based on season states
func (a *App) getTVActionHint(item model.MediaItem, effectiveStatus model.Status) string {
	if len(item.Seasons) == 0 {
		return mutedItemStyle.Render(" → add season")
	}

	// Count seasons by status and find the common next action
	var completed, inProgress, failed, pending int
	var nextAction string
	for _, season := range item.Seasons {
		switch season.StageStatus {
		case model.StatusCompleted:
			completed++
			// Track what the next action would be for completed seasons
			if season.CurrentStage != model.StagePublish {
				nextAction = season.CurrentStage.NextAction()
			}
		case model.StatusInProgress:
			inProgress++
		case model.StatusFailed:
			failed++
		default:
			pending++
		}
	}

	// Generate hint based on status
	switch effectiveStatus {
	case model.StatusFailed:
		return mutedItemStyle.Render(fmt.Sprintf(" [%d failed]", failed))
	case model.StatusInProgress:
		// Could be actively in progress, or a mix of completed+pending
		if inProgress > 0 {
			return mutedItemStyle.Render(" → finish ripping")
		}
		// Mix of completed and pending - show progress
		return mutedItemStyle.Render(fmt.Sprintf(" [%d/%d ripped]", completed, completed+pending))
	case model.StatusCompleted:
		// All seasons ready for next action - show what that action is
		if nextAction != "" {
			return mutedItemStyle.Render(fmt.Sprintf(" → %s", nextAction))
		}
		return mutedItemStyle.Render(" → done")
	default:
		return mutedItemStyle.Render(" → start ripping")
	}
}

// categorizeItem returns the display category for an item based on its most urgent status
// Priority: Failed > InProgress > Mixed (treated as InProgress) > AllCompleted > AllPending
// Items at publish stage with completed status are categorized as "done".
func (a *App) categorizeItem(item model.MediaItem) model.Status {
	if item.Type == model.MediaTypeMovie {
		// If publish is complete, the item is fully done
		if item.CurrentStage == model.StagePublish && item.StageStatus == model.StatusCompleted {
			return statusDone
		}
		return item.StageStatus
	}

	// For TV shows, categorize based on season states
	// A show is only "needs action" if ALL seasons are completed
	// If there's a mix of completed and pending, the show is still "in progress"
	hasFailed := false
	hasInProgress := false
	hasCompletedNeedsNext := false // completed but not at publish
	hasFullyDone := false          // publish completed
	hasPending := false

	for _, season := range item.Seasons {
		switch season.StageStatus {
		case model.StatusFailed:
			hasFailed = true
		case model.StatusInProgress:
			hasInProgress = true
		case model.StatusCompleted:
			if season.CurrentStage == model.StagePublish {
				hasFullyDone = true
			} else {
				hasCompletedNeedsNext = true
			}
		default:
			hasPending = true
		}
	}

	if hasFailed {
		return model.StatusFailed
	}
	if hasInProgress {
		return model.StatusInProgress
	}
	// Mix of completed and pending = still in progress (not all seasons done)
	if (hasCompletedNeedsNext || hasFullyDone) && hasPending {
		return model.StatusInProgress
	}
	// Some seasons need next stage
	if hasCompletedNeedsNext {
		return model.StatusCompleted
	}
	// All seasons fully done
	if hasFullyDone && !hasPending {
		return statusDone
	}
	return model.StatusPending
}

// filterItemsByCategory returns items that belong to the given category
func (a *App) filterItemsByCategory(targetStatus model.Status) []model.MediaItem {
	var result []model.MediaItem
	for _, item := range a.state.Items {
		if a.categorizeItem(item) == targetStatus {
			result = append(result, item)
		}
	}
	return result
}

// getDisplayOrderItems returns all items in the order they appear on screen
// (NEEDS ACTION, IN PROGRESS, FAILED, NOT STARTED, DONE)
func (a *App) getDisplayOrderItems() []model.MediaItem {
	var result []model.MediaItem
	result = append(result, a.filterItemsByCategory(model.StatusCompleted)...)
	result = append(result, a.filterItemsByCategory(model.StatusInProgress)...)
	result = append(result, a.filterItemsByCategory(model.StatusFailed)...)
	result = append(result, a.filterItemsByCategory(model.StatusPending)...)
	result = append(result, a.filterItemsByCategory(statusDone)...)
	return result
}
