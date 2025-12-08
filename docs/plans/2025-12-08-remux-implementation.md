# Remux Stage Implementation

**Date:** 2025-12-08
**Status:** Ready for implementation
**Branch:** `feat/remux-implementation`
**Depends on:** TUI Database Integration (completed)

## Goal

Implement the remux stage that:
1. Filters MKV files to keep only configured languages (audio + subtitles)
2. Consolidates multi-disc TV shows into properly named episodes
3. Uses mkvmerge (shelling out) for the actual remuxing
4. Integrates with the existing TUI dispatch system

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Implementation | Native Go | No bash wrapper, direct Go implementation |
| Track filtering | mkvmerge | Shell out to mkvmerge for track selection |
| Language config | config.yaml | `remux.languages` section in pipeline config |
| Track selection | Keep all matching | Keep all tracks matching configured languages (no manual selection) |
| Input discovery | From organize job | Look up previous stage's job output_dir |
| Output path | Explicit on job | OutputDir stored on job when created |

## Config File Changes

Location: `$MEDIA_BASE/pipeline/config.yaml`

```yaml
staging_base: /tmp/test-media/staging
library_base: /tmp/test-media/library

dispatch:
  rip: ""
  remux: ""
  transcode: ""
  publish: ""

# NEW: Remux configuration
remux:
  languages:
    - eng  # English
    - bul  # Bulgarian
```

---

## Task 1: Add Remux Config to Config Package

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1.1: Write failing test for remux languages config**

Add to `internal/config/config_test.go`:

```go
func TestConfig_RemuxLanguages(t *testing.T) {
	tmpDir := t.TempDir()
	pipelineDir := filepath.Join(tmpDir, "pipeline")
	os.MkdirAll(pipelineDir, 0755)

	configContent := `
staging_base: /mnt/media/staging
library_base: /mnt/media/library
remux:
  languages:
    - eng
    - bul
`
	os.WriteFile(filepath.Join(pipelineDir, "config.yaml"), []byte(configContent), 0644)

	t.Setenv("MEDIA_BASE", tmpDir)

	cfg, err := LoadFromMediaBase()
	if err != nil {
		t.Fatalf("LoadFromMediaBase() error = %v", err)
	}

	langs := cfg.RemuxLanguages()
	if len(langs) != 2 {
		t.Errorf("RemuxLanguages() = %v, want 2 languages", langs)
	}
	if langs[0] != "eng" || langs[1] != "bul" {
		t.Errorf("RemuxLanguages() = %v, want [eng, bul]", langs)
	}
}

func TestConfig_RemuxLanguages_Default(t *testing.T) {
	tmpDir := t.TempDir()
	pipelineDir := filepath.Join(tmpDir, "pipeline")
	os.MkdirAll(pipelineDir, 0755)

	// Config without remux section
	configContent := `
staging_base: /mnt/media/staging
library_base: /mnt/media/library
`
	os.WriteFile(filepath.Join(pipelineDir, "config.yaml"), []byte(configContent), 0644)

	t.Setenv("MEDIA_BASE", tmpDir)

	cfg, err := LoadFromMediaBase()
	if err != nil {
		t.Fatalf("LoadFromMediaBase() error = %v", err)
	}

	langs := cfg.RemuxLanguages()
	if len(langs) != 1 || langs[0] != "eng" {
		t.Errorf("RemuxLanguages() default = %v, want [eng]", langs)
	}
}
```

**Step 1.2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestConfig_Remux -v
```

**Step 1.3: Update config.go with RemuxConfig struct and methods**

In `internal/config/config.go`, add:

```go
// RemuxConfig holds remux-specific configuration
type RemuxConfig struct {
	Languages []string `yaml:"languages"`
}

// Config holds application configuration (update struct)
type Config struct {
	StagingBase string            `yaml:"staging_base"`
	LibraryBase string            `yaml:"library_base"`
	Dispatch    map[string]string `yaml:"dispatch"`
	Remux       RemuxConfig       `yaml:"remux"`

	mediaBase string
}

