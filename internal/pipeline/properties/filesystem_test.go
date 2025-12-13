package properties

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssertOutputNotLargerThanInput(t *testing.T) {
	tmpDir := t.TempDir()

	inputDir := filepath.Join(tmpDir, "input")
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Create input file (100 bytes)
	inputFile := filepath.Join(inputDir, "test.mkv")
	os.WriteFile(inputFile, make([]byte, 100), 0644)

	// Create output file smaller than input (good case)
	outputFile := filepath.Join(outputDir, "test.mkv")
	os.WriteFile(outputFile, make([]byte, 50), 0644)

	// Should pass
	err := AssertOutputNotLargerThanInput(inputDir, outputDir, 1.5)
	if err != nil {
		t.Errorf("unexpected error for smaller output: %v", err)
	}

	// Create output file larger than allowed ratio (bad case)
	os.WriteFile(outputFile, make([]byte, 200), 0644)

	err = AssertOutputNotLargerThanInput(inputDir, outputDir, 1.5)
	if err == nil {
		t.Error("expected error for larger output")
	}
}
