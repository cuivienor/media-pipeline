package model

import "time"

// Stage represents a step in the media processing pipeline
type Stage int

const (
	StageRipped Stage = iota
	StageRemuxed
	StageTranscoded
	StageInLibrary
)

func (s Stage) String() string {
	switch s {
	case StageRipped:
		return "ripped"
	case StageRemuxed:
		return "remuxed"
	case StageTranscoded:
		return "transcoded"
	case StageInLibrary:
		return "in_library"
	default:
		return "unknown"
	}
}

func (s Stage) DisplayName() string {
	switch s {
	case StageRipped:
		return "1-Ripped"
	case StageRemuxed:
		return "2-Remuxed"
	case StageTranscoded:
		return "3-Transcoded"
	case StageInLibrary:
		return "Library"
	default:
		return "Unknown"
	}
}

func (s Stage) NextStage() Stage {
	switch s {
	case StageRipped:
		return StageRemuxed
	case StageRemuxed:
		return StageTranscoded
	case StageTranscoded:
		return StageInLibrary
	default:
		return StageInLibrary
	}
}

func (s Stage) NextAction() string {
	switch s {
	case StageRipped:
		return "needs remux"
	case StageRemuxed:
		return "needs transcode"
	case StageTranscoded:
		return "needs filebot"
	case StageInLibrary:
		return "complete"
	default:
		return "unknown"
	}
}

// Status represents the current state of a stage
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// MediaType distinguishes movies from TV shows
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

// StageInfo contains metadata about a specific pipeline stage for an item
type StageInfo struct {
	Stage       Stage
	Status      Status
	StartedAt   time.Time
	CompletedAt time.Time
	Path        string
	Metadata    map[string]interface{}
}

// MediaItem represents a single media item (movie or TV season) in the pipeline
type MediaItem struct {
	Type     MediaType // "movie" or "tv"
	Name     string    // Human-readable name like "The Lion King"
	SafeName string    // Filesystem-safe name like "The_Lion_King"
	Season   string    // Season identifier for TV (e.g., "S02"), empty for movies

	// Pipeline state
	Stages  []StageInfo // History of all stages this item has been through
	Current Stage       // The furthest completed stage
	Status  Status      // Status of the current stage
}

// UniqueKey returns a string that uniquely identifies this media item
func (m *MediaItem) UniqueKey() string {
	if m.Type == MediaTypeTV && m.Season != "" {
		return m.SafeName + "_" + m.Season
	}
	return m.SafeName
}

// IsReadyForNextStage returns true if the item has completed its current stage
func (m *MediaItem) IsReadyForNextStage() bool {
	return m.Status == StatusCompleted && m.Current != StageInLibrary
}

// IsFailed returns true if the item is in a failed state
func (m *MediaItem) IsFailed() bool {
	return m.Status == StatusFailed
}

// IsInProgress returns true if the item is currently being processed
func (m *MediaItem) IsInProgress() bool {
	return m.Status == StatusInProgress
}

// GetStageInfo returns the StageInfo for a specific stage, or nil if not found
func (m *MediaItem) GetStageInfo(stage Stage) *StageInfo {
	for i := range m.Stages {
		if m.Stages[i].Stage == stage {
			return &m.Stages[i]
		}
	}
	return nil
}

// PipelineState holds the complete state of the media pipeline
type PipelineState struct {
	Items     []MediaItem
	ScannedAt time.Time
}

// CountByStage returns the number of items at each stage
func (p *PipelineState) CountByStage() map[Stage]int {
	counts := make(map[Stage]int)
	for _, item := range p.Items {
		counts[item.Current]++
	}
	return counts
}

// ItemsAtStage returns all items currently at the specified stage
func (p *PipelineState) ItemsAtStage(stage Stage) []MediaItem {
	var result []MediaItem
	for _, item := range p.Items {
		if item.Current == stage {
			result = append(result, item)
		}
	}
	return result
}

// ItemsReadyForNextStage returns all items that have completed their current stage
func (p *PipelineState) ItemsReadyForNextStage() []MediaItem {
	var result []MediaItem
	for _, item := range p.Items {
		if item.IsReadyForNextStage() {
			result = append(result, item)
		}
	}
	return result
}

// ItemsInProgress returns all items currently being processed
func (p *PipelineState) ItemsInProgress() []MediaItem {
	var result []MediaItem
	for _, item := range p.Items {
		if item.IsInProgress() {
			result = append(result, item)
		}
	}
	return result
}

// ItemsFailed returns all items in a failed state
func (p *PipelineState) ItemsFailed() []MediaItem {
	var result []MediaItem
	for _, item := range p.Items {
		if item.IsFailed() {
			result = append(result, item)
		}
	}
	return result
}