// RemuxLanguages returns the list of languages to keep during remux
// Defaults to ["eng"] if not configured
func (c *Config) RemuxLanguages() []string {
	if len(c.Remux.Languages) == 0 {
		return []string{"eng"}
	}
	return c.Remux.Languages
}
```

**Step 1.4: Run tests**

```bash
go test ./internal/config/... -v
```

**Step 1.5: Commit**

```bash
git add internal/config/
git commit -m "config: add remux.languages configuration"
```

---

## Task 2: Create internal/remux Package with Track Analysis

**Files:**
- Create: `internal/remux/remux.go`
- Create: `internal/remux/remux_test.go`
- Create: `internal/remux/mkvmerge.go`
- Create: `internal/remux/mkvmerge_test.go`

**Step 2.1: Write failing test for track analysis**

Create `internal/remux/mkvmerge_test.go`:

```go
package remux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTrackInfo(t *testing.T) {
	// This JSON represents mkvmerge -J output
	jsonOutput := `{
		"container": {"type": "Matroska"},
		"tracks": [
			{"id": 0, "type": "video", "codec": "HEVC", "properties": {}},
			{"id": 1, "type": "audio", "codec": "AAC", "properties": {"language": "eng", "track_name": "English"}},
			{"id": 2, "type": "audio", "codec": "AAC", "properties": {"language": "bul", "track_name": "Bulgarian"}},
			{"id": 3, "type": "audio", "codec": "AAC", "properties": {"language": "fra", "track_name": "French"}},
			{"id": 4, "type": "subtitles", "codec": "SubRip/SRT", "properties": {"language": "eng", "track_name": "English"}},
			{"id": 5, "type": "subtitles", "codec": "SubRip/SRT", "properties": {"language": "eng", "track_name": "English (Forced)", "forced_track": true}},
			{"id": 6, "type": "subtitles", "codec": "SubRip/SRT", "properties": {"language": "bul", "track_name": "Bulgarian"}}
		]
	}`

	info, err := ParseTrackInfo([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("ParseTrackInfo() error = %v", err)
	}

	if len(info.Video) != 1 {
		t.Errorf("Video tracks = %d, want 1", len(info.Video))
	}
	if len(info.Audio) != 3 {
		t.Errorf("Audio tracks = %d, want 3", len(info.Audio))
	}
	if len(info.Subtitles) != 3 {
		t.Errorf("Subtitle tracks = %d, want 3", len(info.Subtitles))
	}

	// Check audio track details
	if info.Audio[0].Language != "eng" {
		t.Errorf("Audio[0].Language = %q, want eng", info.Audio[0].Language)
	}
	if info.Audio[1].Language != "bul" {
		t.Errorf("Audio[1].Language = %q, want bul", info.Audio[1].Language)
	}

	// Check forced subtitle flag
	if !info.Subtitles[1].Forced {
		t.Error("Subtitles[1].Forced = false, want true")
	}
}

func TestFilterTracks(t *testing.T) {
	info := &TrackInfo{
		Video: []Track{{ID: 0, Type: "video"}},
		Audio: []Track{
			{ID: 1, Type: "audio", Language: "eng", Title: "English"},
			{ID: 2, Type: "audio", Language: "bul", Title: "Bulgarian"},
			{ID: 3, Type: "audio", Language: "fra", Title: "French"},
		},
		Subtitles: []Track{
			{ID: 4, Type: "subtitles", Language: "eng", Title: "English"},
			{ID: 5, Type: "subtitles", Language: "eng", Title: "English (Forced)", Forced: true},
			{ID: 6, Type: "subtitles", Language: "bul", Title: "Bulgarian"},
			{ID: 7, Type: "subtitles", Language: "spa", Title: "Spanish"},
		},
	}

	languages := []string{"eng", "bul"}
	filtered := FilterTracks(info, languages)

	// All video tracks should be kept
	if len(filtered.Video) != 1 {
		t.Errorf("Filtered video = %d, want 1", len(filtered.Video))
	}

	// Only eng and bul audio should remain
	if len(filtered.Audio) != 2 {
		t.Errorf("Filtered audio = %d, want 2", len(filtered.Audio))
	}
	for _, a := range filtered.Audio {
		if a.Language != "eng" && a.Language != "bul" {
			t.Errorf("Unexpected audio language: %s", a.Language)
		}
	}

	// Only eng and bul subtitles should remain (3 tracks: 2 eng, 1 bul)
	if len(filtered.Subtitles) != 3 {
		t.Errorf("Filtered subtitles = %d, want 3", len(filtered.Subtitles))
	}
	for _, s := range filtered.Subtitles {
		if s.Language != "eng" && s.Language != "bul" {
			t.Errorf("Unexpected subtitle language: %s", s.Language)
		}
	}
}
```

**Step 2.2: Run test to verify it fails**

```bash
go test ./internal/remux/... -v
```

**Step 2.3: Implement mkvmerge.go**

Create `internal/remux/mkvmerge.go`:

```go
package remux

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Track represents a single track in an MKV file
type Track struct {
	ID       int
	Type     string // "video", "audio", "subtitles"
	Codec    string
	Language string
	Title    string
	Forced   bool
	Default  bool
}

// TrackInfo holds parsed track information from mkvmerge -J
type TrackInfo struct {
	Video     []Track
	Audio     []Track
	Subtitles []Track
}

// mkvmergeJSON represents the JSON output from mkvmerge -J
type mkvmergeJSON struct {
	Container struct {
		Type string `json:"type"`
	} `json:"container"`
	Tracks []struct {
		ID         int    `json:"id"`
		Type       string `json:"type"`
		Codec      string `json:"codec"`
		Properties struct {
			Language    string `json:"language"`
			TrackName   string `json:"track_name"`
			ForcedTrack bool   `json:"forced_track"`
			DefaultTrack bool  `json:"default_track"`
		} `json:"properties"`
	} `json:"tracks"`
}

// GetTrackInfo runs mkvmerge -J on a file and returns parsed track info
func GetTrackInfo(path string) (*TrackInfo, error) {
	cmd := exec.Command("mkvmerge", "-J", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mkvmerge -J failed: %w", err)
	}

	return ParseTrackInfo(output)
}

// ParseTrackInfo parses mkvmerge -J JSON output into TrackInfo
func ParseTrackInfo(jsonData []byte) (*TrackInfo, error) {
	var data mkvmergeJSON
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse mkvmerge output: %w", err)
	}

	info := &TrackInfo{}
	for _, t := range data.Tracks {
		track := Track{
			ID:       t.ID,
			Type:     t.Type,
			Codec:    t.Codec,
			Language: t.Properties.Language,
			Title:    t.Properties.TrackName,
			Forced:   t.Properties.ForcedTrack,
			Default:  t.Properties.DefaultTrack,
		}

		switch t.Type {
		case "video":
			info.Video = append(info.Video, track)
		case "audio":
			info.Audio = append(info.Audio, track)
		case "subtitles":
			info.Subtitles = append(info.Subtitles, track)
		}
	}

	return info, nil
}

