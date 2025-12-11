package transcode

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetDuration returns the duration of a media file in seconds
func GetDuration(inputPath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return 0, fmt.Errorf("ffprobe failed: %s", string(exitErr.Stderr))
		}
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	if durationStr == "" || durationStr == "N/A" {
		return 0, fmt.Errorf("could not determine duration")
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration %q: %w", durationStr, err)
	}

	return duration, nil
}

// CheckHardwareSupport checks if Intel QSV is available
func CheckHardwareSupport() error {
	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-init_hw_device", "qsv=hw",
		"-f", "lavfi",
		"-i", "nullsrc=s=256x256:d=1",
		"-vf", "hwupload=extra_hw_frames=64,format=qsv",
		"-c:v", "hevc_qsv",
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("QSV not available: %s", strings.TrimSpace(string(output)))
	}

	return nil
}
