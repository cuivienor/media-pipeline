package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderSeasonDetail renders the detail view for a TV season
func (a *App) renderSeasonDetail() string {
	if a.selectedItem == nil || a.selectedSeason == nil {
		return "No season selected"
	}

	item := a.selectedItem
	season := a.selectedSeason

	var b strings.Builder

	// Title
	title := fmt.Sprintf("%s - Season %d", item.Name, season.Number)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Current State
	b.WriteString(sectionHeaderStyle.Render("CURRENT STATE"))
	b.WriteString("\n")

	stageStyle := lipgloss.NewStyle()
	switch season.StageStatus {
	case model.StatusCompleted:
		stageStyle = stageStyle.Foreground(colorSuccess)
	case model.StatusInProgress:
		stageStyle = stageStyle.Foreground(colorWarning)
	case model.StatusFailed:
		stageStyle = stageStyle.Foreground(colorError)
	}

	b.WriteString(fmt.Sprintf("  Stage: %s\n", season.CurrentStage.DisplayName()))
	b.WriteString(fmt.Sprintf("  Status: %s\n", stageStyle.Render(string(season.StageStatus))))
	b.WriteString("\n")

	// Next Action
	if season.StageStatus == model.StatusCompleted && season.CurrentStage != model.StagePublish {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		nextStage := season.CurrentStage.NextStage()
		b.WriteString(fmt.Sprintf("  Press [Enter] to %s\n", nextStage.String()))
		b.WriteString("\n")
	} else if season.StageStatus == model.StatusFailed {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString("  Press [Enter] to retry\n")
		b.WriteString("\n")
	} else if season.StageStatus == model.StatusPending ||
		(season.CurrentStage == model.StageRip && season.StageStatus != model.StatusInProgress) {
		b.WriteString(sectionHeaderStyle.Render("NEXT ACTION"))
		b.WriteString("\n")
		b.WriteString("  Press [s] to rip a disc\n")
		b.WriteString("\n")
	}

	// Rip Jobs (for TV seasons, multiple discs)
	jobs := a.state.SeasonJobs[season.ID]
	ripJobs := filterJobsByStage(jobs, model.StageRip)
	if len(ripJobs) > 0 {
		b.WriteString(sectionHeaderStyle.Render("DISC RIPS"))
		b.WriteString("\n")
		for _, job := range ripJobs {
			statusIcon := "○"
			switch job.Status {
			case model.JobStatusCompleted:
				statusIcon = "✓"
			case model.JobStatusInProgress:
				statusIcon = "◐"
			case model.JobStatusFailed:
				statusIcon = "✗"
			}
			discLabel := "Disc"
			if job.Disc != nil {
				discLabel = fmt.Sprintf("Disc %d", *job.Disc)
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, discLabel))
		}
		b.WriteString("\n")
	}

	// Other Job History
	otherJobs := filterJobsExcludingStage(jobs, model.StageRip)
	if len(otherJobs) > 0 {
		b.WriteString(sectionHeaderStyle.Render("HISTORY"))
		b.WriteString("\n")
		for _, job := range otherJobs {
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

	// Help - show different options based on state
	var helpText string
	if season.CurrentStage == model.StageRip && season.StageStatus == model.StatusCompleted {
		helpText = "[o] Organize  [s] Rip Another Disc  [r] Refresh  [Esc] Back  [q] Quit"
	} else if season.CurrentStage == model.StageRip && len(ripJobs) > 0 {
		// Has rip jobs, can mark done or add more
		helpText = "[s] Rip Disc  [d] Done Ripping  [r] Refresh  [Esc] Back  [q] Quit"
	} else {
		helpText = "[s] Start Rip  [r] Refresh  [Esc] Back  [q] Quit"
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// filterJobsByStage returns jobs for a specific stage
func filterJobsByStage(jobs []model.Job, stage model.Stage) []model.Job {
	var result []model.Job
	for _, job := range jobs {
		if job.Stage == stage {
			result = append(result, job)
		}
	}
	return result
}

// filterJobsExcludingStage returns jobs excluding a specific stage
func filterJobsExcludingStage(jobs []model.Job, stage model.Stage) []model.Job {
	var result []model.Job
	for _, job := range jobs {
		if job.Stage != stage {
			result = append(result, job)
		}
	}
	return result
}
