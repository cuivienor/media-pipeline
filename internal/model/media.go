package model

import (
	"fmt"
	"time"
)

// Stage represents a step in the media processing pipeline
type Stage int

const (
	StageRip Stage = iota
	StageOrganize
	StageRemux
	StageTranscode
	StagePublish
)

func (s Stage) String() string {
	switch s {
	case StageRip:
		return "rip"
	case StageOrganize:
		return "organize"
	case StageRemux:
		return "remux"
	case StageTranscode:
		return "transcode"
	case StagePublish:
		return "publish"
	default:
		return "unknown"
	}
}

func (s Stage) DisplayName() string {
	switch s {
	case StageRip:
		return "1-Ripped"
	case StageOrganize:
		return "2-Organized"
	case StageRemux:
		return "3-Remuxed"
	case StageTranscode:
		return "4-Transcoded"
	case StagePublish:
		return "Library"
	default:
		return "Unknown"
	}
}

func (s Stage) NextStage() Stage {
	switch s {
	case StageRip:
		return StageOrganize
	case StageOrganize:
		return StageRemux
	case StageRemux:
		return StageTranscode
	case StageTranscode:
		return StagePublish
	default:
		return StagePublish
	}
}

func (s Stage) NextAction() string {
	switch s {
	case StageRip:
		return "needs organize"
	case StageOrganize:
		return "needs remux"
	case StageRemux:
		return "needs transcode"
	case StageTranscode:
		return "needs publish"
	case StagePublish:
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
	ID       int64     // Database ID (0 if not persisted)
	Type     MediaType // "movie" or "tv"
	Name     string    // Human-readable name like "The Lion King"
	SafeName string    // Filesystem-safe name like "The_Lion_King"
	Season   *int      // Season number for TV (nil for movies)

	// Pipeline state
	Stages  []StageInfo // History of all stages this item has been through
	Current Stage       // The furthest completed stage
	Status  Status      // Status of the current stage
}

// UniqueKey returns a string that uniquely identifies this media item
func (m *MediaItem) UniqueKey() string {
	if m.Type == MediaTypeTV && m.Season != nil {
		return fmt.Sprintf("%s_S%02d", m.SafeName, *m.Season)
	}
	return m.SafeName
}

// IsReadyForNextStage returns true if the item has completed its current stage
func (m *MediaItem) IsReadyForNextStage() bool {
	return m.Status == StatusCompleted && m.Current != StagePublish
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
