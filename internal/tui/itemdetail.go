package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuivienor/media-pipeline/internal/model"
)

// renderItemDetail renders the item detail view
func (a *App) renderItemDetail() string {
	if a.selectedItem == nil {
		return "No item selected"
	}

	item := a.selectedItem
	var b strings.Builder

	// Title
	title := item.Name
	if item.Type == model.MediaTypeTV && item.Season != "" {
		title = fmt.Sprintf("%s %s", item.Name, item.Season)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Basic info
	b.WriteString(sectionHeaderStyle.Render("INFO"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Type:          %s\n", item.Type))
	b.WriteString(fmt.Sprintf("  Current Stage: %s (%s)\n", item.Current.DisplayName(), item.Status))
	if item.IsReadyForNextStage() {
		b.WriteString(fmt.Sprintf("  Next Action:   %s\n", item.Current.NextAction()))
	}
	b.WriteString("\n")

	// Pipeline history
	b.WriteString(sectionHeaderStyle.Render("HISTORY"))
	b.WriteString("\n")

	// Define all stages in order
	allStages := []model.Stage{
		model.StageRip,
		model.StageOrganize,
		model.StageRemux,
		model.StageTranscode,
		model.StagePublish,
	}

	for _, stage := range allStages {
		stageInfo := item.GetStageInfo(stage)
		if stageInfo != nil {
			// Stage exists
			icon := StatusIcon(string(stageInfo.Status))
			date := ""
			if !stageInfo.StartedAt.IsZero() {
				date = stageInfo.StartedAt.Format("2006-01-02 15:04")
			}
			b.WriteString(fmt.Sprintf("  %s %-12s %s\n", icon, stage.String(), date))
			if stageInfo.Path != "" {
				b.WriteString(fmt.Sprintf("    %s\n", mutedItemStyle.Render(stageInfo.Path)))
			}
		} else if stage <= item.Current {
			// Stage should have happened but we don't have info
			b.WriteString(fmt.Sprintf("  %s %-12s (no data)\n", statusPending.String(), stage.String()))
		} else {
			// Stage is in the future
			b.WriteString(fmt.Sprintf("  %s %-12s pending\n", statusPending.String(), stage.String()))
		}
	}
	b.WriteString("\n")

	// Files (for the current stage path)
	currentStageInfo := item.GetStageInfo(item.Current)
	if currentStageInfo != nil && currentStageInfo.Path != "" {
		files := listMediaFiles(currentStageInfo.Path)
		if len(files) > 0 {
			b.WriteString(sectionHeaderStyle.Render("FILES"))
			b.WriteString("\n")
			for _, file := range files {
				b.WriteString(fmt.Sprintf("  • %s\n", file))
			}
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString(helpStyle.Render("[Esc] Back  [r] Refresh  [q] Quit"))

	return b.String()
}

// listMediaFiles returns a list of media files in a directory
func listMediaFiles(dir string) []string {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories except the state directories
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include video files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".mkv" || ext == ".mp4" || ext == ".avi" || ext == ".m4v" {
			// Make path relative to dir
			relPath, _ := filepath.Rel(dir, path)

			// Get file size
			sizeStr := ""
			if info.Size() > 0 {
				sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
				if sizeGB >= 1 {
					sizeStr = fmt.Sprintf(" (%.1f GB)", sizeGB)
				} else {
					sizeMB := float64(info.Size()) / (1024 * 1024)
					sizeStr = fmt.Sprintf(" (%.0f MB)", sizeMB)
				}
			}

			files = append(files, relPath+sizeStr)
		}
		return nil
	})

	if err != nil {
		return nil
	}

	// Limit to 10 files
	if len(files) > 10 {
		files = append(files[:10], fmt.Sprintf("... and %d more", len(files)-10))
	}

	return files
}
