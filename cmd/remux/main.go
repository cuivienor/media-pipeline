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
		fmt.Fprintln(os.Stderr, "Usage: remux -job-id <id> -db <path>")
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	repo := db.NewSQLiteRepository(database)
	ctx := context.Background()

	// Update job to in_progress
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusInProgress, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating job status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Remux stub: simulating work...")
	time.Sleep(2 * time.Second)

	// Mark complete
	if err := repo.UpdateJobStatus(ctx, jobID, model.JobStatusCompleted, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating job status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Remux stub: complete")
}
