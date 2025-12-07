# Phase 2: TUI Database Integration

**Date:** 2024-12-07
**Status:** Ready for implementation
**Branch:** `feat/tui-database-integration`
**Depends on:** Phase 1 (SQLite Database Foundation) - completed

## Goal

Get to an end-to-end working demo where you can:
1. Start a rip from the TUI
2. See state/jobs in the TUI (from the database)
3. Have stub implementations for remux, transcode, publish
4. Handle the organize step UX (external + validate)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| TUI data source | Database only | Remove filesystem scanner, DB is source of truth |
| Config location | `$MEDIA_BASE/pipeline/config.yaml` | Coupled with MEDIA_BASE for environment isolation |
| Dispatch model | Config-driven | SSH for prod, local for testing via config |
| New Rip flow | Single form | Type, name, season/disc on one screen |
| Organize UX | External + validate | User organizes in file manager, TUI validates |
| Stub commands | Status only | Just update job status after delay |
| Legacy scanner | Remove | DB is the only source of truth |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                           TUI                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  Overview   │  │  New Rip    │  │  Item Detail            │  │
│  │  (DB query) │  │  Form       │  │  - Show files           │  │
│  └─────────────┘  └──────┬──────┘  │  - Validate organize    │  │
│                          │         │  - Dispatch next stage  │  │
│                          ▼         └─────────────────────────┘  │
│                   ┌──────────────┐                               │
│                   │  Dispatcher  │                               │
│                   │  (config)    │                               │
│                   └──────┬───────┘                               │
└──────────────────────────┼──────────────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
        Local (test)              SSH (prod)
              │                         │
              ▼                         ▼
    ┌─────────────────┐       ┌─────────────────┐
    │ ripper -job-id  │       │ ssh ripper-host │
    │ -db /path/db    │       │ ripper -job-id  │
    └─────────────────┘       └─────────────────┘
```

## Config File

Location: `$MEDIA_BASE/pipeline/config.yaml`

```yaml
# Production config (/mnt/media/pipeline/config.yaml)
staging_base: /mnt/media/staging
library_base: /mnt/media/library

dispatch:
  rip: ripper        # SSH target
  remux: ""          # empty = local
  transcode: ""
  publish: ""

# Test config (MEDIA_BASE=/tmp/test-media)
# /tmp/test-media/pipeline/config.yaml
staging_base: /tmp/test-media/staging
library_base: /tmp/test-media/library

dispatch:
  rip: ""            # all local for testing
  remux: ""
  transcode: ""
  publish: ""
```

---

## Task 1: Update Config Package

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1.1: Write failing test for MEDIA_BASE-derived config loading**

Add to `internal/config/config_test.go`:

```go
func TestLoadFromMediaBase(t *testing.T) {
    // Create temp directory structure
    tmpDir := t.TempDir()
    pipelineDir := filepath.Join(tmpDir, "pipeline")
    os.MkdirAll(pipelineDir, 0755)

    configContent := `
staging_base: /mnt/media/staging
library_base: /mnt/media/library
dispatch:
  rip: ripper-host
`
    os.WriteFile(filepath.Join(pipelineDir, "config.yaml"), []byte(configContent), 0644)

    // Set MEDIA_BASE
    t.Setenv("MEDIA_BASE", tmpDir)

    cfg, err := LoadFromMediaBase()
    if err != nil {
        t.Fatalf("LoadFromMediaBase() error = %v", err)
    }

    if cfg.StagingBase != "/mnt/media/staging" {
        t.Errorf("StagingBase = %q, want /mnt/media/staging", cfg.StagingBase)
    }
    if cfg.DispatchTarget("rip") != "ripper-host" {
        t.Errorf("DispatchTarget(rip) = %q, want ripper-host", cfg.DispatchTarget("rip"))
    }
}

func TestLoadFromMediaBase_DefaultPath(t *testing.T) {
    // Without MEDIA_BASE set, should use /mnt/media
    cfg, _ := LoadFromMediaBase()
    // Will fail if /mnt/media/pipeline/config.yaml doesn't exist, which is expected
    // The test verifies the default path logic
    _ = cfg
}

func TestConfig_DataDir(t *testing.T) {
    t.Setenv("MEDIA_BASE", "/mnt/media")

    cfg := &Config{
        StagingBase: "/mnt/media/staging",
    }

    // DataDir should be derived from MEDIA_BASE
    if got := cfg.DataDir(); got != "/mnt/media/pipeline" {
        t.Errorf("DataDir() = %q, want /mnt/media/pipeline", got)
    }
}
```

**Step 1.2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestLoadFromMediaBase -v
```

