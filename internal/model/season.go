package model

import "time"

// Season represents a TV show season that moves through the pipeline
type Season struct {
	ID           int64
	ItemID       int64     // Foreign key to Item (TV show)
	Number       int       // Season number (1, 2, 3...)
	CurrentStage Stage     // Current pipeline stage
	StageStatus  Status    // Status of current stage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsReadyForNextStage returns true if the season has completed its current stage
func (s *Season) IsReadyForNextStage() bool {
	return s.StageStatus == StatusCompleted && s.CurrentStage != StagePublish
}

// IsFailed returns true if the season is in a failed state
func (s *Season) IsFailed() bool {
	return s.StageStatus == StatusFailed
}

// IsInProgress returns true if the season is currently being processed
func (s *Season) IsInProgress() bool {
	return s.StageStatus == StatusInProgress
}

// IsComplete returns true if the season has been published
func (s *Season) IsComplete() bool {
	return s.CurrentStage == StagePublish && s.StageStatus == StatusCompleted
}

// NextAction returns a human-readable description of what needs to happen next
func (s *Season) NextAction() string {
	if s.StageStatus == StatusFailed {
		return "retry " + s.CurrentStage.String()
	}
	if s.StageStatus == StatusInProgress {
		return s.CurrentStage.String() + " in progress"
	}
	if s.StageStatus == StatusCompleted {
		return s.CurrentStage.NextStage().String()
	}
	return "start " + s.CurrentStage.String()
}
