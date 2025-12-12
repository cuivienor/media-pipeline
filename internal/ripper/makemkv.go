package ripper

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DefaultMakeMKVRunner executes makemkvcon commands
type DefaultMakeMKVRunner struct {
	makemkvconPath string
	// execCommand allows injection of command execution for testing
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewMakeMKVRunner creates a new MakeMKV runner
// If makemkvconPath is empty, uses "makemkvcon" from PATH
func NewMakeMKVRunner(makemkvconPath string) *DefaultMakeMKVRunner {
	if makemkvconPath == "" {
		makemkvconPath = "makemkvcon"
	}
	return &DefaultMakeMKVRunner{
		makemkvconPath: makemkvconPath,
		execCommand:    exec.CommandContext,
	}
}

// GetDiscInfo retrieves information about a disc
func (r *DefaultMakeMKVRunner) GetDiscInfo(ctx context.Context, discPath string) (*DiscInfo, error) {
	args := r.buildInfoArgs(discPath)

	cmd := r.execCommand(ctx, r.makemkvconPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start makemkvcon: %w", err)
	}

	parser := NewMakeMKVParser()
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		parser.ParseLine(scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("makemkvcon failed: %w", err)
	}

	return parser.GetDiscInfo(), nil
}

// RipTitles rips specified titles from a disc
// If titleIndices is nil or empty, rips all titles
func (r *DefaultMakeMKVRunner) RipTitles(ctx context.Context, discPath, outputDir string, titleIndices []int, onLine LineCallback, onProgress ProgressCallback) error {
	// If specific titles requested, rip each one
	// Otherwise, use "all" to rip everything
	if len(titleIndices) > 0 {
		for _, idx := range titleIndices {
			if err := r.ripTitle(ctx, discPath, outputDir, idx, onLine, onProgress); err != nil {
				return err
			}
		}
		return nil
	}

	return r.ripAllTitles(ctx, discPath, outputDir, onLine, onProgress)
}

// ripTitle rips a single title
func (r *DefaultMakeMKVRunner) ripTitle(ctx context.Context, discPath, outputDir string, titleIdx int, onLine LineCallback, onProgress ProgressCallback) error {
	args := []string{"-r", "--noscan", "mkv", discPath, strconv.Itoa(titleIdx), outputDir}

	cmd := r.execCommand(ctx, r.makemkvconPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start makemkvcon: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if onLine != nil {
			onLine(line)
		}
		r.handleProgressLine(line, onProgress)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("makemkvcon failed: %w", err)
	}

	return nil
}

// ripAllTitles rips all titles from a disc
func (r *DefaultMakeMKVRunner) ripAllTitles(ctx context.Context, discPath, outputDir string, onLine LineCallback, onProgress ProgressCallback) error {
	args := r.buildMkvArgs(discPath, outputDir, nil)

	cmd := r.execCommand(ctx, r.makemkvconPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start makemkvcon: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if onLine != nil {
			onLine(line)
		}
		r.handleProgressLine(line, onProgress)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("makemkvcon failed: %w", err)
	}

	return nil
}

// handleProgressLine parses and dispatches progress updates
func (r *DefaultMakeMKVRunner) handleProgressLine(line string, callback ProgressCallback) {
	if callback == nil {
		return
	}

	current, _, max, ok := ParseProgress(line)
	if !ok {
		return
	}

	percent := 0.0
	if max > 0 {
		percent = float64(current) / float64(max) * 100
	}

	callback(Progress{
		Percent: percent,
	})
}

// buildInfoArgs builds command line arguments for info command
func (r *DefaultMakeMKVRunner) buildInfoArgs(discPath string) []string {
	return []string{"-r", "--noscan", "info", discPath}
}

// buildMkvArgs builds command line arguments for mkv command
func (r *DefaultMakeMKVRunner) buildMkvArgs(discPath, outputDir string, titleIndices []int) []string {
	args := []string{"-r", "--noscan", "mkv", discPath}

	if len(titleIndices) == 0 {
		args = append(args, "all")
	} else {
		// Join title indices with comma for single command
		titles := make([]string, len(titleIndices))
		for i, idx := range titleIndices {
			titles[i] = strconv.Itoa(idx)
		}
		args = append(args, strings.Join(titles, ","))
	}

	args = append(args, outputDir)
	return args
}

// Ensure DefaultMakeMKVRunner implements MakeMKVRunner interface
var _ MakeMKVRunner = (*DefaultMakeMKVRunner)(nil)