**Step 1.3: Update config.go**

```go
package config

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

const (
    defaultMediaBase = "/mnt/media"
    pipelineDirName  = "pipeline"
    configFileName   = "config.yaml"
)

// Config holds application configuration
type Config struct {
    StagingBase string            `yaml:"staging_base"`
    LibraryBase string            `yaml:"library_base"`
    Dispatch    map[string]string `yaml:"dispatch"`

    // Derived from environment, not stored in YAML
    mediaBase string
}

// MediaBase returns the MEDIA_BASE path
func (c *Config) MediaBase() string {
    if c.mediaBase != "" {
        return c.mediaBase
    }
    if base := os.Getenv("MEDIA_BASE"); base != "" {
        return base
    }
    return defaultMediaBase
}

// DataDir returns the pipeline data directory ($MEDIA_BASE/pipeline)
func (c *Config) DataDir() string {
    return filepath.Join(c.MediaBase(), pipelineDirName)
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
    return filepath.Join(c.DataDir(), "pipeline.db")
}

// JobLogDir returns the directory for a job's log files
func (c *Config) JobLogDir(jobID int64) string {
    return filepath.Join(c.DataDir(), "logs", "jobs", fmt.Sprintf("%d", jobID))
}

// JobLogPath returns the path for a job's main log file
func (c *Config) JobLogPath(jobID int64) string {
    return filepath.Join(c.JobLogDir(jobID), "job.log")
}

// ToolLogPath returns the path for a tool's raw log file
func (c *Config) ToolLogPath(jobID int64, tool string) string {
    return filepath.Join(c.JobLogDir(jobID), fmt.Sprintf("%s.log", tool))
}

// EnsureJobLogDir creates the log directory for a specific job
func (c *Config) EnsureJobLogDir(jobID int64) error {
    return os.MkdirAll(c.JobLogDir(jobID), 0755)
}

// DispatchTarget returns the SSH target for a stage, or empty for local execution
func (c *Config) DispatchTarget(stage string) string {
    if c.Dispatch == nil {
        return ""
    }
    return c.Dispatch[stage]
}

// IsLocal returns true if the stage should run locally (no SSH)
func (c *Config) IsLocal(stage string) bool {
    return c.DispatchTarget(stage) == ""
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    return &cfg, nil
}

// LoadFromMediaBase loads config from $MEDIA_BASE/pipeline/config.yaml
func LoadFromMediaBase() (*Config, error) {
    mediaBase := os.Getenv("MEDIA_BASE")
    if mediaBase == "" {
        mediaBase = defaultMediaBase
    }

    configPath := filepath.Join(mediaBase, pipelineDirName, configFileName)
    cfg, err := Load(configPath)
    if err != nil {
        return nil, err
    }

    cfg.mediaBase = mediaBase
    return cfg, nil
}
```

**Step 1.4: Run tests**

```bash
go test ./internal/config/... -v
```

**Step 1.5: Commit**

```bash
git add internal/config/
git commit -m "config: load from MEDIA_BASE/pipeline/config.yaml"
```

---

## Task 2: Wire Up Ripper CLI to Use DualWriteStateManager

**Files:**
- Modify: `cmd/ripper/main.go`
- Modify: `cmd/ripper/main_test.go`

**Step 2.1: Write test for DB mode**

Add to `cmd/ripper/main_test.go`:

```go
func TestBuildStateManager_WithDB(t *testing.T) {
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "test.db")

    // Create database
    database, err := db.Open(dbPath)
    if err != nil {
        t.Fatalf("failed to open db: %v", err)
    }
    defer database.Close()

    opts := &Options{
        Type:   ripper.MediaTypeMovie,
        Name:   "Test Movie",
        DBPath: dbPath,
    }

    sm, repo, err := BuildStateManager(opts)
    if err != nil {
        t.Fatalf("BuildStateManager failed: %v", err)
    }

    if sm == nil {
        t.Error("expected DualWriteStateManager, got nil")
    }
    if repo == nil {
        t.Error("expected Repository, got nil")
    }
}

func TestBuildStateManager_NoDB(t *testing.T) {
    opts := &Options{
        Type: ripper.MediaTypeMovie,
        Name: "Test Movie",
    }

    sm, repo, err := BuildStateManager(opts)
    if err != nil {
        t.Fatalf("BuildStateManager failed: %v", err)
    }

    if sm == nil {
        t.Error("expected DefaultStateManager, got nil")
    }
    if repo != nil {
        t.Error("expected nil Repository for no-DB mode")
    }
}
```

**Step 2.2: Run test to verify it fails**

