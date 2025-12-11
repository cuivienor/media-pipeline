package transcode

import (
	"os/exec"
	"testing"
)

func TestGetDuration_Integration(t *testing.T) {
	// Skip if ffprobe not available
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	// This test requires a real MKV file
	// In practice, use a test fixture or skip in CI
	t.Skip("requires test fixture")
}

func TestCheckHardwareSupport(t *testing.T) {
	// Skip if ffmpeg not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	err := CheckHardwareSupport()
	// Just log the result - don't fail if QSV not available
	if err != nil {
		t.Logf("QSV not available: %v", err)
	} else {
		t.Log("QSV is available")
	}
}
