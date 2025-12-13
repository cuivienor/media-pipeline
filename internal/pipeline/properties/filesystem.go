package properties

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssertOutputNotLargerThanInput verifies output files don't exceed input size by more than maxRatio
// maxRatio of 1.5 means output can be at most 150% of input size
func AssertOutputNotLargerThanInput(inputDir, outputDir string, maxRatio float64) error {
	// Build map of input file sizes
	inputSizes := make(map[string]int64)
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".mkv") {
			return nil
		}
		relPath, _ := filepath.Rel(inputDir, path)
		inputSizes[relPath] = info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk input dir: %w", err)
	}

	// Check output files
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".mkv") {
			return nil
		}

		relPath, _ := filepath.Rel(outputDir, path)
		inputSize, ok := inputSizes[relPath]
		if !ok {
			// New file in output not in input - skip
			return nil
		}

		outputSize := info.Size()
		if inputSize > 0 {
			ratio := float64(outputSize) / float64(inputSize)
			if ratio > maxRatio {
				return fmt.Errorf("file %s: output size %d is %.1fx input size %d (max allowed: %.1fx)",
					relPath, outputSize, ratio, inputSize, maxRatio)
			}
		}
		return nil
	})

	return err
}
