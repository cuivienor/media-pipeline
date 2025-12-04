package model

import (
	"testing"
)

func TestStage_String(t *testing.T) {
	tests := []struct {
		stage Stage
		want  string
	}{
		{StageRipped, "ripped"},
		{StageRemuxed, "remuxed"},
		{StageTranscoded, "transcoded"},
		{StageInLibrary, "in_library"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.stage.String(); got != tt.want {
				t.Errorf("Stage.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStage_DisplayName(t *testing.T) {
	tests := []struct {
		stage Stage
		want  string
	}{
		{StageRipped, "1-Ripped"},
		{StageRemuxed, "2-Remuxed"},
		{StageTranscoded, "3-Transcoded"},
		{StageInLibrary, "Library"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.stage.DisplayName(); got != tt.want {
				t.Errorf("Stage.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStage_NextStage(t *testing.T) {
	tests := []struct {
		stage Stage
		want  Stage
	}{
		{StageRipped, StageRemuxed},
		{StageRemuxed, StageTranscoded},
		{StageTranscoded, StageInLibrary},
		{StageInLibrary, StageInLibrary},
	}
	for _, tt := range tests {
		t.Run(tt.stage.String(), func(t *testing.T) {
			if got := tt.stage.NextStage(); got != tt.want {
				t.Errorf("Stage.NextStage() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			item: MediaItem{Status: StatusCompleted, Current: StageTranscoded},
			want: true,
		},
		{
			name: "completed in library",
			item: MediaItem{Status: StatusCompleted, Current: StageInLibrary},
			want: false,
		},
		{
			name: "in progress",
			item: MediaItem{Status: StatusInProgress, Current: StageRipped},
			want: false,
		},
		{
			name: "failed",
			item: MediaItem{Status: StatusFailed, Current: StageRemuxed},
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
			{Current: StageRipped},
			{Current: StageRipped},
			{Current: StageRemuxed},
			{Current: StageInLibrary},
		},
	}
	counts := state.CountByStage()
	if counts[StageRipped] != 2 {
		t.Errorf("CountByStage[Ripped] = %d, want 2", counts[StageRipped])
	}
	if counts[StageRemuxed] != 1 {
		t.Errorf("CountByStage[Remuxed] = %d, want 1", counts[StageRemuxed])
	}
	if counts[StageInLibrary] != 1 {
		t.Errorf("CountByStage[InLibrary] = %d, want 1", counts[StageInLibrary])
	}
}

func TestPipelineState_ItemsAtStage(t *testing.T) {
	state := &PipelineState{
		Items: []MediaItem{
			{SafeName: "Movie1", Current: StageRipped},
			{SafeName: "Movie2", Current: StageRipped},
			{SafeName: "Movie3", Current: StageRemuxed},
		},
	}
	ripped := state.ItemsAtStage(StageRipped)
	if len(ripped) != 2 {
		t.Errorf("ItemsAtStage(Ripped) returned %d items, want 2", len(ripped))
	}
}

func TestPipelineState_FilterMethods(t *testing.T) {
	state := &PipelineState{
		Items: []MediaItem{
			{SafeName: "Ready", Status: StatusCompleted, Current: StageRipped},
			{SafeName: "InProgress", Status: StatusInProgress, Current: StageRemuxed},
			{SafeName: "Failed", Status: StatusFailed, Current: StageTranscoded},
			{SafeName: "Done", Status: StatusCompleted, Current: StageInLibrary},
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