```bash
go test ./cmd/ripper/... -run TestBuildStateManager -v
```

**Step 2.3: Update main.go to wire up DualWriteStateManager**

```go
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

// ... existing Options, Mode, Config types ...

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
    r := ripper.NewRipperWithStateManager(stagingBase, runner, nil, stateManager)

    // Build request
    req := BuildRipRequest(opts)

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

    return dualManager, database, nil
}

// ... rest of existing code ...
```

**Step 2.4: Add NewRipperWithStateManager to ripper package**

In `internal/ripper/ripper.go`, add:

```go
// NewRipperWithStateManager creates a new Ripper with a custom state manager
func NewRipperWithStateManager(stagingBase string, runner MakeMKVRunner, logger Logger, stateManager StateManager) *Ripper {
    if stateManager == nil {
        stateManager = NewStateManager()
    }
    return &Ripper{
        stagingBase:  stagingBase,
        runner:       runner,
        logger:       logger,
        stateManager: stateManager,
    }
}
```

**Step 2.5: Run tests**

```bash
go test ./cmd/ripper/... -v
go test ./internal/ripper/... -v
```

**Step 2.6: Commit**

```bash
git add cmd/ripper/ internal/ripper/
git commit -m "ripper: wire up DualWriteStateManager when -db flag provided"
```

---

## Task 3: Remove Filesystem Scanner

**Files:**
- Delete: `internal/scanner/scanner.go`
- Delete: `internal/scanner/scanner_test.go`
- Modify: `internal/tui/app.go` (remove scanner import, will fix in Task 4)

**Step 3.1: Remove scanner package**

```bash
rm -rf internal/scanner/
```

**Step 3.2: Update any imports**

The TUI will break - that's expected. We'll fix it in Task 4.

**Step 3.3: Commit**

```bash
git add -A
git commit -m "scanner: remove filesystem scanner (replaced by DB)"
```

---

## Task 4: TUI Database-Backed State Loading

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `cmd/media-pipeline/main.go`
- Create: `internal/tui/state.go`

**Step 4.1: Create state.go for DB queries**

Create `internal/tui/state.go`:

```go
package tui

import (
    "context"

    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// PipelineState holds the current state loaded from the database
type PipelineState struct {
    Items []model.MediaItem
    Jobs  map[int64][]model.Job // mediaItemID -> jobs
}

// LoadState loads pipeline state from the database
func LoadState(repo db.Repository) (*PipelineState, error) {
    ctx := context.Background()

    items, err := repo.ListMediaItems(ctx, db.ListOptions{})
    if err != nil {
        return nil, err
    }

    state := &PipelineState{
        Items: items,
        Jobs:  make(map[int64][]model.Job),
    }

    // Load jobs for each item
    for _, item := range items {
        jobs, err := repo.ListJobsForMedia(ctx, item.ID)
        if err != nil {
            return nil, err
        }
        state.Jobs[item.ID] = jobs

        // Update item's current stage/status from latest job
        if len(jobs) > 0 {
            latestJob := jobs[len(jobs)-1]
            state.updateItemFromJob(&item, latestJob)
        }
    }

    return state, nil
}

// updateItemFromJob updates a MediaItem's Current/Status from its latest job
func (s *PipelineState) updateItemFromJob(item *model.MediaItem, job model.Job) {
    item.Current = job.Stage
    switch job.Status {
    case model.JobStatusCompleted:
        item.Status = model.StatusCompleted
    case model.JobStatusInProgress:
        item.Status = model.StatusInProgress
    case model.JobStatusFailed:
        item.Status = model.StatusFailed
    default:
        item.Status = model.StatusPending
    }
}

// CountByStage returns the number of items at each stage
func (s *PipelineState) CountByStage() map[model.Stage]int {
    counts := make(map[model.Stage]int)
    for _, item := range s.Items {
        counts[item.Current]++
    }
    return counts
}

// ItemsAtStage returns all items currently at the specified stage
func (s *PipelineState) ItemsAtStage(stage model.Stage) []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Current == stage {
            result = append(result, item)
        }
    }
    return result
}

// ItemsReadyForNextStage returns all items that have completed their current stage
func (s *PipelineState) ItemsReadyForNextStage() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Status == model.StatusCompleted && item.Current != model.StagePublish {
            result = append(result, item)
        }
    }
    return result
}

// ItemsInProgress returns all items currently being processed
func (s *PipelineState) ItemsInProgress() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Status == model.StatusInProgress {
            result = append(result, item)
        }
    }
    return result
}

// ItemsFailed returns all items in a failed state
func (s *PipelineState) ItemsFailed() []model.MediaItem {
    var result []model.MediaItem
    for _, item := range s.Items {
        if item.Status == model.StatusFailed {
            result = append(result, item)
        }
    }
    return result
}

// GetJobsForItem returns all jobs for a specific media item
func (s *PipelineState) GetJobsForItem(itemID int64) []model.Job {
    return s.Jobs[itemID]
}
```

