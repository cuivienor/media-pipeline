package organize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// ValidationResult holds the result of validating an organization directory
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// Validator validates that media has been organized correctly
type Validator struct{}

// ValidateMovie validates that a movie directory is properly organized
func (v *Validator) ValidateMovie(outputDir string) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check root is empty (except _ dirs and .rip)
	if errs := v.checkRootEmpty(outputDir); len(errs) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, errs...)
	}

	// Check _main exists and has files
	mainDir := filepath.Join(outputDir, "_main")
	if _, err := os.Stat(mainDir); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "_main directory not found")
	} else {
		files, _ := filepath.Glob(filepath.Join(mainDir, "*.mkv"))
		if len(files) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, "_main has no .mkv files")
		}
	}

	return result
}

// ValidateTV validates that a TV season directory is properly organized
// For single-disc seasons, validates the season directory directly
// This is the legacy behavior - prefer ValidateTVDisc for multi-disc seasons
func (v *Validator) ValidateTV(outputDir string) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check root is empty
	if errs := v.checkRootEmpty(outputDir); len(errs) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, errs...)
	}

	// Check _episodes exists
	episodesDir := filepath.Join(outputDir, "_episodes")
	if _, err := os.Stat(episodesDir); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "_episodes directory not found")
		return result
	}

	// Check episode naming and sequence
	files, _ := filepath.Glob(filepath.Join(episodesDir, "*.mkv"))
	episodes := v.parseEpisodeNumbers(files)

	if len(episodes) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "_episodes has no valid episode files")
		return result
	}

	// Check for gaps
	if gaps := v.findGaps(episodes); len(gaps) > 0 {
		result.Valid = false
		for _, gap := range gaps {
			result.Errors = append(result.Errors, fmt.Sprintf("missing episode %d", gap))
		}
	}

	return result
}

// ValidateTVDisc validates a single disc directory within a TV season
// Each disc should have _episodes/ with properly named files
func (v *Validator) ValidateTVDisc(discDir string) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check root is empty (except _ dirs and .rip)
	if errs := v.checkRootEmpty(discDir); len(errs) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, errs...)
	}

	// Check _episodes exists
	episodesDir := filepath.Join(discDir, "_episodes")
	if _, err := os.Stat(episodesDir); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "_episodes directory not found")
		return result
	}

	// Check episode naming
	files, _ := filepath.Glob(filepath.Join(episodesDir, "*.mkv"))
	episodes := v.parseEpisodeNumbers(files)

	if len(episodes) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "_episodes has no valid episode files")
		return result
	}

	// Note: We don't check for gaps within a single disc since episodes may span discs

	return result
}

// ValidateTVSeason validates a multi-disc TV season by checking each disc
func (v *Validator) ValidateTVSeason(discPaths []string) ValidationResult {
	result := ValidationResult{Valid: true}

	if len(discPaths) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no disc paths provided")
		return result
	}

	// Validate each disc and collect all episodes
	allEpisodes := make(map[int]bool)

	for _, discPath := range discPaths {
		discName := filepath.Base(discPath)
		discResult := v.ValidateTVDisc(discPath)

		if !discResult.Valid {
			result.Valid = false
			for _, err := range discResult.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", discName, err))
			}
		}

		// Collect episode numbers from this disc
		episodesDir := filepath.Join(discPath, "_episodes")
		files, _ := filepath.Glob(filepath.Join(episodesDir, "*.mkv"))
		for _, ep := range v.parseEpisodeNumbers(files) {
			allEpisodes[ep] = true
		}
	}

	// Check for duplicate episodes across discs (warning, not error)
	// Check for gaps in the combined episode list
	if len(allEpisodes) > 0 {
		var episodes []int
		for ep := range allEpisodes {
			episodes = append(episodes, ep)
		}
		sort.Ints(episodes)

		if gaps := v.findGaps(episodes); len(gaps) > 0 {
			for _, gap := range gaps {
				result.Warnings = append(result.Warnings, fmt.Sprintf("missing episode %d across all discs", gap))
			}
		}
	}

	return result
}

// checkRootEmpty verifies the root directory only contains underscore-prefixed directories and .rip state
func (v *Validator) checkRootEmpty(dir string) []string {
	var errors []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to read directory: %v", err))
		return errors
	}

	for _, entry := range entries {
		name := entry.Name()
		// Allow _ prefixed dirs and .rip state dir
		if len(name) > 0 && name[0] != '_' && name != ".rip" {
			errors = append(errors, fmt.Sprintf("root directory not empty: found %s", name))
		}
	}
	return errors
}

// episodePattern matches episode filenames like "01.mkv", "01-02.mkv", "01_Episode_Name.mkv"
var episodePattern = regexp.MustCompile(`^(\d+)(?:-(\d+))?(?:_.*)?\.mkv$`)

// parseEpisodeNumbers extracts episode numbers from filenames
// For multi-episode files like "01-02.mkv", it extracts both 1 and 2
func (v *Validator) parseEpisodeNumbers(files []string) []int {
	seen := make(map[int]bool)

	for _, file := range files {
		base := filepath.Base(file)
		if matches := episodePattern.FindStringSubmatch(base); matches != nil {
			// Parse first episode number
			if num, err := strconv.Atoi(matches[1]); err == nil {
				seen[num] = true
			}

			// Parse second episode number if this is a multi-episode file (e.g., "01-02.mkv")
			if matches[2] != "" {
				if num, err := strconv.Atoi(matches[2]); err == nil {
					// Add all episodes in the range
					start, _ := strconv.Atoi(matches[1])
					end := num
					for i := start; i <= end; i++ {
						seen[i] = true
					}
				}
			}
		}
	}

	// Convert map to sorted slice
	var episodes []int
	for ep := range seen {
		episodes = append(episodes, ep)
	}
	sort.Ints(episodes)
	return episodes
}

// findGaps finds missing episode numbers in the sequence
func (v *Validator) findGaps(episodes []int) []int {
	if len(episodes) == 0 {
		return nil
	}

	var gaps []int
	for i := episodes[0]; i < episodes[len(episodes)-1]; i++ {
		found := false
		for _, ep := range episodes {
			if ep == i {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, i)
		}
	}
	return gaps
}
