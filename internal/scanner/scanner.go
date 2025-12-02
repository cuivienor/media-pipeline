package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/petercsiba/media-pipeline/internal/model"
)

// Config holds scanner configuration
type Config struct {
	StagingBase string // e.g., "/mnt/media/staging"
	LibraryBase string // e.g., "/mnt/media/library"
}

// DefaultConfig returns the default configuration for production
func DefaultConfig() Config {
	return Config{
		StagingBase: "/mnt/media/staging",
		LibraryBase: "/mnt/media/library",
	}
}

// Scanner reads the filesystem to build pipeline state
type Scanner struct {
	config Config
}

// New creates a new Scanner with the given configuration
func New(config Config) *Scanner {
	return &Scanner{config: config}
}

// stageMetadata is the JSON structure from metadata.json files
type stageMetadata struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	SafeName  string `json:"safe_name"`
	Season    string `json:"season"`
	Disc      string `json:"disc,omitempty"`
	InputDir  string `json:"input_dir,omitempty"`
	OutputDir string `json:"output_dir,omitempty"`
	StartedAt string `json:"started_at"`
	CRF       string `json:"crf,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Database  string `json:"database,omitempty"`
}

// stageResult holds the result of scanning a single stage directory
type stageResult struct {
	key      string // unique key: safe_name or safe_name_season
	stage    model.Stage
	info     model.StageInfo
	metadata stageMetadata
}

// ScanPipeline scans all staging directories and builds the pipeline state
func (s *Scanner) ScanPipeline() (*model.PipelineState, error) {
	// Collect all stage results
	var results []stageResult

	// Scan each stage
	rippedResults, err := s.scanStage(model.StageRipped, "1-ripped", ".rip")
	if err != nil {
		return nil, err
	}
	results = append(results, rippedResults...)

	remuxedResults, err := s.scanStage(model.StageRemuxed, "2-remuxed", ".remux")
	if err != nil {
		return nil, err
	}
	results = append(results, remuxedResults...)

	transcodedResults, err := s.scanStage(model.StageTranscoded, "3-transcoded", ".transcode")
	if err != nil {
		return nil, err
	}
	results = append(results, transcodedResults...)

	// Also check for .filebot directories in 3-transcoded (indicates filebot was run)
	filebotResults, err := s.scanStage(model.StageInLibrary, "3-transcoded", ".filebot")
	if err != nil {
		return nil, err
	}
	results = append(results, filebotResults...)

	// Merge results into MediaItems by unique key
	items := s.mergeResults(results)

	return &model.PipelineState{
		Items:     items,
		ScannedAt: time.Now(),
	}, nil
}

// scanStage scans a staging directory for state directories
func (s *Scanner) scanStage(stage model.Stage, stageDirName, stateDirName string) ([]stageResult, error) {
	var results []stageResult

	stagePath := filepath.Join(s.config.StagingBase, stageDirName)

	// Check if stage directory exists
	if _, err := os.Stat(stagePath); os.IsNotExist(err) {
		return results, nil
	}

	// Scan both movies and tv subdirectories
	for _, mediaType := range []string{"movies", "tv"} {
		mediaPath := filepath.Join(stagePath, mediaType)
		if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
			continue
		}

		// Walk the directory tree looking for state directories
		err := filepath.Walk(mediaPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors, continue scanning
			}

			// Look for state directories (.rip, .remux, .transcode, .filebot)
			if info.IsDir() && info.Name() == stateDirName {
				result, err := s.parseStateDir(path, stage)
				if err == nil && result != nil {
					results = append(results, *result)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// parseStateDir reads a state directory and returns a stageResult
func (s *Scanner) parseStateDir(stateDir string, stage model.Stage) (*stageResult, error) {
	// Read metadata.json
	metadataPath := filepath.Join(stateDir, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata stageMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, err
	}

	// Read status file
	statusPath := filepath.Join(stateDir, "status")
	statusBytes, err := os.ReadFile(statusPath)
	status := model.StatusPending
	if err == nil {
		statusStr := strings.TrimSpace(string(statusBytes))
		switch statusStr {
		case "completed":
			status = model.StatusCompleted
		case "in_progress":
			status = model.StatusInProgress
		case "failed":
			status = model.StatusFailed
		}
	}

	// Parse timestamps
	startedAt, _ := time.Parse(time.RFC3339, metadata.StartedAt)

	// Read completed_at if it exists
	var completedAt time.Time
	completedAtPath := filepath.Join(stateDir, "completed_at")
	if completedAtBytes, err := os.ReadFile(completedAtPath); err == nil {
		completedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(string(completedAtBytes)))
	}

	// Build unique key
	key := metadata.SafeName
	if metadata.Type == "tv" && metadata.Season != "" {
		// Normalize season format (S02 or Season_02)
		season := metadata.Season
		if strings.HasPrefix(season, "Season_") {
			season = "S" + strings.TrimPrefix(season, "Season_")
		}
		key = metadata.SafeName + "_" + season
	}

	// Get the parent directory (the actual media item directory)
	itemDir := filepath.Dir(stateDir)

	return &stageResult{
		key:   key,
		stage: stage,
		info: model.StageInfo{
			Stage:       stage,
			Status:      status,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			Path:        itemDir,
			Metadata:    make(map[string]interface{}),
		},
		metadata: metadata,
	}, nil
}

// mergeResults combines stage results into MediaItems
func (s *Scanner) mergeResults(results []stageResult) []model.MediaItem {
	// Group by unique key
	itemMap := make(map[string]*model.MediaItem)

	for _, result := range results {
		item, exists := itemMap[result.key]
		if !exists {
			// Create new item
			mediaType := model.MediaTypeMovie
			if result.metadata.Type == "tv" {
				mediaType = model.MediaTypeTV
			}

			// Extract season from key if TV
			season := ""
			if mediaType == model.MediaTypeTV {
				parts := strings.Split(result.key, "_")
				if len(parts) > 0 {
					lastPart := parts[len(parts)-1]
					if strings.HasPrefix(lastPart, "S") && len(lastPart) <= 4 {
						season = lastPart
					}
				}
				// Also check metadata
				if result.metadata.Season != "" {
					season = result.metadata.Season
					if strings.HasPrefix(season, "Season_") {
						season = "S" + strings.TrimPrefix(season, "Season_")
					}
				}
			}

			item = &model.MediaItem{
				Type:     mediaType,
				Name:     result.metadata.Name,
				SafeName: result.metadata.SafeName,
				Season:   season,
				Stages:   []model.StageInfo{},
				Current:  result.stage,
				Status:   result.info.Status,
			}
			itemMap[result.key] = item
		}

		// Add stage info
		item.Stages = append(item.Stages, result.info)

		// Update current stage if this is further along (and completed)
		// The "furthest" stage is the one we're currently at
		if result.stage > item.Current {
			item.Current = result.stage
			item.Status = result.info.Status
		} else if result.stage == item.Current {
			// Same stage, update status
			item.Status = result.info.Status
		}
	}

	// Convert map to slice
	items := make([]model.MediaItem, 0, len(itemMap))
	for _, item := range itemMap {
		items = append(items, *item)
	}

	return items
}
