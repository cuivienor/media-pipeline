package ripper

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Common extras categories for organization
var extrasCategories = []string{
	"behind the scenes",
	"deleted scenes",
	"featurettes",
	"interviews",
	"scenes",
	"shorts",
	"trailers",
	"other",
}

// CreateOrganizationScaffolding creates the directory structure for manual review
// after ripping. This includes _discarded, _extras/{categories}, and type-specific
// directories (_main for movies, _episodes for TV shows).
func CreateOrganizationScaffolding(outputDir string, req *RipRequest) error {
	// Create _discarded directory
	if err := os.MkdirAll(filepath.Join(outputDir, "_discarded"), 0755); err != nil {
		return fmt.Errorf("failed to create _discarded: %w", err)
	}

	// Create _extras subdirectories
	for _, category := range extrasCategories {
		path := filepath.Join(outputDir, "_extras", category)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create _extras/%s: %w", category, err)
		}
	}

	// Create type-specific directories
	switch req.Type {
	case MediaTypeMovie:
		if err := os.MkdirAll(filepath.Join(outputDir, "_main"), 0755); err != nil {
			return fmt.Errorf("failed to create _main: %w", err)
		}
	case MediaTypeTV:
		if err := os.MkdirAll(filepath.Join(outputDir, "_episodes"), 0755); err != nil {
			return fmt.Errorf("failed to create _episodes: %w", err)
		}
	}

	// Create _REVIEW.txt
	if err := createReviewFile(outputDir, req); err != nil {
		return fmt.Errorf("failed to create _REVIEW.txt: %w", err)
	}

	return nil
}

// createReviewFile creates the _REVIEW.txt template file
func createReviewFile(outputDir string, req *RipRequest) error {
	reviewPath := filepath.Join(outputDir, "_REVIEW.txt")
	timestamp := time.Now().Format("2006-01-02 15:04")

	var content string
	switch req.Type {
	case MediaTypeMovie:
		content = fmt.Sprintf(`# Manual Review Notes
# Movie: %s
# Ripped: %s

## Disc Info
- Blu-ray.com URL:
- Total titles ripped:

## Main Feature
# Identify the main movie file and move to _main/
# Rename to: %s.mkv

## Extras Found
# Move to appropriate _extras/ subdirectory with descriptive names
# Example: Making_Of.mkv → _extras/behind the scenes/

## Discarded
# Move duplicates/unwanted to _discarded/

## Notes

`, req.Name, timestamp, req.SafeName())

	case MediaTypeTV:
		content = fmt.Sprintf(`# Manual Review Notes
# Show: %s
# Season: %d
# Disc: %d
# Ripped: %s

## Disc Info
- Blu-ray.com URL:
- Total titles ripped:

## Episode Mapping
# Rename files to: 01.mkv, 02.mkv, etc. (or 01_Episode_Name.mkv)
# Then move to _episodes/

## Extras Found
# Move to appropriate _extras/ subdirectory with descriptive names
# Example: Making_Of_Season_%d.mkv → _extras/behind the scenes/

## Discarded
# Move duplicates/unwanted to _discarded/

## Notes

`, req.Name, req.Season, req.Disc, timestamp, req.Season)

	default:
		content = fmt.Sprintf(`# Manual Review Notes
# Name: %s
# Ripped: %s

## Notes

`, req.Name, timestamp)
	}

	return os.WriteFile(reviewPath, []byte(content), 0644)
}
