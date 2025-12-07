package model

import "testing"

func TestStage_String(t *testing.T) {
	tests := []struct {
		stage Stage
		want  string
	}{
		{StageRip, "rip"},
		{StageOrganize, "organize"},
		{StageRemux, "remux"},
		{StageTranscode, "transcode"},
		{StagePublish, "publish"},
	}
	for _, tt := range tests {
		if got := tt.stage.String(); got != tt.want {
			t.Errorf("Stage(%d).String() = %q, want %q", tt.stage, got, tt.want)
		}
	}
}

func TestStage_DisplayName(t *testing.T) {
	tests := []struct {
		stage Stage
		want  string
	}{
		{StageRip, "1-Ripped"},
		{StageOrganize, "2-Organized"},
		{StageRemux, "3-Remuxed"},
		{StageTranscode, "4-Transcoded"},
		{StagePublish, "Library"},
	}
	for _, tt := range tests {
		if got := tt.stage.DisplayName(); got != tt.want {
			t.Errorf("Stage(%d).DisplayName() = %q, want %q", tt.stage, got, tt.want)
		}
	}
}

func TestStage_NextStage(t *testing.T) {
	tests := []struct {
		stage Stage
		want  Stage
	}{
		{StageRip, StageOrganize},
		{StageOrganize, StageRemux},
		{StageRemux, StageTranscode},
		{StageTranscode, StagePublish},
		{StagePublish, StagePublish}, // terminal
	}
	for _, tt := range tests {
		if got := tt.stage.NextStage(); got != tt.want {
			t.Errorf("Stage(%d).NextStage() = %d, want %d", tt.stage, got, tt.want)
		}
	}
}