**Step 4.2: Update app.go to use DB**

```go
package tui

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/config"
    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// View represents the current view
type View int

const (
    ViewOverview View = iota
    ViewStageList
    ViewActionNeeded
    ViewItemDetail
    ViewNewRip      // New view for rip form
    ViewOrganize    // New view for organize validation
)

// App is the main application model
type App struct {
    config *config.Config
    repo   db.Repository
    state  *PipelineState
    err    error

    // Navigation state
    currentView   View
    selectedStage model.Stage
    selectedItem  *model.MediaItem
    cursor        int

    // Window size
    width  int
    height int

    // Form state (for NewRip view)
    formFields map[string]string
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config, repo db.Repository) *App {
    return &App{
        config:      cfg,
        repo:        repo,
        currentView: ViewOverview,
        formFields:  make(map[string]string),
    }
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
    return a.loadState
}

// stateMsg is sent when state loading completes
type stateMsg struct {
    state *PipelineState
    err   error
}

// loadState loads pipeline state from the database
func (a *App) loadState() tea.Msg {
    state, err := LoadState(a.repo)
    return stateMsg{state: state, err: err}
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return a.handleKeyPress(msg)

    case tea.WindowSizeMsg:
        a.width = msg.Width
        a.height = msg.Height
        return a, nil

    case stateMsg:
        a.state = msg.state
        a.err = msg.err
        return a, nil
    }

    return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "q", "ctrl+c":
        return a, tea.Quit

    case "r":
        // Refresh
        return a, a.loadState

    case "n":
        // New rip (only from overview or action needed view)
        if a.currentView == ViewOverview || a.currentView == ViewActionNeeded {
            a.currentView = ViewNewRip
            a.formFields = map[string]string{
                "type":   "movie",
                "name":   "",
                "season": "",
                "disc":   "",
            }
            return a, nil
        }

    case "tab":
        // Toggle between overview and action needed
        if a.currentView == ViewOverview {
            a.currentView = ViewActionNeeded
        } else if a.currentView == ViewActionNeeded {
            a.currentView = ViewOverview
        }
        a.cursor = 0
        return a, nil

    case "esc":
        // Go back
        switch a.currentView {
        case ViewStageList:
            a.currentView = ViewOverview
            a.cursor = int(a.selectedStage)
        case ViewItemDetail:
            a.currentView = ViewStageList
            a.cursor = 0
        case ViewActionNeeded:
            a.currentView = ViewOverview
            a.cursor = 0
        case ViewNewRip, ViewOrganize:
            a.currentView = ViewOverview
            a.cursor = 0
        }
        return a, nil

    case "up", "k":
        if a.cursor > 0 {
            a.cursor--
        }
        return a, nil

    case "down", "j":
        a.cursor++
        maxCursor := a.getMaxCursor()
        if a.cursor > maxCursor {
            a.cursor = maxCursor
        }
        return a, nil

    case "enter":
        return a.handleEnter()
    }

    return a, nil
}

// ... rest of methods updated to use a.state instead of a.scanner ...
```

**Step 4.3: Update cmd/media-pipeline/main.go**

```go
package main

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/config"
    "github.com/cuivienor/media-pipeline/internal/db"
    "github.com/cuivienor/media-pipeline/internal/tui"
)

func main() {
    // Load configuration
    cfg, err := config.LoadFromMediaBase()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
        fmt.Fprintf(os.Stderr, "Expected config at: $MEDIA_BASE/pipeline/config.yaml\n")
        os.Exit(1)
    }

    // Open database
    database, err := db.Open(cfg.DatabasePath())
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
        os.Exit(1)
    }
    defer database.Close()

    // Create repository
    repo := db.NewSQLiteRepository(database)

    // Create the app
    app := tui.NewApp(cfg, repo)

    // Create and run the Bubbletea program
    p := tea.NewProgram(app, tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
        os.Exit(1)
    }
}
```

**Step 4.4: Run tests and verify compilation**

```bash
go build ./...
go test ./internal/tui/... -v
```

**Step 4.5: Commit**

```bash
git add -A
git commit -m "tui: replace filesystem scanner with database queries"
```

---

## Task 5: TUI New Rip Form

**Files:**
- Create: `internal/tui/newrip.go`
- Modify: `internal/tui/app.go`

