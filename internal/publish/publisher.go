package publish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/logging"
	"github.com/cuivienor/media-pipeline/internal/model"
)

// FilebotRunner executes FileBot commands
type FilebotRunner interface {
	Run(args []string) (string, error)
}

// defaultFilebotRunner runs the real FileBot command
type defaultFilebotRunner struct{}

func (r *defaultFilebotRunner) Run(args []string) (string, error) {
	cmd := exec.Command("filebot", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Jellyfin-supported extras types
var extrasTypes = []string{
	"behind the scenes",
	"deleted scenes",
	"featurettes",
	"interviews",
	"scenes",
	"shorts",
	"trailers",
	"other",
}

// PublishOptions configures the publisher
type PublishOptions struct {
	LibraryMovies string // Destination for movies
	LibraryTV     string // Destination for TV shows
}

// ExtraDir represents an extras directory found in the input
type ExtraDir struct {
	Type  string   // e.g., "featurettes"
	Path  string   // Full path to extras directory
	Files []string // MKV files in the directory
}

// Publisher handles copying media to the library using FileBot
type Publisher struct {
	repo    db.Repository
	logger  *logging.Logger
	opts    PublishOptions
	filebot FilebotRunner // Injectable for testing
}

// NewPublisher creates a new Publisher
func NewPublisher(repo db.Repository, logger *logging.Logger, opts PublishOptions) *Publisher {
	return &Publisher{
		repo:    repo,
		logger:  logger,
		opts:    opts,
		filebot: &defaultFilebotRunner{},
	}
}

// SetFilebotRunner allows injecting a custom FileBot runner (for testing)
func (p *Publisher) SetFilebotRunner(runner FilebotRunner) {
	p.filebot = runner
}

// buildFilebotArgs constructs FileBot CLI arguments
func (p *Publisher) buildFilebotArgs(inputDir string, mediaType string, dbID int) []string {
	var db, output, format string

	if mediaType == "movie" {
		db = "TheMovieDB"
		output = p.opts.LibraryMovies
		format = "{n} ({y})/{n} ({y})"
	} else {
		db = "TheTVDB"
		output = p.opts.LibraryTV
		format = "{n}/Season {s.pad(2)}/{n} - {s00e00} - {t}"
	}

	return []string{
		"-rename", inputDir,
		"--db", db,
		"--q", fmt.Sprintf("%d", dbID),
		"--output", output,
		"--format", format,
		"-non-strict",
		"--action", "copy",
	}
}

// findExtras scans for Jellyfin-compatible extras in _extras/<type>/
func (p *Publisher) findExtras(inputDir string) []ExtraDir {
	var extras []ExtraDir

	extrasBase := filepath.Join(inputDir, "_extras")

	for _, extType := range extrasTypes {
		extPath := filepath.Join(extrasBase, extType)
		if info, err := os.Stat(extPath); err == nil && info.IsDir() {
			files, _ := filepath.Glob(filepath.Join(extPath, "*.mkv"))
			if len(files) > 0 {
				extras = append(extras, ExtraDir{
					Type:  extType,
					Path:  extPath,
					Files: files,
				})
			}
		}
	}

	return extras
}

// runFilebot executes FileBot and returns the output
func (p *Publisher) runFilebot(args []string) (string, error) {
	return p.filebot.Run(args)
}

// parseFilebotDestination extracts the library destination from FileBot output
func parseFilebotDestination(output string) string {
	// Pattern: [COPY] from [...] to [/path/to/dest/file.mkv]
	re := regexp.MustCompile(`\[COPY\] from .* to \[([^\]]+)\]`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		// Return parent directory of the copied file
		return filepath.Dir(matches[1])
	}
	return ""
}

// copyExtras copies extras directories to the library destination
func (p *Publisher) copyExtras(extras []ExtraDir, libraryDest string) (int, error) {
	copied := 0

	for _, extra := range extras {
		destDir := filepath.Join(libraryDest, extra.Type)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return copied, fmt.Errorf("failed to create extras dir %s: %w", destDir, err)
		}

		for _, srcFile := range extra.Files {
			dstFile := filepath.Join(destDir, filepath.Base(srcFile))
			if err := copyFile(srcFile, dstFile); err != nil {
				return copied, fmt.Errorf("failed to copy %s: %w", srcFile, err)
			}
			copied++
		}
	}

	return copied, nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = dstF.ReadFrom(srcF)
	return err
}

// verifyFiles checks that files exist in the destination directory
func (p *Publisher) verifyFiles(destDir string) error {
	files, err := filepath.Glob(filepath.Join(destDir, "*.mkv"))
	if err != nil {
		return fmt.Errorf("failed to glob destination: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no MKV files found in destination %s", destDir)
	}

	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return fmt.Errorf("file not accessible: %s: %w", f, err)
		}
		if info.Size() == 0 {
			return fmt.Errorf("file is empty: %s", f)
		}
	}

	return nil
}

// PublishResult contains the results of a publish operation
type PublishResult struct {
	LibraryPath   string // Where main content was copied
	MainFiles     int    // Number of main content files copied
	ExtrasFiles   int    // Number of extras files copied
	FilebotOutput string // Raw FileBot output
}

// Publish copies media to the library using FileBot
func (p *Publisher) Publish(ctx context.Context, item *model.MediaItem, inputDir string) (*PublishResult, error) {
	dbID := item.DatabaseID()
	if dbID == 0 {
		return nil, fmt.Errorf("media item requires a database ID (tmdb_id for movies, tvdb_id for TV)")
	}

	mediaType := string(item.Type)

	// Transcode outputs to _main/ subdirectory - use that for FileBot
	mainDir := filepath.Join(inputDir, "_main")

	// Run FileBot on main content
	args := p.buildFilebotArgs(mainDir, mediaType, dbID)
	if p.logger != nil {
		p.logger.Info("Running FileBot: filebot %s", strings.Join(args, " "))
	}

	output, err := p.runFilebot(args)
	if err != nil {
		return nil, fmt.Errorf("filebot failed: %w\nOutput: %s", err, output)
	}

	// Parse destination from output
	libraryDest := parseFilebotDestination(output)
	if libraryDest == "" {
		return nil, fmt.Errorf("failed to determine library destination from FileBot output")
	}

	// Count main files copied
	mainCount := strings.Count(output, "[COPY]")

	// Find and copy extras
	extras := p.findExtras(inputDir)
	extrasCount := 0
	if len(extras) > 0 {
		if p.logger != nil {
			p.logger.Info("Found %d extras directories", len(extras))
		}
		extrasCount, err = p.copyExtras(extras, libraryDest)
		if err != nil {
			return nil, fmt.Errorf("failed to copy extras: %w", err)
		}
	}

	// Verify files exist
	if err := p.verifyFiles(libraryDest); err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	return &PublishResult{
		LibraryPath:   libraryDest,
		MainFiles:     mainCount,
		ExtrasFiles:   extrasCount,
		FilebotOutput: output,
	}, nil
}
