package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cuivienor/media-pipeline/internal/db"
	"github.com/cuivienor/media-pipeline/internal/model"
)

func main() {
	var jobID int64
	var dbPath string

	flag.Int64Var(&jobID, "job-id", 0, "Job ID to execute")
	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Parse()

	if jobID == 0 || dbPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: publish -job-id <id> -db <path>")
		os.Exit(1)
	}

	if err := run(jobID, dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(jobID int64, dbPath string) error {
	ctx := context.Background()

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)

	// Helper to mark job as failed
	markFailed := func(errMsg string) {
		if updateErr := repo.UpdateJobStatus(ctx, jobID, model.JobStatusFailed, errMsg); updateErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to update job status: %v\n", updateErr)
		}
	}

	// Update job to in_progress
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusInProgress, ""); err != nil {
		markFailed(err.Error())
		return fmt.Errorf("failed to update job status: %w", err)
	}

	fmt.Println("Publish stub: simulating work...")
	time.Sleep(2 * time.Second)

	// TODO: Implement actual publish logic here
	// When implementing, wrap errors with markFailed() like other commands

	// Mark complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	fmt.Println("Publish stub: complete")
	return nil
}