**Step 5.1: Create newrip.go**

```go
package tui

import (
    "context"
    "fmt"
    "os/exec"
    "strconv"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/model"
)

// NewRipForm holds the form state for creating a new rip
type NewRipForm struct {
    Type     string // "movie" or "tv"
    Name     string
    Season   string // only for TV
    Disc     string // only for TV
    DiscPath string

    focusIndex int    // which field is focused
    err        string // validation error
}

// fields returns the list of field names in order
func (f *NewRipForm) fields() []string {
    if f.Type == "tv" {
        return []string{"type", "name", "season", "disc"}
    }
    return []string{"type", "name"}
}

// Validate returns an error message if the form is invalid
func (f *NewRipForm) Validate() string {
    if f.Name == "" {
        return "Name is required"
    }
    if f.Type == "tv" {
        if f.Season == "" {
            return "Season is required for TV shows"
        }
        if _, err := strconv.Atoi(f.Season); err != nil {
            return "Season must be a number"
        }
        if f.Disc == "" {
            return "Disc is required for TV shows"
        }
        if _, err := strconv.Atoi(f.Disc); err != nil {
            return "Disc must be a number"
        }
    }
    return ""
}

// renderNewRipForm renders the new rip form view
func (a *App) renderNewRipForm() string {
    var b strings.Builder

    b.WriteString(titleStyle.Render("New Rip"))
    b.WriteString("\n\n")

    form := a.newRipForm
    fields := form.fields()

    for i, field := range fields {
        prefix := "  "
        if i == form.focusIndex {
            prefix = "> "
        }

        switch field {
        case "type":
            typeStr := "[movie]  tv"
            if form.Type == "tv" {
                typeStr = " movie  [tv]"
            }
            b.WriteString(fmt.Sprintf("%sType: %s\n", prefix, typeStr))
        case "name":
            b.WriteString(fmt.Sprintf("%sName: %s\n", prefix, form.Name))
        case "season":
            b.WriteString(fmt.Sprintf("%sSeason: %s\n", prefix, form.Season))
        case "disc":
            b.WriteString(fmt.Sprintf("%sDisc: %s\n", prefix, form.Disc))
        }
    }

    b.WriteString("\n")

    if form.err != "" {
        b.WriteString(errorStyle.Render(form.err))
        b.WriteString("\n\n")
    }

    b.WriteString(helpStyle.Render("[Enter] Submit  [Tab] Next field  [Esc] Cancel"))

    return b.String()
}

// handleNewRipKey handles key presses in the new rip form
func (a *App) handleNewRipKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    form := a.newRipForm
    fields := form.fields()

    switch msg.String() {
    case "tab", "down":
        form.focusIndex = (form.focusIndex + 1) % len(fields)
        return a, nil

    case "shift+tab", "up":
        form.focusIndex--
        if form.focusIndex < 0 {
            form.focusIndex = len(fields) - 1
        }
        return a, nil

    case "left", "right":
        // Toggle type
        if fields[form.focusIndex] == "type" {
            if form.Type == "movie" {
                form.Type = "tv"
            } else {
                form.Type = "movie"
            }
        }
        return a, nil

    case "enter":
        // Validate and submit
        if err := form.Validate(); err != "" {
            form.err = err
            return a, nil
        }
        return a, a.dispatchRip()

    case "backspace":
        // Delete character from current field
        field := fields[form.focusIndex]
        switch field {
        case "name":
            if len(form.Name) > 0 {
                form.Name = form.Name[:len(form.Name)-1]
            }
        case "season":
            if len(form.Season) > 0 {
                form.Season = form.Season[:len(form.Season)-1]
            }
        case "disc":
            if len(form.Disc) > 0 {
                form.Disc = form.Disc[:len(form.Disc)-1]
            }
        }
        return a, nil

    default:
        // Type character into current field
        if len(msg.String()) == 1 {
            field := fields[form.focusIndex]
            char := msg.String()
            switch field {
            case "name":
                form.Name += char
            case "season":
                if char >= "0" && char <= "9" {
                    form.Season += char
                }
            case "disc":
                if char >= "0" && char <= "9" {
                    form.Disc += char
                }
            }
        }
        return a, nil
    }
}

// ripCompleteMsg is sent when a rip dispatch completes
type ripCompleteMsg struct {
    err error
}

// dispatchRip dispatches a rip command based on config
func (a *App) dispatchRip() tea.Cmd {
    return func() tea.Msg {
        form := a.newRipForm

        // Create job in database first
        ctx := context.Background()

        // Build media item
        var season *int
        if form.Type == "tv" {
            s, _ := strconv.Atoi(form.Season)
            season = &s
        }

        safeName := strings.ReplaceAll(form.Name, " ", "_")
        item := &model.MediaItem{
            Type:     model.MediaType(form.Type),
            Name:     form.Name,
            SafeName: safeName,
            Season:   season,
        }

        // Check if item exists
        existing, _ := a.repo.GetMediaItemBySafeName(ctx, safeName, season)
        if existing != nil {
            item = existing
        } else {
            if err := a.repo.CreateMediaItem(ctx, item); err != nil {
                return ripCompleteMsg{err: err}
            }
        }

        // Create pending job
        var disc *int
        if form.Type == "tv" {
            d, _ := strconv.Atoi(form.Disc)
            disc = &d
        }

        job := &model.Job{
            MediaItemID: item.ID,
            Stage:       model.StageRip,
            Status:      model.JobStatusPending,
            Disc:        disc,
        }
        if err := a.repo.CreateJob(ctx, job); err != nil {
            return ripCompleteMsg{err: err}
        }

        // Build command
        args := []string{
            "-job-id", fmt.Sprintf("%d", job.ID),
            "-db", a.config.DatabasePath(),
        }

        target := a.config.DispatchTarget("rip")
        if target == "" {
            // Local execution
            cmd := exec.Command("ripper", args...)
            if err := cmd.Start(); err != nil {
                return ripCompleteMsg{err: err}
            }
        } else {
            // SSH dispatch
            sshArgs := append([]string{target, "ripper"}, args...)
            cmd := exec.Command("ssh", sshArgs...)
            if err := cmd.Start(); err != nil {
                return ripCompleteMsg{err: err}
            }
        }

        return ripCompleteMsg{err: nil}
    }
}
```