// FilterTracks returns a new TrackInfo containing only tracks with matching languages
// Video tracks are always kept. Audio and subtitle tracks are filtered by language.
func FilterTracks(info *TrackInfo, languages []string) *TrackInfo {
	langSet := make(map[string]bool)
	for _, lang := range languages {
		langSet[strings.ToLower(lang)] = true
	}

	filtered := &TrackInfo{
		Video: info.Video, // Keep all video tracks
	}

	for _, track := range info.Audio {
		if langSet[strings.ToLower(track.Language)] {
			filtered.Audio = append(filtered.Audio, track)
		}
	}

	for _, track := range info.Subtitles {
		if langSet[strings.ToLower(track.Language)] {
			filtered.Subtitles = append(filtered.Subtitles, track)
		}
	}

	return filtered
}

// BuildMkvmergeArgs builds mkvmerge command arguments for remuxing with filtered tracks
func BuildMkvmergeArgs(inputPath, outputPath string, tracks *TrackInfo) []string {
	args := []string{"-o", outputPath}

	// Build track selection arguments
	// Video: always keep all
	if len(tracks.Video) > 0 {
		var videoIDs []string
		for _, v := range tracks.Video {
			videoIDs = append(videoIDs, fmt.Sprintf("%d", v.ID))
		}
		args = append(args, "--video-tracks", strings.Join(videoIDs, ","))
	} else {
		args = append(args, "--no-video")
	}

	// Audio: keep filtered tracks
	if len(tracks.Audio) > 0 {
		var audioIDs []string
		for _, a := range tracks.Audio {
			audioIDs = append(audioIDs, fmt.Sprintf("%d", a.ID))
		}
		args = append(args, "--audio-tracks", strings.Join(audioIDs, ","))
	} else {
		args = append(args, "--no-audio")
	}

	// Subtitles: keep filtered tracks
	if len(tracks.Subtitles) > 0 {
		var subIDs []string
		for _, s := range tracks.Subtitles {
			subIDs = append(subIDs, fmt.Sprintf("%d", s.ID))
		}
		args = append(args, "--subtitle-tracks", strings.Join(subIDs, ","))
	} else {
		args = append(args, "--no-subtitles")
	}

	args = append(args, inputPath)
	return args
}

