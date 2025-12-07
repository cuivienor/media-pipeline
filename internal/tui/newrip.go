package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// NewRipForm holds the form state for creating a new rip
type NewRipForm struct {
	Type     string // "movie" or "tv"
	Name     string
	Season   string // only for TV
	Disc     string // only for TV
	DiscPath string

	focusIndex int    // which field is focused
	err        string // validation error
}

// fields returns the list of field names in order
func (f *NewRipForm) fields() []string {
	if f.Type == "tv" {
		return []string{"type", "name", "season", "disc"}
	}
	return []string{"type", "name"}
}

// Validate returns an error message if the form is invalid
func (f *NewRipForm) Validate() string {
	if f.Name == "" {
		return "Name is required"
	}
	if f.Type == "tv" {
		if f.Season == "" {
			return "Season is required for TV shows"
		}
		if _, err := strconv.Atoi(f.Season); err != nil {
			return "Season must be a number"
		}
		if f.Disc == "" {
			return "Disc is required for TV shows"
		}
		if _, err := strconv.Atoi(f.Disc); err != nil {
			return "Disc must be a number"
		}
	}
	return ""
}

// renderNewRipForm renders the new rip form view
func (a *App) renderNewRipForm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Rip"))
	b.WriteString("\n\n")

	form := a.newRipForm
	fields := form.fields()

	for i, field := range fields {
		prefix := "  "
		if i == form.focusIndex {
			prefix = "> "
		}

		switch field {
		case "type":
			typeStr := "[movie]  tv"
			if form.Type == "tv" {
				typeStr = " movie  [tv]"
			}
			b.WriteString(fmt.Sprintf("%sType: %s\n", prefix, typeStr))
		case "name":
			b.WriteString(fmt.Sprintf("%sName: %s\n", prefix, form.Name))
		case "season":
			b.WriteString(fmt.Sprintf("%sSeason: %s\n", prefix, form.Season))
		case "disc":
			b.WriteString(fmt.Sprintf("%sDisc: %s\n", prefix, form.Disc))
		}
	}

	b.WriteString("\n")

	if form.err != "" {
		b.WriteString(errorStyle.Render(form.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("[Enter] Submit  [Tab] Next field  [Esc] Cancel"))

	return b.String()
}

// handleNewRipKey handles key presses in the new rip form
func (a *App) handleNewRipKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := a.newRipForm
	fields := form.fields()

	switch msg.String() {
	case "tab", "down":
		form.focusIndex = (form.focusIndex + 1) % len(fields)
		return a, nil

	case "shift+tab", "up":
		form.focusIndex--
		if form.focusIndex < 0 {
			form.focusIndex = len(fields) - 1
		}
		return a, nil

	case "left", "right":
		// Toggle type
		if fields[form.focusIndex] == "type" {
			if form.Type == "movie" {
				form.Type = "tv"
			} else {
				form.Type = "movie"
			}
		}
		return a, nil

	case "enter":
		// Validate and submit
		if err := form.Validate(); err != "" {
			form.err = err
			return a, nil
		}
		return a, a.dispatchRip()

	case "backspace":
		// Delete character from current field
		field := fields[form.focusIndex]
		switch field {
		case "name":
			if len(form.Name) > 0 {
				form.Name = form.Name[:len(form.Name)-1]
			}
		case "season":
			if len(form.Season) > 0 {
				form.Season = form.Season[:len(form.Season)-1]
			}
		case "disc":
			if len(form.Disc) > 0 {
				form.Disc = form.Disc[:len(form.Disc)-1]
			}
		}
		return a, nil

	default:
		// Type character into current field
		if len(msg.String()) == 1 {
			field := fields[form.focusIndex]
			char := msg.String()
			switch field {
			case "name":
				form.Name += char
			case "season":
				if char >= "0" && char <= "9" {
					form.Season += char
				}
			case "disc":
				if char >= "0" && char <= "9" {
					form.Disc += char
				}
			}
		}
		return a, nil
	}
}

// ripCompleteMsg is sent when a rip dispatch completes
type ripCompleteMsg struct {
	err error
}

// dispatchRip dispatches a rip command based on config
func (a *App) dispatchRip() tea.Cmd {
	return func() tea.Msg {
		form := a.newRipForm

		// Create job in database first
		ctx := context.Background()

		// Build media item
		var season *int
		if form.Type == "tv" {
			s, _ := strconv.Atoi(form.Season)
			season = &s
		}

		safeName := strings.ReplaceAll(form.Name, " ", "_")
		item := &model.MediaItem{
			Type:     model.MediaType(form.Type),
			Name:     form.Name,
			SafeName: safeName,
			Season:   season,
		}

		// Check if item exists
		existing, err := a.repo.GetMediaItemBySafeName(ctx, safeName, season)
		if err != nil {
			return ripCompleteMsg{err: fmt.Errorf("failed to check for existing item: %w", err)}
		}
		if existing != nil {
			item = existing
		} else {
			if err := a.repo.CreateMediaItem(ctx, item); err != nil {
				return ripCompleteMsg{err: err}
			}
		}

		// Create pending job
		var disc *int
		if form.Type == "tv" {
			d, _ := strconv.Atoi(form.Disc)
			disc = &d
		}

		job := &model.Job{
			MediaItemID: item.ID,
			Stage:       model.StageRip,
			Status:      model.JobStatusPending,
			Disc:        disc,
		}
		if err := a.repo.CreateJob(ctx, job); err != nil {
			return ripCompleteMsg{err: err}
		}

		// Build command
		args := []string{
			"-job-id", fmt.Sprintf("%d", job.ID),
			"-db", a.config.DatabasePath(),
		}

		target := a.config.DispatchTarget("rip")
		if target == "" {
			// Local execution
			cmd := exec.Command("ripper", args...)
			if err := cmd.Start(); err != nil {
				return ripCompleteMsg{err: err}
			}
		} else {
			// SSH dispatch
			sshArgs := append([]string{target, "ripper"}, args...)
			cmd := exec.Command("ssh", sshArgs...)
			if err := cmd.Start(); err != nil {
				return ripCompleteMsg{err: err}
			}
		}

		return ripCompleteMsg{err: nil}
	}
}
