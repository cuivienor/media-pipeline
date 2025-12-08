package remux

import (
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

func TestBuildMkvmergeArgs(t *testing.T) {
	tracks := &TrackInfo{
		Video: []Track{
			{ID: 0, Type: "video"},
		},
		Audio: []Track{
			{ID: 1, Type: "audio", Language: "eng"},
			{ID: 2, Type: "audio", Language: "bul"},
		},
		Subtitles: []Track{
			{ID: 4, Type: "subtitles", Language: "eng"},
			{ID: 5, Type: "subtitles", Language: "eng", Forced: true},
		},
	}

	args := BuildMkvmergeArgs("/input/file.mkv", "/output/file.mkv", tracks)

	// Check output path
	if args[0] != "-o" || args[1] != "/output/file.mkv" {
		t.Errorf("Expected -o /output/file.mkv at start, got %v", args[0:2])
	}

	// Check video tracks
	foundVideoTracks := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--video-tracks" && args[i+1] == "0" {
			foundVideoTracks = true
			break
		}
	}
	if !foundVideoTracks {
		t.Error("Expected --video-tracks 0 in args")
	}

	// Check audio tracks
	foundAudioTracks := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--audio-tracks" && args[i+1] == "1,2" {
			foundAudioTracks = true
			break
		}
	}
	if !foundAudioTracks {
		t.Error("Expected --audio-tracks 1,2 in args")
	}

	// Check subtitle tracks
	foundSubtitleTracks := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--subtitle-tracks" && args[i+1] == "4,5" {
			foundSubtitleTracks = true
			break
		}
	}
	if !foundSubtitleTracks {
		t.Error("Expected --subtitle-tracks 4,5 in args")
	}

	// Check input path is last
	if args[len(args)-1] != "/input/file.mkv" {
		t.Errorf("Expected input path at end, got %s", args[len(args)-1])
	}
}

func TestBuildMkvmergeArgs_NoTracks(t *testing.T) {
	tracks := &TrackInfo{
		Video:     []Track{{ID: 0, Type: "video"}},
		Audio:     []Track{},
		Subtitles: []Track{},
	}

	args := BuildMkvmergeArgs("/input/file.mkv", "/output/file.mkv", tracks)

	// Should have --no-audio and --no-subtitles
	hasNoAudio := false
	hasNoSubtitles := false
	for _, arg := range args {
		if arg == "--no-audio" {
			hasNoAudio = true
		}
		if arg == "--no-subtitles" {
			hasNoSubtitles = true
		}
	}

	if !hasNoAudio {
		t.Error("Expected --no-audio when no audio tracks")
	}
	if !hasNoSubtitles {
		t.Error("Expected --no-subtitles when no subtitle tracks")
	}
}
