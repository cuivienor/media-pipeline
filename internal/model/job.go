package model

import "time"

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// Job represents a single stage execution attempt
type Job struct {
	ID           int64
	MediaItemID  int64
	Stage        Stage
	Status       JobStatus
	Disc         *int
	WorkerID     string
	PID          int
	InputDir     string
	OutputDir    string
	LogPath      string
	ErrorMessage string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

// IsActive returns true if the job is pending or in progress
func (j *Job) IsActive() bool {
	return j.Status == JobStatusPending || j.Status == JobStatusInProgress
}

// Duration returns the job duration, or zero if not completed
func (j *Job) Duration() time.Duration {
	if j.StartedAt == nil || j.CompletedAt == nil {
		return 0
	}
	return j.CompletedAt.Sub(*j.StartedAt)
}

// LogEvent represents a significant event during job execution
type LogEvent struct {
	ID        int64
	JobID     int64
	Level     string // "info", "warn", "error"
	Message   string
	Timestamp time.Time
}

// DiscProgress tracks rip status for a TV disc
type DiscProgress struct {
	Disc   int
	Status JobStatus
	JobID  int64
}