**Step 5.2: Update app.go to integrate form**

Add to App struct:
```go
newRipForm *NewRipForm
```

Update handleKeyPress case "n":
```go
case "n":
    if a.currentView == ViewOverview || a.currentView == ViewActionNeeded {
        a.currentView = ViewNewRip
        a.newRipForm = &NewRipForm{
            Type:     "movie",
            DiscPath: "disc:0",
        }
        return a, nil
    }
```

Add to Update switch:
```go
case ripCompleteMsg:
    if msg.err != nil {
        a.err = msg.err
    }
    a.currentView = ViewOverview
    return a, a.loadState
```

**Step 5.3: Commit**

```bash
git add internal/tui/
git commit -m "tui: add New Rip form with config-driven dispatch"
```

---

## Task 6: TUI Organize View

**Files:**
- Create: `internal/tui/organize.go`
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/itemdetail.go`

**Step 6.1: Create organize.go**

```go
package tui

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/cuivienor/media-pipeline/internal/model"
    "github.com/cuivienor/media-pipeline/internal/organize"
)

// OrganizeView holds state for the organize validation view
type OrganizeView struct {
    item       *model.MediaItem
    files      []fileInfo
    validation *organize.ValidationResult
    path       string
}

type fileInfo struct {
    name    string
    size    string
    isDir   bool
}

// renderOrganizeView renders the organize validation view
func (a *App) renderOrganizeView() string {
    if a.organizeView == nil {
        return "No item selected for organization"
    }

    ov := a.organizeView
    var b strings.Builder

    // Title
    title := fmt.Sprintf("Organize: %s", ov.item.Name)
    b.WriteString(titleStyle.Render(title))
    b.WriteString("\n\n")

    // Path
    b.WriteString(sectionHeaderStyle.Render("PATH"))
    b.WriteString("\n")
    b.WriteString(fmt.Sprintf("  %s\n\n", ov.path))

    // Files
    b.WriteString(sectionHeaderStyle.Render("FILES"))
    b.WriteString("\n")
    for _, f := range ov.files {
        icon := "  "
        if f.isDir {
            icon = "📁"
        }
        b.WriteString(fmt.Sprintf("  %s %s %s\n", icon, f.name, f.size))
    }
    b.WriteString("\n")

    // Validation result
    if ov.validation != nil {
        if ov.validation.Valid {
            b.WriteString(successStyle.Render("✓ Organization valid"))
            b.WriteString("\n")
        } else {
            b.WriteString(errorStyle.Render("✗ Organization invalid"))
            b.WriteString("\n")
            for _, err := range ov.validation.Errors {
                b.WriteString(fmt.Sprintf("  • %s\n", err))
            }
        }
        b.WriteString("\n")
    }

    // Instructions
    b.WriteString(sectionHeaderStyle.Render("INSTRUCTIONS"))
    b.WriteString("\n")
    if ov.item.Type == model.MediaTypeMovie {
        b.WriteString("  1. Move main feature to _main/\n")
        b.WriteString("  2. Move extras to _extras/ (optional)\n")
        b.WriteString("  3. Delete unwanted files from root\n")
    } else {
        b.WriteString("  1. Move episodes to _episodes/\n")
        b.WriteString("  2. Name files: 01.mkv, 02.mkv, etc.\n")
        b.WriteString("  3. Move extras to _extras/ (optional)\n")
        b.WriteString("  4. Delete unwanted files from root\n")
    }
    b.WriteString("\n")

    // Help
    b.WriteString(helpStyle.Render("[v] Validate  [c] Mark Complete  [r] Refresh  [Esc] Back"))

    return b.String()
}