// RunMkvmerge executes mkvmerge with the given arguments
func RunMkvmerge(args []string) error {
	cmd := exec.Command("mkvmerge", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkvmerge failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
```

**Step 2.4: Run tests**

```bash
go test ./internal/remux/... -v
```

**Step 2.5: Commit**

```bash
git add internal/remux/
git commit -m "remux: add mkvmerge track parsing and filtering"
```

---

## Task 3: Create Remuxer with File Processing Logic

**Files:**
- Create: `internal/remux/remuxer.go`
- Create: `internal/remux/remuxer_test.go`

**Step 3.1: Write failing test for Remuxer**

Create `internal/remux/remuxer_test.go`:

```go
package remux

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRemuxer_RemuxFile(t *testing.T) {
	// Skip if mkvmerge not available
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed, skipping integration test")
	}

	// Create a test MKV file with multiple tracks
	// This requires ffmpeg to be installed
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mkv")
	outputPath := filepath.Join(tmpDir, "output.mkv")

	// Generate test MKV with multiple tracks (using ffmpeg)
	if err := generateTestMKV(inputPath); err != nil {
		t.Skipf("Could not generate test MKV: %v", err)
	}

	remuxer := NewRemuxer([]string{"eng", "bul"})

	result, err := remuxer.RemuxFile(context.Background(), inputPath, outputPath)
	if err != nil {
		t.Fatalf("RemuxFile() error = %v", err)
	}

	// Verify output exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file not created: %v", err)
	}

	// Verify track counts
	if result.InputTracks.Video == 0 {
		t.Error("Expected video tracks in input")
	}
	if result.OutputTracks.Audio == 0 {
		t.Error("Expected audio tracks in output")
	}

	// Verify only eng/bul tracks remain
	outputInfo, err := GetTrackInfo(outputPath)
	if err != nil {
		t.Fatalf("GetTrackInfo(output) error = %v", err)
	}

	for _, audio := range outputInfo.Audio {
		if audio.Language != "eng" && audio.Language != "bul" {
			t.Errorf("Unexpected audio language in output: %s", audio.Language)
		}
	}
}

// generateTestMKV creates a test MKV file with multiple language tracks
// Requires ffmpeg to be installed
func generateTestMKV(outputPath string) error {
	// Implementation would use ffmpeg similar to mock-makemkv
	// For now, this is a placeholder that tests can skip if not available
	return nil
}
```

**Step 3.2: Run test to verify it fails**

```bash
go test ./internal/remux/... -run TestRemuxer -v
```

**Step 3.3: Implement remuxer.go**

Create `internal/remux/remuxer.go`:

```go
package remux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Remuxer handles MKV file remuxing with track filtering
type Remuxer struct {
	languages []string
}

// NewRemuxer creates a new Remuxer with the specified language filters
func NewRemuxer(languages []string) *Remuxer {
	return &Remuxer{languages: languages}
}

// RemuxResult contains statistics about a remux operation
type RemuxResult struct {
	InputPath   string
	OutputPath  string
	InputTracks TrackCounts
	OutputTracks TrackCounts
	TracksRemoved int
}

