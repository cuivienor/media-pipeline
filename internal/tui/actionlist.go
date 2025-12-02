package tui

import (
	"fmt"
	"strings"

	"github.com/petercsiba/media-pipeline/internal/model"
)

// renderActionNeeded renders the action needed view
func (a *App) renderActionNeeded() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Action Needed"))
	b.WriteString("\n\n")

	ready := a.state.ItemsReadyForNextStage()
	inProgress := a.state.ItemsInProgress()
	failed := a.state.ItemsFailed()

	if len(ready) == 0 && len(inProgress) == 0 && len(failed) == 0 {
		b.WriteString(mutedItemStyle.Render("No items need attention.\n"))
		b.WriteString(helpStyle.Render("\n[Tab] Overview  [r] Refresh  [q] Quit"))
		return b.String()
	}

	// Track overall index for cursor
	index := 0

	// Ready for next stage
	if len(ready) > 0 {
		b.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("READY FOR NEXT STAGE (%d)", len(ready))))
		b.WriteString("\n")
		for _, item := range ready {
			b.WriteString(a.renderActionItem(item, index, "→"))
			b.WriteString("\n")
			index++
		}
	}

	// In progress
	if len(inProgress) > 0 {
		b.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("IN PROGRESS (%d)", len(inProgress))))
		b.WriteString("\n")
		for _, item := range inProgress {
			b.WriteString(a.renderActionItem(item, index, "●"))
			b.WriteString("\n")
			index++
		}
	}

	// Failed
	if len(failed) > 0 {
		b.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("FAILED (%d)", len(failed))))
		b.WriteString("\n")
		for _, item := range failed {
			b.WriteString(a.renderActionItem(item, index, "✗"))
			b.WriteString("\n")
			index++
		}
	}

	// Help
	b.WriteString(helpStyle.Render("\n[↑/↓] Navigate  [Enter] Details  [Tab] Overview  [r] Refresh  [q] Quit"))

	return b.String()
}

// renderActionItem renders a single action item line
func (a *App) renderActionItem(item model.MediaItem, index int, icon string) string {
	prefix := "  "
	if a.cursor == index {
		prefix = "> "
	}

	// Name with season for TV
	name := item.Name
	if item.Type == model.MediaTypeTV && item.Season != "" {
		name = fmt.Sprintf("%s %s", item.Name, item.Season)
	}

	// Current stage
	stage := item.Current.String()

	// Next action
	nextAction := item.Current.NextAction()

	line := fmt.Sprintf("%s%s %-28s %-12s → %s",
		prefix,
		icon,
		truncate(name, 28),
		stage,
		nextAction,
	)

	if a.cursor == index {
		return selectedItemStyle.Render(line)
	}
	return normalItemStyle.Render(line)
}
