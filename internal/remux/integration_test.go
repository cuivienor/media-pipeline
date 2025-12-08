//go:build integration

package remux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRemuxer_Integration(t *testing.T) {
	// Skip if mkvmerge not installed
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed")
	}

	// Skip if ffmpeg not installed (needed for test file generation)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Generate test MKV with multiple tracks using ffmpeg
	// This mimics what mock-makemkv does
	inputPath := filepath.Join(tmpDir, "input", "_main", "movie.mkv")
	if err := generateMultiTrackMKV(inputPath); err != nil {
		t.Fatalf("Failed to generate test MKV: %v", err)
	}

	// Verify input has expected tracks
	inputInfo, err := GetTrackInfo(inputPath)
	if err != nil {
		t.Fatalf("GetTrackInfo(input) error: %v", err)
	}
	t.Logf("Input tracks: video=%d, audio=%d, subtitles=%d",
		len(inputInfo.Video), len(inputInfo.Audio), len(inputInfo.Subtitles))

	// Remux with eng+bul filter
	outputDir := filepath.Join(tmpDir, "output")
	remuxer := NewRemuxer([]string{"eng", "bul"})

	results, err := remuxer.RemuxDirectory(context.Background(),
		filepath.Join(tmpDir, "input"), outputDir, false)
	if err != nil {
		t.Fatalf("RemuxDirectory() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Verify output
	outputPath := filepath.Join(outputDir, "_main", "movie.mkv")
	outputInfo, err := GetTrackInfo(outputPath)
	if err != nil {
		t.Fatalf("GetTrackInfo(output) error: %v", err)
	}

	t.Logf("Output tracks: video=%d, audio=%d, subtitles=%d",
		len(outputInfo.Video), len(outputInfo.Audio), len(outputInfo.Subtitles))

	// Verify only eng/bul audio tracks remain
	for _, audio := range outputInfo.Audio {
		if audio.Language != "eng" && audio.Language != "bul" {
			t.Errorf("Unexpected audio language: %s", audio.Language)
		}
	}

	// Verify only eng/bul subtitle tracks remain
	for _, sub := range outputInfo.Subtitles {
		if sub.Language != "eng" && sub.Language != "bul" {
			t.Errorf("Unexpected subtitle language: %s", sub.Language)
		}
	}

	// Should have removed fra and spa tracks
	result := results[0]
	if result.TracksRemoved == 0 {
		t.Error("Expected some tracks to be removed")
	}
	t.Logf("Tracks removed: %d", result.TracksRemoved)
}

// generateMultiTrackMKV creates an MKV file with eng, bul, fra, spa audio and subtitle tracks
func generateMultiTrackMKV(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	durationSec := 3

	// Create subtitle files
	subs := []struct {
		lang  string
		title string
	}{
		{"eng", "English"},
		{"bul", "Bulgarian"},
		{"fra", "French"},
		{"spa", "Spanish"},
	}

	var subFiles []string
	for i, sub := range subs {
		srtPath := filepath.Join(dir, fmt.Sprintf(".sub_%d.srt", i))
		content := fmt.Sprintf("1\n00:00:00,000 --> 00:00:02,000\n%s\n", sub.title)
		if err := os.WriteFile(srtPath, []byte(content), 0644); err != nil {
			return err
		}
		subFiles = append(subFiles, srtPath)
	}
	defer func() {
		for _, f := range subFiles {
			os.Remove(f)
		}
	}()

	// Build ffmpeg command
	args := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=24", durationSec),
	}

	// Add 4 audio inputs
	for range subs {
		args = append(args, "-f", "lavfi", "-i",
			fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%d", durationSec))
	}

	// Add 4 subtitle inputs
	for _, f := range subFiles {
		args = append(args, "-i", f)
	}

	// Map all streams
	args = append(args, "-map", "0:v")
	for i := range subs {
		args = append(args, "-map", fmt.Sprintf("%d:a", i+1))
	}
	for i := range subs {
		args = append(args, "-map", fmt.Sprintf("%d", i+1+len(subs)))
	}

	// Codecs
	args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-c:s", "srt")

	// Audio metadata
	for i, sub := range subs {
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", sub.lang))
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("title=%s", sub.title))
	}

	// Subtitle metadata
	for i, sub := range subs {
		args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("language=%s", sub.lang))
		args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("title=%s", sub.title))
	}

	args = append(args, "-shortest", "-y", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, output)
	}

	return nil
}