// TrackCounts holds counts by track type
type TrackCounts struct {
	Video     int
	Audio     int
	Subtitles int
}

// RemuxFile remuxes a single MKV file, filtering tracks by language
func (r *Remuxer) RemuxFile(ctx context.Context, inputPath, outputPath string) (*RemuxResult, error) {
	// Get track info from input
	inputInfo, err := GetTrackInfo(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze %s: %w", inputPath, err)
	}

	// Filter tracks
	filteredInfo := FilterTracks(inputInfo, r.languages)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build and run mkvmerge
	args := BuildMkvmergeArgs(inputPath, outputPath, filteredInfo)
	if err := RunMkvmerge(args); err != nil {
		return nil, err
	}

	inputCounts := TrackCounts{
		Video:     len(inputInfo.Video),
		Audio:     len(inputInfo.Audio),
		Subtitles: len(inputInfo.Subtitles),
	}
	outputCounts := TrackCounts{
		Video:     len(filteredInfo.Video),
		Audio:     len(filteredInfo.Audio),
		Subtitles: len(filteredInfo.Subtitles),
	}

	return &RemuxResult{
		InputPath:    inputPath,
		OutputPath:   outputPath,
		InputTracks:  inputCounts,
		OutputTracks: outputCounts,
		TracksRemoved: (inputCounts.Audio - outputCounts.Audio) +
		               (inputCounts.Subtitles - outputCounts.Subtitles),
	}, nil
}

// RemuxDirectory remuxes all MKV files in a directory
// For movies: remuxes _main/*.mkv files
// For TV: remuxes _episodes/*.mkv files, preserving episode names
func (r *Remuxer) RemuxDirectory(ctx context.Context, inputDir, outputDir string, isTV bool) ([]RemuxResult, error) {
	var results []RemuxResult

	// Determine input subdirectory
	var srcDir string
	if isTV {
		srcDir = filepath.Join(inputDir, "_episodes")
	} else {
		srcDir = filepath.Join(inputDir, "_main")
	}

	// Find MKV files
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".mkv") {
			continue
		}

		inputPath := filepath.Join(srcDir, entry.Name())

		// Determine output path
		var outputPath string
		if isTV {
			// TV: preserve episode naming in _episodes
			outputPath = filepath.Join(outputDir, "_episodes", entry.Name())
		} else {
			// Movie: single file in _main
			outputPath = filepath.Join(outputDir, "_main", entry.Name())
		}

		result, err := r.RemuxFile(ctx, inputPath, outputPath)
		if err != nil {
			return results, fmt.Errorf("failed to remux %s: %w", entry.Name(), err)
		}
		results = append(results, *result)
	}

	// Also copy extras if present
	extrasDir := filepath.Join(inputDir, "_extras")
	if _, err := os.Stat(extrasDir); err == nil {
		outputExtras := filepath.Join(outputDir, "_extras")
		if err := copyDirectory(extrasDir, outputExtras); err != nil {
			return results, fmt.Errorf("failed to copy extras: %w", err)
		}
	}

	return results, nil
}

// copyDirectory copies a directory recursively
func copyDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
```

**Step 3.4: Run tests**

```bash
go test ./internal/remux/... -v
```

**Step 3.5: Commit**

```bash
git add internal/remux/
git commit -m "remux: add Remuxer with file and directory processing"
```

---

## Task 4: Implement cmd/remux Binary

**Files:**
- Modify: `cmd/remux/main.go`

**Step 4.1: Replace stub with full implementation**

Replace `cmd/remux/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cuivienor/media-pipeline/internal/config"
	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
	"github.com/cuivienor/media-pipeline/internal/remux"
)

