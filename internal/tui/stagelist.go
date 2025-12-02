package tui

import (
	"fmt"
	"strings"

	"github.com/petercsiba/media-pipeline/internal/model"
)

// renderStageList renders the list of items at a stage
func (a *App) renderStageList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(a.selectedStage.DisplayName()))
	b.WriteString("\n\n")

	items := a.state.ItemsAtStage(a.selectedStage)

	if len(items) == 0 {
		b.WriteString(mutedItemStyle.Render("No items at this stage.\n"))
		b.WriteString(helpStyle.Render("\n[Esc] Back  [r] Refresh  [q] Quit"))
		return b.String()
	}

	// Group by type
	var movies, tvShows []model.MediaItem
	for _, item := range items {
		if item.Type == model.MediaTypeMovie {
			movies = append(movies, item)
		} else {
			tvShows = append(tvShows, item)
		}
	}

	// Track overall index for cursor
	index := 0

	// Render movies
	if len(movies) > 0 {
		b.WriteString(sectionHeaderStyle.Render("MOVIES"))
		b.WriteString("\n")
		for _, item := range movies {
			b.WriteString(a.renderItemLine(item, index))
			b.WriteString("\n")
			index++
		}
	}

	// Render TV shows
	if len(tvShows) > 0 {
		b.WriteString(sectionHeaderStyle.Render("TV SHOWS"))
		b.WriteString("\n")
		for _, item := range tvShows {
			b.WriteString(a.renderItemLine(item, index))
			b.WriteString("\n")
			index++
		}
	}

	// Help
	b.WriteString(helpStyle.Render("\n[↑/↓] Navigate  [Enter] Details  [Esc] Back  [r] Refresh  [q] Quit"))

	return b.String()
}

// renderItemLine renders a single item line
func (a *App) renderItemLine(item model.MediaItem, index int) string {
	prefix := "  "
	if a.cursor == index {
		prefix = "> "
	}

	// Status icon
	icon := StatusIcon(string(item.Status))

	// Name with season for TV
	name := item.Name
	if item.Type == model.MediaTypeTV && item.Season != "" {
		name = fmt.Sprintf("%s %s", item.Name, item.Season)
	}

	// Date from most recent stage
	date := ""
	if len(item.Stages) > 0 {
		latestStage := item.Stages[len(item.Stages)-1]
		if !latestStage.StartedAt.IsZero() {
			date = latestStage.StartedAt.Format("2006-01-02")
		}
	}

	// Next action (for completed items)
	nextAction := ""
	if item.IsReadyForNextStage() {
		nextAction = "→ " + item.Current.NextAction()
	}

	line := fmt.Sprintf("%s%s %-30s %-12s %s",
		prefix,
		icon,
		truncate(name, 30),
		date,
		nextAction,
	)

	if a.cursor == index {
		return selectedItemStyle.Render(line)
	}
	return normalItemStyle.Render(line)
}

// truncate truncates a string to the given length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
