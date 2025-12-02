package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	colorPrimary   = lipgloss.Color("39")  // Blue
	colorSecondary = lipgloss.Color("241") // Gray
	colorSuccess   = lipgloss.Color("42")  // Green
	colorWarning   = lipgloss.Color("214") // Orange
	colorError     = lipgloss.Color("196") // Red
	colorMuted     = lipgloss.Color("240") // Dark gray
)

// Styles
var (
	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			MarginBottom(1)

	// Status styles
	statusCompleted = lipgloss.NewStyle().
			Foreground(colorSuccess).
			SetString("✓")

	statusInProgress = lipgloss.NewStyle().
				Foreground(colorWarning).
				SetString("●")

	statusFailed = lipgloss.NewStyle().
			Foreground(colorError).
			SetString("✗")

	statusPending = lipgloss.NewStyle().
			Foreground(colorMuted).
			SetString("○")

	// List item styles
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	mutedItemStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Bar chart styles
	barFull = lipgloss.NewStyle().
		Foreground(colorSuccess).
		SetString("█")

	barEmpty = lipgloss.NewStyle().
			Foreground(colorMuted).
			SetString("░")

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary).
			Padding(1, 2)

	// Section header
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				MarginTop(1).
				MarginBottom(1)
)

// StatusIcon returns the appropriate icon for a status
func StatusIcon(status string) string {
	switch status {
	case "completed":
		return statusCompleted.String()
	case "in_progress":
		return statusInProgress.String()
	case "failed":
		return statusFailed.String()
	default:
		return statusPending.String()
	}
}

// RenderBar creates a progress bar
func RenderBar(filled, total, width int) string {
	if total == 0 {
		return ""
	}
	filledWidth := (filled * width) / total
	if filledWidth > width {
		filledWidth = width
	}

	bar := ""
	for i := 0; i < filledWidth; i++ {
		bar += barFull.String()
	}
	for i := filledWidth; i < width; i++ {
		bar += barEmpty.String()
	}
	return bar
}