func main() {
	var jobID int64
	var dbPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: remux -job-id <id> -db <path>")
		os.Exit(1)
	}

	if err := run(jobID, dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(jobID int64, dbPath string) error {
	ctx := context.Background()

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)

	// Get job
	job, err := repo.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Get media item
	item, err := repo.GetMediaItem(ctx, job.MediaItemID)
	if err != nil {
		return fmt.Errorf("failed to get media item: %w", err)
	}

	// Load config for languages
	cfg, err := config.LoadFromMediaBase()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find input directory from organize job
	inputDir, err := findOrganizeOutput(ctx, repo, job)
	if err != nil {
		return fmt.Errorf("failed to find input: %w", err)
	}

	// Determine output directory
	outputDir := buildOutputPath(cfg, item, job)

	// Update job to in_progress with input/output paths
	job.Status = model.JobStatusInProgress
	job.InputDir = inputDir
	job.OutputDir = outputDir
	now := time.Now()
	job.StartedAt = &now
	if err := repo.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	fmt.Printf("Remux: %s\n", item.Name)
	fmt.Printf("  Input:  %s\n", inputDir)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Languages: %v\n", cfg.RemuxLanguages())

	// Create remuxer and process
	remuxer := remux.NewRemuxer(cfg.RemuxLanguages())
	isTV := item.Type == model.MediaTypeTV

	results, err := remuxer.RemuxDirectory(ctx, inputDir, outputDir, isTV)
	if err != nil {
		// Mark job as failed
		repo.UpdateJobStatus(ctx, jobID, model.JobStatusFailed, err.Error())
		return err
	}

	// Log results
	totalRemoved := 0
	for _, r := range results {
		fmt.Printf("  Processed: %s (%d tracks removed)\n",
			filepath.Base(r.InputPath), r.TracksRemoved)
		totalRemoved += r.TracksRemoved
	}
	fmt.Printf("  Total: %d files, %d tracks removed\n", len(results), totalRemoved)

	// Mark job as complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update media item stage
	if err := repo.UpdateMediaItemStage(ctx, item.ID, model.StageRemux, model.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update item stage: %w", err)
	}

	fmt.Println("Remux complete!")
	return nil
}

// findOrganizeOutput finds the output directory from the organize stage
func findOrganizeOutput(ctx context.Context, repo db.Repository, job *model.Job) (string, error) {
	// Look for completed organize job for this media item
	jobs, err := repo.ListJobsForMedia(ctx, job.MediaItemID)
	if err != nil {
		return "", err
	}

	// Find the most recent completed organize job
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if j.Stage == model.StageOrganize && j.Status == model.JobStatusCompleted {
			if j.OutputDir != "" {
				return j.OutputDir, nil
			}
		}
	}

	return "", fmt.Errorf("no completed organize job found for media item %d", job.MediaItemID)
}

// buildOutputPath constructs the output directory for remuxed files
func buildOutputPath(cfg *config.Config, item *model.MediaItem, job *model.Job) string {
	// Output goes to staging/2-remuxed/{movies,tv}/{safe_name}
	mediaTypeDir := "movies"
	if item.Type == model.MediaTypeTV {
		mediaTypeDir = "tv"
	}

	baseName := item.SafeName
	if item.Type == model.MediaTypeTV && job.SeasonID != nil {
		// Include season in path for TV
		// TODO: Look up season number from SeasonID
		baseName = fmt.Sprintf("%s/Season_XX", item.SafeName)
	}

	return filepath.Join(cfg.StagingBase, "2-remuxed", mediaTypeDir, baseName)
}
```

**Step 4.2: Run tests and build**

```bash
go build ./cmd/remux/
go test ./... -v
```

**Step 4.3: Commit**

```bash
git add cmd/remux/
git commit -m "remux: implement full remux command with track filtering"
```

---

## Task 5: Integration Test with Mock MKV Files

**Files:**
- Create: `internal/remux/integration_test.go`

**Step 5.1: Write integration test using mock-makemkv generated files**

Create `internal/remux/integration_test.go`:

```go
//go:build integration

