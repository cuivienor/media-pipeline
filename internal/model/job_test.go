package model

import (
	"testing"
	"time"
)

func TestJob_IsActive(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   bool
	}{
		{JobStatusPending, true},
		{JobStatusInProgress, true},
		{JobStatusCompleted, false},
		{JobStatusFailed, false},
	}
	for _, tt := range tests {
		job := Job{Status: tt.status}
		if got := job.IsActive(); got != tt.want {
			t.Errorf("Job{Status: %q}.IsActive() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestJob_Duration(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	job := Job{
		StartedAt:   &start,
		CompletedAt: &end,
	}

	duration := job.Duration()
	if duration < 59*time.Minute || duration > 61*time.Minute {
		t.Errorf("Duration() = %v, want ~1 hour", duration)
	}
}

func TestJob_Duration_NotStarted(t *testing.T) {
	job := Job{
		StartedAt:   nil,
		CompletedAt: nil,
	}

	duration := job.Duration()
	if duration != 0 {
		t.Errorf("Duration() = %v, want 0 for not started job", duration)
	}
}

func TestJob_Duration_InProgress(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	job := Job{
		StartedAt:   &start,
		CompletedAt: nil,
	}

	duration := job.Duration()
	if duration != 0 {
		t.Errorf("Duration() = %v, want 0 for in-progress job", duration)
	}
}
