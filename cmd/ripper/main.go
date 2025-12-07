package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/ripper"
)

const defaultMediaBase = "/mnt/media"

// Options holds parsed command-line options
type Options struct {
	Type     ripper.MediaType
	Name     string
	Season   int
	Disc     int
	DiscPath string
	DBPath   string // Path to SQLite database (optional)
	JobID    int64  // Job ID for TUI dispatch mode (optional)
}

// Mode represents the execution mode
type Mode int

const (
	ModeStandaloneNoDB   Mode = iota // Standalone mode without DB tracking
	ModeStandaloneWithDB             // Standalone mode with DB tracking
	ModeJobDispatch                  // TUI dispatch mode (load job from DB)
)

// Config holds runtime configuration
type Config struct {
	MediaBase      string
	MakeMKVConPath string
}

func main() {
	opts, err := ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Usage: ripper -t <movie|tv> -n <name> [-s <season>] [-d <disc>] [--disc-path <path>] [-db <path>]\n")
		os.Exit(1)
	}

	// Build configuration from environment
	env := getEnvMap()
	config := BuildConfig(opts, env)

	// Build state manager (with or without DB)
	stateManager, database, err := BuildStateManager(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if database != nil {
		defer database.Close()
	}

	// Create ripper
	stagingBase := filepath.Join(config.MediaBase, "staging")
	runner := ripper.NewMakeMKVRunner(config.MakeMKVConPath)
	r := ripper.NewRipper(stagingBase, runner, stateManager)

	// Build request
	var req *ripper.RipRequest
	if opts.JobID > 0 {
		// Job-id mode: load request from database
		req, err = LoadRipRequestFromJob(database, opts.JobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// Override disc path if specified
		if opts.DiscPath != "" {
			req.DiscPath = opts.DiscPath
		}
	} else {
		// Standalone mode: build request from opts
		req = BuildRipRequest(opts)
	}

	// Run the rip
	result, err := r.Rip(context.Background(), req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Rip completed successfully!\n")
	fmt.Printf("Output: %s\n", result.OutputDir)
	fmt.Printf("Duration: %s\n", result.Duration())
}

// BuildStateManager creates the appropriate state manager based on options
func BuildStateManager(opts *Options) (ripper.StateManager, *db.DB, error) {
	if opts.DBPath == "" {
		// No DB - use filesystem-only state manager
		return ripper.NewStateManager(), nil, nil
	}

	// Open database
	database, err := db.Open(opts.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create dual-write state manager
	repo := db.NewSQLiteRepository(database)
	fsManager := ripper.NewStateManager()
	dualManager := ripper.NewDualWriteStateManager(fsManager, repo)

	// If we're in job-id mode, set the job ID on the state manager
	if opts.JobID > 0 {
		dualManager.WithJobID(opts.JobID)
	}

	return dualManager, database, nil
}

// ParseArgs parses command-line arguments
func ParseArgs(args []string) (*Options, error) {
	fs := flag.NewFlagSet("ripper", flag.ContinueOnError)

	var typeStr string
	var name string
	var season, disc int
	var discPath string
	var dbPath string
	var jobID int64

	fs.StringVar(&typeStr, "t", "", "Media type: movie or tv/show")
	fs.StringVar(&typeStr, "type", "", "Media type: movie or tv/show")
	fs.StringVar(&name, "n", "", "Media name")
	fs.StringVar(&name, "name", "", "Media name")
	fs.IntVar(&season, "s", 0, "Season number (TV only)")
	fs.IntVar(&season, "season", 0, "Season number (TV only)")
	fs.IntVar(&disc, "d", 0, "Disc number (TV only)")
	fs.IntVar(&disc, "disc", 0, "Disc number (TV only)")
	fs.StringVar(&discPath, "disc-path", "disc:0", "Path to disc device")
	fs.StringVar(&dbPath, "db", "", "Path to SQLite database")
	fs.Int64Var(&jobID, "job-id", 0, "Job ID to resume (TUI dispatch mode)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Validate job-id mode
	if jobID > 0 {
		if dbPath == "" {
			return nil, errors.New("-db is required when using -job-id")
		}
		// In job-id mode, other fields are loaded from the job
		return &Options{
			JobID:    jobID,
			DBPath:   dbPath,
			DiscPath: discPath,
		}, nil
	}

	// Validate standalone mode required fields
	if typeStr == "" {
		return nil, errors.New("type (-t) is required")
	}
	if name == "" {
		return nil, errors.New("name (-n) is required")
	}

	// Parse type
	var mediaType ripper.MediaType
	switch strings.ToLower(typeStr) {
	case "movie":
		mediaType = ripper.MediaTypeMovie
	case "tv", "show":
		mediaType = ripper.MediaTypeTV
	default:
		return nil, fmt.Errorf("invalid type %q: must be movie, tv, or show", typeStr)
	}

	// Validate TV-specific requirements
	if mediaType == ripper.MediaTypeTV {
		if season <= 0 {
			return nil, errors.New("season (-s) is required for TV shows")
		}
		if disc <= 0 {
			return nil, errors.New("disc (-d) is required for TV shows")
		}
	}

	return &Options{
		Type:     mediaType,
		Name:     name,
		Season:   season,
		Disc:     disc,
		DiscPath: discPath,
		DBPath:   dbPath,
		JobID:    jobID,
	}, nil
}

// DetermineMode returns the execution mode based on options
func DetermineMode(opts *Options) Mode {
	if opts.JobID > 0 {
		return ModeJobDispatch
	}
	if opts.DBPath != "" {
		return ModeStandaloneWithDB
	}
	return ModeStandaloneNoDB
}

// BuildConfig creates runtime configuration from options and environment
func BuildConfig(opts *Options, env map[string]string) *Config {
	config := &Config{
		MediaBase: defaultMediaBase,
	}

	if env != nil {
		if val, ok := env["MEDIA_BASE"]; ok && val != "" {
			config.MediaBase = val
		}
		if val, ok := env["MAKEMKVCON_PATH"]; ok && val != "" {
			config.MakeMKVConPath = val
		}
	}

	return config
}

// BuildRipRequest creates a RipRequest from options
func BuildRipRequest(opts *Options) *ripper.RipRequest {
	return &ripper.RipRequest{
		Type:     opts.Type,
		Name:     opts.Name,
		Season:   opts.Season,
		Disc:     opts.Disc,
		DiscPath: opts.DiscPath,
	}
}

// LoadRipRequestFromJob loads a RipRequest from an existing database job
func LoadRipRequestFromJob(database *db.DB, jobID int64) (*ripper.RipRequest, error) {
	ctx := context.Background()
	repo := db.NewSQLiteRepository(database)

	// Get the job
	job, err := repo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("job %d not found", jobID)
	}

	// Get the media item
	item, err := repo.GetMediaItem(ctx, job.MediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("media item %d not found", job.MediaItemID)
	}

	// Build RipRequest from job and media item
	req := &ripper.RipRequest{
		Name:     item.Name,
		DiscPath: "disc:0", // Default disc path
	}

	// Set type
	switch item.Type {
	case "movie":
		req.Type = ripper.MediaTypeMovie
	case "tv":
		req.Type = ripper.MediaTypeTV
	default:
		return nil, fmt.Errorf("unknown media type: %s", item.Type)
	}

	// Set TV-specific fields
	if req.Type == ripper.MediaTypeTV {
		if item.Season == nil {
			return nil, fmt.Errorf("TV show missing season number")
		}
		req.Season = *item.Season

		if job.Disc == nil {
			return nil, fmt.Errorf("TV show job missing disc number")
		}
		req.Disc = *job.Disc
	}

	return req, nil
}

// getEnvMap returns environment variables as a map
func getEnvMap() map[string]string {
	return map[string]string{
		"MEDIA_BASE":      os.Getenv("MEDIA_BASE"),
		"MAKEMKVCON_PATH": os.Getenv("MAKEMKVCON_PATH"),
	}
}