package remux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRemuxer_Integration(t *testing.T) {
	// Skip if mkvmerge not installed
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed")
	}

	// Skip if ffmpeg not installed (needed for test file generation)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Generate test MKV with multiple tracks using ffmpeg
	// This mimics what mock-makemkv does
	inputPath := filepath.Join(tmpDir, "input", "_main", "movie.mkv")
	if err := generateMultiTrackMKV(inputPath); err != nil {
		t.Fatalf("Failed to generate test MKV: %v", err)
	}

	// Verify input has expected tracks
	inputInfo, err := GetTrackInfo(inputPath)
	if err != nil {
		t.Fatalf("GetTrackInfo(input) error: %v", err)
	}
	t.Logf("Input tracks: video=%d, audio=%d, subtitles=%d",
		len(inputInfo.Video), len(inputInfo.Audio), len(inputInfo.Subtitles))

	// Remux with eng+bul filter
	outputDir := filepath.Join(tmpDir, "output")
	remuxer := NewRemuxer([]string{"eng", "bul"})

	results, err := remuxer.RemuxDirectory(context.Background(),
		filepath.Join(tmpDir, "input"), outputDir, false)
	if err != nil {
		t.Fatalf("RemuxDirectory() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Verify output
	outputPath := filepath.Join(outputDir, "_main", "movie.mkv")
	outputInfo, err := GetTrackInfo(outputPath)
	if err != nil {
		t.Fatalf("GetTrackInfo(output) error: %v", err)
	}

	t.Logf("Output tracks: video=%d, audio=%d, subtitles=%d",
		len(outputInfo.Video), len(outputInfo.Audio), len(outputInfo.Subtitles))

	// Verify only eng/bul audio tracks remain
	for _, audio := range outputInfo.Audio {
		if audio.Language != "eng" && audio.Language != "bul" {
			t.Errorf("Unexpected audio language: %s", audio.Language)
		}
	}

	// Verify only eng/bul subtitle tracks remain
	for _, sub := range outputInfo.Subtitles {
		if sub.Language != "eng" && sub.Language != "bul" {
			t.Errorf("Unexpected subtitle language: %s", sub.Language)
		}
	}

	// Should have removed fra and spa tracks
	result := results[0]
	if result.TracksRemoved == 0 {
		t.Error("Expected some tracks to be removed")
	}
	t.Logf("Tracks removed: %d", result.TracksRemoved)
}

// generateMultiTrackMKV creates an MKV file with eng, bul, fra, spa audio and subtitle tracks
func generateMultiTrackMKV(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	durationSec := 3

	// Create subtitle files
	subs := []struct {
		lang  string
		title string
	}{
		{"eng", "English"},
		{"bul", "Bulgarian"},
		{"fra", "French"},
		{"spa", "Spanish"},
	}

	var subFiles []string
	for i, sub := range subs {
		srtPath := filepath.Join(dir, fmt.Sprintf(".sub_%d.srt", i))
		content := fmt.Sprintf("1\n00:00:00,000 --> 00:00:02,000\n%s\n", sub.title)
		if err := os.WriteFile(srtPath, []byte(content), 0644); err != nil {
			return err
		}
		subFiles = append(subFiles, srtPath)
	}
	defer func() {
		for _, f := range subFiles {
			os.Remove(f)
		}
	}()

	// Build ffmpeg command
	args := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=24", durationSec),
	}

	// Add 4 audio inputs
	for range subs {
		args = append(args, "-f", "lavfi", "-i",
			fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%d", durationSec))
	}

	// Add 4 subtitle inputs
	for _, f := range subFiles {
		args = append(args, "-i", f)
	}

	// Map all streams
	args = append(args, "-map", "0:v")
	for i := range subs {
		args = append(args, "-map", fmt.Sprintf("%d:a", i+1))
	}
	for i := range subs {
		args = append(args, "-map", fmt.Sprintf("%d", i+1+len(subs)))
	}

	// Codecs
	args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-c:s", "srt")

	// Audio metadata
	for i, sub := range subs {
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", sub.lang))
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("title=%s", sub.title))
	}

	// Subtitle metadata
	for i, sub := range subs {
		args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("language=%s", sub.lang))
		args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("title=%s", sub.title))
	}

	args = append(args, "-shortest", "-y", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, output)
	}

	return nil
}
```

**Step 5.2: Run integration tests**

```bash
go test ./internal/remux/... -tags=integration -v
```

**Step 5.3: Commit**

```bash
git add internal/remux/
git commit -m "remux: add integration tests with real mkvmerge"
```

---

## Task 6: Update TUI to Show Remux Trigger

**Files:**
- Modify: `internal/tui/itemdetail.go` (verify existing code works)
- Modify: `internal/tui/startstage.go` (verify existing code works)

**Step 6.1: Verify TUI integration**

The TUI should already handle the remux stage via the existing `startStageForItem` function in `startstage.go`. Verify:

1. After organize completes, item detail shows "Press [s] to start remux"
2. Pressing `s` creates a remux job and dispatches the remux binary
3. Job status updates correctly in the database

**Step 6.2: Manual testing**

```bash
# Build all binaries
make build-all

