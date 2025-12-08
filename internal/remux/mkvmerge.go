package remux

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Track represents a single track in an MKV file
type Track struct {
	ID       int
	Type     string // "video", "audio", "subtitles"
	Codec    string
	Language string
	Title    string
	Forced   bool
	Default  bool
}

// TrackInfo holds parsed track information from mkvmerge -J
type TrackInfo struct {
	Video     []Track
	Audio     []Track
	Subtitles []Track
}

// mkvmergeJSON represents the JSON output from mkvmerge -J
type mkvmergeJSON struct {
	Container struct {
		Type string `json:"type"`
	} `json:"container"`
	Tracks []struct {
		ID         int    `json:"id"`
		Type       string `json:"type"`
		Codec      string `json:"codec"`
		Properties struct {
			Language     string `json:"language"`
			TrackName    string `json:"track_name"`
			ForcedTrack  bool   `json:"forced_track"`
			DefaultTrack bool   `json:"default_track"`
		} `json:"properties"`
	} `json:"tracks"`
}

// GetTrackInfo runs mkvmerge -J on a file and returns parsed track info
func GetTrackInfo(path string) (*TrackInfo, error) {
	cmd := exec.Command("mkvmerge", "-J", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mkvmerge -J failed: %w", err)
	}

	return ParseTrackInfo(output)
}

// ParseTrackInfo parses mkvmerge -J JSON output into TrackInfo
func ParseTrackInfo(jsonData []byte) (*TrackInfo, error) {
	var data mkvmergeJSON
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse mkvmerge output: %w", err)
	}

	info := &TrackInfo{}
	for _, t := range data.Tracks {
		track := Track{
			ID:       t.ID,
			Type:     t.Type,
			Codec:    t.Codec,
			Language: t.Properties.Language,
			Title:    t.Properties.TrackName,
			Forced:   t.Properties.ForcedTrack,
			Default:  t.Properties.DefaultTrack,
		}

		switch t.Type {
		case "video":
			info.Video = append(info.Video, track)
		case "audio":
			info.Audio = append(info.Audio, track)
		case "subtitles":
			info.Subtitles = append(info.Subtitles, track)
		}
	}

	return info, nil
}

// FilterTracks returns a new TrackInfo containing only tracks with matching languages
// Video tracks are always kept. Audio and subtitle tracks are filtered by language.
func FilterTracks(info *TrackInfo, languages []string) *TrackInfo {
	langSet := make(map[string]bool)
	for _, lang := range languages {
		langSet[strings.ToLower(lang)] = true
	}

	filtered := &TrackInfo{
		Video: info.Video, // Keep all video tracks
	}

	for _, track := range info.Audio {
		if langSet[strings.ToLower(track.Language)] {
			filtered.Audio = append(filtered.Audio, track)
		}
	}

	for _, track := range info.Subtitles {
		if langSet[strings.ToLower(track.Language)] {
			filtered.Subtitles = append(filtered.Subtitles, track)
		}
	}

	return filtered
}

// BuildMkvmergeArgs builds mkvmerge command arguments for remuxing with filtered tracks
func BuildMkvmergeArgs(inputPath, outputPath string, tracks *TrackInfo) []string {
	args := []string{"-o", outputPath}

	// Build track selection arguments
	// Video: always keep all
	if len(tracks.Video) > 0 {
		var videoIDs []string
		for _, v := range tracks.Video {
			videoIDs = append(videoIDs, fmt.Sprintf("%d", v.ID))
		}
		args = append(args, "--video-tracks", strings.Join(videoIDs, ","))
	} else {
		args = append(args, "--no-video")
	}

	// Audio: keep filtered tracks
	if len(tracks.Audio) > 0 {
		var audioIDs []string
		for _, a := range tracks.Audio {
			audioIDs = append(audioIDs, fmt.Sprintf("%d", a.ID))
		}
		args = append(args, "--audio-tracks", strings.Join(audioIDs, ","))
	} else {
		args = append(args, "--no-audio")
	}

	// Subtitles: keep filtered tracks
	if len(tracks.Subtitles) > 0 {
		var subIDs []string
		for _, s := range tracks.Subtitles {
			subIDs = append(subIDs, fmt.Sprintf("%d", s.ID))
		}
		args = append(args, "--subtitle-tracks", strings.Join(subIDs, ","))
	} else {
		args = append(args, "--no-subtitles")
	}

	args = append(args, inputPath)
	return args
}

// RunMkvmerge executes mkvmerge with the given arguments
func RunMkvmerge(args []string) error {
	cmd := exec.Command("mkvmerge", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkvmerge failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}
