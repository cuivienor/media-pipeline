package tui

import (
	"fmt"
	"strings"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderOverview renders the pipeline overview
func (a *App) renderOverview() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Media Pipeline"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("Scanned at %s", a.state.ScannedAt.Format("15:04:05"))))
	b.WriteString("\n\n")

	// Calculate total items
	totalItems := len(a.state.Items)
	if totalItems == 0 {
		b.WriteString(mutedItemStyle.Render("No media items found in pipeline.\n"))
		b.WriteString(helpStyle.Render("\n[r] Refresh  [q] Quit"))
		return b.String()
	}

	// Get counts per stage
	counts := a.state.CountByStage()
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Render each stage
	stages := []model.Stage{
		model.StageRip,
		model.StageOrganize,
		model.StageRemux,
		model.StageTranscode,
		model.StagePublish,
	}

	for i, stage := range stages {
		count := counts[stage]

		// Calculate ready and in-progress counts for this stage
		items := a.state.ItemsAtStage(stage)
		ready := 0
		inProgress := 0
		for _, item := range items {
			if item.IsReadyForNextStage() {
				ready++
			}
			if item.IsInProgress() {
				inProgress++
			}
		}

		// Render line
		prefix := "  "
		if a.cursor == i {
			prefix = "> "
		}

		name := stage.DisplayName()
		barWidth := 30
		bar := RenderBar(count, maxCount, barWidth)

		// Format count and status
		countStr := fmt.Sprintf("%d item", count)
		if count != 1 {
			countStr += "s"
		}

		statusParts := []string{}
		if ready > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d ready", ready))
		}
		if inProgress > 0 {
			statusParts = append(statusParts, fmt.Sprintf("%d in progress", inProgress))
		}
		statusStr := ""
		if len(statusParts) > 0 {
			statusStr = " (" + strings.Join(statusParts, ", ") + ")"
		}

		line := fmt.Sprintf("%s%-12s %s  %s%s",
			prefix,
			name,
			bar,
			countStr,
			statusStr,
		)

		if a.cursor == i {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(normalItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("\n")
	readyCount := len(a.state.ItemsReadyForNextStage())
	inProgressCount := len(a.state.ItemsInProgress())
	failedCount := len(a.state.ItemsFailed())

	if readyCount > 0 || inProgressCount > 0 || failedCount > 0 {
		b.WriteString(sectionHeaderStyle.Render("Summary"))
		b.WriteString("\n")
		if readyCount > 0 {
			b.WriteString(fmt.Sprintf("  %s %d items ready for next stage\n",
				statusCompleted.String(), readyCount))
		}
		if inProgressCount > 0 {
			b.WriteString(fmt.Sprintf("  %s %d items in progress\n",
				statusInProgress.String(), inProgressCount))
		}
		if failedCount > 0 {
			b.WriteString(fmt.Sprintf("  %s %d items failed\n",
				statusFailed.String(), failedCount))
		}
	}

	// Help
	b.WriteString(helpStyle.Render("\n[↑/↓] Navigate  [Enter] Drill down  [Tab] Action view  [r] Refresh  [q] Quit"))

	return b.String()
}