# Set up test environment
./scripts/setup-test-env.sh
export MEDIA_BASE=/tmp/test-media
export MAKEMKVCON_PATH=$(pwd)/bin/mock-makemkv

# Run TUI and test full flow:
# 1. Create new movie
# 2. Complete rip (with mock)
# 3. Organize files manually
# 4. Mark organize complete
# 5. Trigger remux
./bin/media-pipeline
```

**Step 6.3: Commit any fixes**

```bash
git add internal/tui/
git commit -m "tui: verify remux integration"
```

---

## Task 7: Update Test Environment Setup

**Files:**
- Modify: `scripts/setup-test-env.sh`

**Step 7.1: Add remux languages to test config**

Update `scripts/setup-test-env.sh` to include remux config:

```bash
# Create config file
cat > "$TEST_MEDIA_BASE/pipeline/config.yaml" << EOF
staging_base: $TEST_MEDIA_BASE/staging
library_base: $TEST_MEDIA_BASE/library

dispatch:
  rip: ""        # all local for testing
  remux: ""
  transcode: ""
  publish: ""

remux:
  languages:
    - eng
    - bul
EOF
```

**Step 7.2: Commit**

```bash
git add scripts/
git commit -m "scripts: add remux config to test environment"
```

---

## Task 8: End-to-End Verification

**Step 8.1: Build everything**

```bash
make build-all
```

**Step 8.2: Run all tests**

```bash
make test
go test ./internal/remux/... -tags=integration -v
```

**Step 8.3: Manual end-to-end test**

```bash
# Set up fresh test environment
rm -rf /tmp/test-media
./scripts/setup-test-env.sh
export MEDIA_BASE=/tmp/test-media
export MAKEMKVCON_PATH=$(pwd)/bin/mock-makemkv

# Test flow:
./bin/media-pipeline
# 1. Press 'n' to create new movie "Test Movie"
# 2. Wait for mock rip to complete
# 3. Organize files: mv _main etc
# 4. Press 'o' to open organize view
# 5. Press 'v' to validate, 'c' to complete
# 6. Press 's' to start remux
# 7. Verify remux completes and tracks are filtered
```

**Step 8.4: Verify remuxed output**

```bash
# Check output directory exists
ls -la /tmp/test-media/staging/2-remuxed/movies/Test_Movie/_main/

# Verify track filtering with mkvmerge
mkvmerge -J /tmp/test-media/staging/2-remuxed/movies/Test_Movie/_main/*.mkv | jq '.tracks[] | {id, type, language: .properties.language}'
```

**Step 8.5: Final commit**

```bash
git add -A
git commit -m "remux: complete implementation with track filtering"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add remux.languages config | `internal/config/` |
| 2 | Track analysis and filtering | `internal/remux/mkvmerge.go` |
| 3 | Remuxer with file processing | `internal/remux/remuxer.go` |
| 4 | cmd/remux implementation | `cmd/remux/main.go` |
| 5 | Integration tests | `internal/remux/integration_test.go` |
| 6 | TUI verification | `internal/tui/` |
| 7 | Test environment config | `scripts/setup-test-env.sh` |
| 8 | End-to-end verification | - |

## Expected Result

After completing this plan:
1. `remux.languages` configurable in config.yaml (default: ["eng"])
2. Remux stage filters MKV tracks to keep only configured languages
3. Video tracks always preserved
4. Audio and subtitle tracks filtered by language
5. Extras directory copied without modification
6. TUI triggers remux correctly after organize
7. Full end-to-end flow working with mock-makemkv generated files

## Dependencies

- `mkvtoolnix-cli` (mkvmerge) installed on the system
- `ffmpeg` for running integration tests
- Existing TUI database integration working