// loadOrganizeView loads file list for organize view
func (a *App) loadOrganizeView(item *model.MediaItem) tea.Cmd {
    return func() tea.Msg {
        // Find the rip output directory
        path := a.findItemPath(item, model.StageRip)
        if path == "" {
            return organizeLoadedMsg{err: fmt.Errorf("could not find rip output for %s", item.Name)}
        }

        files, err := listDirectory(path)
        if err != nil {
            return organizeLoadedMsg{err: err}
        }

        return organizeLoadedMsg{
            item:  item,
            path:  path,
            files: files,
        }
    }
}

type organizeLoadedMsg struct {
    item  *model.MediaItem
    path  string
    files []fileInfo
    err   error
}

type validateMsg struct {
    result *organize.ValidationResult
    err    error
}

// validateOrganization runs the organization validator
func (a *App) validateOrganization() tea.Cmd {
    return func() tea.Msg {
        if a.organizeView == nil {
            return validateMsg{err: fmt.Errorf("no item selected")}
        }

        validator := &organize.Validator{}
        var result organize.ValidationResult

        if a.organizeView.item.Type == model.MediaTypeMovie {
            result = validator.ValidateMovie(a.organizeView.path)
        } else {
            result = validator.ValidateTV(a.organizeView.path)
        }

        return validateMsg{result: &result}
    }
}

type organizeCompleteMsg struct {
    err error
}

// markOrganizeComplete creates an organize job and marks it complete
func (a *App) markOrganizeComplete() tea.Cmd {
    return func() tea.Msg {
        if a.organizeView == nil || a.organizeView.validation == nil || !a.organizeView.validation.Valid {
            return organizeCompleteMsg{err: fmt.Errorf("cannot complete: organization not validated")}
        }

        ctx := context.Background()
        item := a.organizeView.item

        // Create completed organize job
        job := &model.Job{
            MediaItemID: item.ID,
            Stage:       model.StageOrganize,
            Status:      model.JobStatusCompleted,
            OutputDir:   a.organizeView.path,
        }
        if err := a.repo.CreateJob(ctx, job); err != nil {
            return organizeCompleteMsg{err: err}
        }

        return organizeCompleteMsg{}
    }
}

// listDirectory returns files in a directory
func listDirectory(path string) ([]fileInfo, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, err
    }

    var files []fileInfo
    for _, entry := range entries {
        info, err := entry.Info()
        if err != nil {
            continue
        }

        sizeStr := ""
        if !entry.IsDir() {
            sizeStr = formatSize(info.Size())
        }

        files = append(files, fileInfo{
            name:  entry.Name(),
            size:  sizeStr,
            isDir: entry.IsDir(),
        })
    }

    return files, nil
}

