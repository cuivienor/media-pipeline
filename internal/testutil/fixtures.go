package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// MKVOptions configures synthetic MKV generation
type MKVOptions struct {
	DurationSec int      // Video duration in seconds (default: 1)
	AudioLangs  []string // Audio track languages (default: ["eng"])
	SubLangs    []string // Subtitle track languages (default: ["eng"])
}

// GenerateTestMKV creates a synthetic MKV file with specified tracks
func GenerateTestMKV(outputPath string, opts MKVOptions) error {
	if opts.DurationSec == 0 {
		opts.DurationSec = 1
	}
	if len(opts.AudioLangs) == 0 {
		opts.AudioLangs = []string{"eng"}
	}
	if len(opts.SubLangs) == 0 {
		opts.SubLangs = []string{"eng"}
	}

	if outputPath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create subtitle files
	var subFiles []string
	for i, lang := range opts.SubLangs {
		srtPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf(".sub_%d.srt", i))
		content := fmt.Sprintf("1\n00:00:00,000 --> 00:00:%02d,000\n%s subtitle\n", opts.DurationSec, lang)
		if err := os.WriteFile(srtPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create subtitle file %s: %w", srtPath, err)
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
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=24", opts.DurationSec),
	}

	// Add audio inputs
	for range opts.AudioLangs {
		args = append(args, "-f", "lavfi", "-i",
			fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%d", opts.DurationSec))
	}

	// Add subtitle inputs
	for _, f := range subFiles {
		args = append(args, "-i", f)
	}

	// Map video
	args = append(args, "-map", "0:v")

	// Map audio streams
	for i := range opts.AudioLangs {
		args = append(args, "-map", fmt.Sprintf("%d:a", i+1))
	}

	// Map subtitle streams
	for i := range opts.SubLangs {
		args = append(args, "-map", fmt.Sprintf("%d", i+1+len(opts.AudioLangs)))
	}

	// Codecs - use fast settings
	args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-c:s", "srt")

	// Audio metadata
	for i, lang := range opts.AudioLangs {
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", lang))
	}

	// Subtitle metadata
	for i, lang := range opts.SubLangs {
		args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("language=%s", lang))
	}

	args = append(args, "-shortest", "-y", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, output)
	}

	return nil
}
