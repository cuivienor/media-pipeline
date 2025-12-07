package model

import (
	"testing"
)

func TestMediaItem_UniqueKey(t *testing.T) {
	tests := []struct {
		name string
		item MediaItem
		want string
	}{
		{
			name: "movie",
			item: MediaItem{Type: MediaTypeMovie, SafeName: "The_Matrix"},
			want: "The_Matrix",
		},
		{
			name: "tv with season",
			item: MediaItem{Type: MediaTypeTV, SafeName: "Breaking_Bad", Season: "S02"},
			want: "Breaking_Bad_S02",
		},
		{
			name: "tv without season",
			item: MediaItem{Type: MediaTypeTV, SafeName: "Breaking_Bad"},
			want: "Breaking_Bad",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.UniqueKey(); got != tt.want {
				t.Errorf("MediaItem.UniqueKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaItem_IsReadyForNextStage(t *testing.T) {
	tests := []struct {
		name string
		item MediaItem
		want bool
	}{
		{
			name: "completed not in library",
			item: MediaItem{Status: StatusCompleted, Current: StageTranscode},
			want: true,
		},
		{
			name: "completed in library",
			item: MediaItem{Status: StatusCompleted, Current: StagePublish},
			want: false,
		},
		{
			name: "in progress",
			item: MediaItem{Status: StatusInProgress, Current: StageRip},
			want: false,
		},
		{
			name: "failed",
			item: MediaItem{Status: StatusFailed, Current: StageRemux},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsReadyForNextStage(); got != tt.want {
				t.Errorf("MediaItem.IsReadyForNextStage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPipelineState_CountByStage(t *testing.T) {
	state := &PipelineState{
		Items: []MediaItem{
			{Current: StageRip},
			{Current: StageRip},
			{Current: StageRemux},
			{Current: StagePublish},
		},
	}
	counts := state.CountByStage()
	if counts[StageRip] != 2 {
		t.Errorf("CountByStage[Rip] = %d, want 2", counts[StageRip])
	}
	if counts[StageRemux] != 1 {
		t.Errorf("CountByStage[Remux] = %d, want 1", counts[StageRemux])
	}
	if counts[StagePublish] != 1 {
		t.Errorf("CountByStage[Publish] = %d, want 1", counts[StagePublish])
	}
}

func TestPipelineState_ItemsAtStage(t *testing.T) {
	state := &PipelineState{
		Items: []MediaItem{
			{SafeName: "Movie1", Current: StageRip},
			{SafeName: "Movie2", Current: StageRip},
			{SafeName: "Movie3", Current: StageRemux},
		},
	}
	ripped := state.ItemsAtStage(StageRip)
	if len(ripped) != 2 {
		t.Errorf("ItemsAtStage(Rip) returned %d items, want 2", len(ripped))
	}
}

func TestPipelineState_FilterMethods(t *testing.T) {
	state := &PipelineState{
		Items: []MediaItem{
			{SafeName: "Ready", Status: StatusCompleted, Current: StageRip},
			{SafeName: "InProgress", Status: StatusInProgress, Current: StageRemux},
			{SafeName: "Failed", Status: StatusFailed, Current: StageTranscode},
			{SafeName: "Done", Status: StatusCompleted, Current: StagePublish},
		},
	}

	ready := state.ItemsReadyForNextStage()
	if len(ready) != 1 || ready[0].SafeName != "Ready" {
		t.Errorf("ItemsReadyForNextStage() = %v, want [Ready]", ready)
	}

	inProgress := state.ItemsInProgress()
	if len(inProgress) != 1 || inProgress[0].SafeName != "InProgress" {
		t.Errorf("ItemsInProgress() = %v, want [InProgress]", inProgress)
	}

	failed := state.ItemsFailed()
	if len(failed) != 1 || failed[0].SafeName != "Failed" {
		t.Errorf("ItemsFailed() = %v, want [Failed]", failed)
	}
}