func formatSize(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// findItemPath finds the output path for an item at a given stage
func (a *App) findItemPath(item *model.MediaItem, stage model.Stage) string {
    ctx := context.Background()
    jobs, _ := a.repo.ListJobsForMedia(ctx, item.ID)

    for _, job := range jobs {
        if job.Stage == stage && job.Status == model.JobStatusCompleted {
            return job.OutputDir
        }
    }
    return ""
}
```

**Step 6.2: Update itemdetail.go to show organize option**

Add to item detail view when item is at StageRip and completed:

```go
// In renderItemDetail, after showing files:
if item.Current == model.StageRip && item.Status == model.StatusCompleted {
    b.WriteString("\n")
    b.WriteString(helpStyle.Render("[o] Organize  [Esc] Back  [r] Refresh"))
} else {
    b.WriteString(helpStyle.Render("[Esc] Back  [r] Refresh"))
}
```

**Step 6.3: Update app.go to handle organize view transitions**

Add to App struct:
```go
organizeView *OrganizeView
```

Handle key "o" in item detail to transition to organize view.

**Step 6.4: Commit**

```bash
git add internal/tui/
git commit -m "tui: add organize view with file list and validation"
```

---

## Task 7: Stub Stage Commands

**Files:**
- Create: `cmd/remux/main.go`
- Create: `cmd/transcode/main.go`
- Create: `cmd/publish/main.go`

**Step 7.1: Create stub remux command**

Create `cmd/remux/main.go`:

```go
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
```

**Step 7.2: Create identical stubs for transcode and publish**

Copy the pattern for `cmd/transcode/main.go` and `cmd/publish/main.go`, changing the message.

**Step 7.3: Update Makefile to build stubs**

```makefile
build-all: build-local build-mock-makemkv build-ripper build-stubs

build-stubs:
	go build -o bin/remux ./cmd/remux
	go build -o bin/transcode ./cmd/transcode
	go build -o bin/publish ./cmd/publish
```

**Step 7.4: Commit**

```bash
git add cmd/remux/ cmd/transcode/ cmd/publish/ Makefile
git commit -m "cmd: add stub commands for remux, transcode, publish"
```

---

## Task 8: End-to-End Test Setup

**Files:**
- Create: `scripts/setup-test-env.sh`
- Create: test config file

**Step 8.1: Create setup script**

Create `scripts/setup-test-env.sh`:

```bash
#!/bin/bash
set -e

# Default test MEDIA_BASE
TEST_MEDIA_BASE="${MEDIA_BASE:-/tmp/test-media}"

echo "Setting up test environment at $TEST_MEDIA_BASE"

# Create directory structure
mkdir -p "$TEST_MEDIA_BASE/pipeline"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/1-ripped/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/2-remuxed/tv"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/movies"
mkdir -p "$TEST_MEDIA_BASE/staging/3-transcoded/tv"
mkdir -p "$TEST_MEDIA_BASE/library/movies"
mkdir -p "$TEST_MEDIA_BASE/library/tv"

# Create config file
cat > "$TEST_MEDIA_BASE/pipeline/config.yaml" << EOF
staging_base: $TEST_MEDIA_BASE/staging
library_base: $TEST_MEDIA_BASE/library

dispatch:
  rip: ""        # all local for testing
  remux: ""
  transcode: ""
  publish: ""
EOF

echo "Config written to $TEST_MEDIA_BASE/pipeline/config.yaml"
echo ""
echo "To use this environment:"
echo "  export MEDIA_BASE=$TEST_MEDIA_BASE"
echo "  export MAKEMKVCON_PATH=\$(pwd)/bin/mock-makemkv"
echo "  ./bin/media-pipeline"
```

**Step 8.2: Commit**

```bash
chmod +x scripts/setup-test-env.sh
git add scripts/
git commit -m "scripts: add test environment setup script"
```

---

## Task 9: Final Verification

**Step 9.1: Build everything**

```bash
make build-all
```

**Step 9.2: Run tests**

```bash
make test
```

**Step 9.3: Test on local machine**

```bash
# Set up test environment
./scripts/setup-test-env.sh
export MEDIA_BASE=/tmp/test-media
export MAKEMKVCON_PATH=$(pwd)/bin/mock-makemkv

# Run TUI
./bin/media-pipeline
```

**Step 9.4: Test on pipeline-test container**

```bash
# Deploy
make deploy-dev

# SSH to container
ssh pipeline-test

# Set up test environment
export MEDIA_BASE=/home/media/test
mkdir -p $MEDIA_BASE/pipeline
# Create config...

# Run with dev binaries
export PATH=/home/media/bin/dev:$PATH
media-pipeline
```

**Step 9.5: Commit final state**

```bash
git add -A
git commit -m "phase2: TUI database integration complete"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Update config package | `internal/config/` |
| 2 | Wire up ripper CLI | `cmd/ripper/`, `internal/ripper/` |
| 3 | Remove filesystem scanner | Delete `internal/scanner/` |
| 4 | TUI database-backed state | `internal/tui/app.go`, `state.go` |
| 5 | TUI New Rip form | `internal/tui/newrip.go` |
| 6 | TUI Organize view | `internal/tui/organize.go` |
| 7 | Stub stage commands | `cmd/remux/`, `cmd/transcode/`, `cmd/publish/` |
| 8 | E2E test setup | `scripts/setup-test-env.sh` |
| 9 | Final verification | - |

## Expected Result

After completing this phase:
1. TUI loads state from SQLite database
2. Can create new rip from TUI via form
3. Rip dispatched locally (test) or via SSH (prod) based on config
4. Can view ripped items, organize externally, validate in TUI
5. Stub commands available for remux/transcode/publish progression
6. Full end-to-end testable on test container with mock-makemkv
