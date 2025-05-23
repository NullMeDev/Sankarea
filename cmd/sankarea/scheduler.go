// cmd/sankarea/scheduler.go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Scheduler manages periodic tasks for the bot
type Scheduler struct {
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
    mutex      sync.RWMutex
    tasks      map[string]*Task
    discord    *discordgo.Session
}

// Task represents a scheduled task
type Task struct {
    Name        string
    Interval    time.Duration
    LastRun     time.Time
    IsRunning   bool
    Enabled     bool
    Handler     func(context.Context) error
}

// NewScheduler creates a new Scheduler instance
func NewScheduler(discord *discordgo.Session) *Scheduler {
    ctx, cancel := context.WithCancel(context.Background())
    return &Scheduler{
        ctx:     ctx,
        cancel:  cancel,
        tasks:   make(map[string]*Task),
        discord: discord,
    }
}

// Start initializes and starts all scheduled tasks
func (s *Scheduler) Start() error {
    Logger().Println("Starting scheduler...")

    // Register default tasks
    s.registerDefaultTasks()

    // Start all enabled tasks
    for name, task := range s.tasks {
        if task.Enabled {
            s.wg.Add(1)
            go s.runTask(name, task)
        }
    }

    return nil
}

// Stop gracefully stops all scheduled tasks
func (s *Scheduler) Stop() {
    Logger().Println("Stopping scheduler...")
    s.cancel()
    s.wg.Wait()
}

// RegisterTask adds a new task to the scheduler
func (s *Scheduler) RegisterTask(name string, interval time.Duration, handler func(context.Context) error) {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    s.tasks[name] = &Task{
        Name:     name,
        Interval: interval,
        Enabled:  true,
        Handler:  handler,
    }
}

// registerDefaultTasks sets up the default bot tasks
func (s *Scheduler) registerDefaultTasks() {
    // News fetching task
    s.RegisterTask("news_fetch", time.Duration(cfg.NewsIntervalMinutes)*time.Minute, func(ctx context.Context) error {
        np := NewNewsProcessor()
        return np.ProcessNews(ctx, s.discord)
    })

    // Daily digest task - runs at configured time
    s.RegisterTask("daily_digest", 24*time.Hour, func(ctx context.Context) error {
        // Only run if within the configured time window
        if isDigestTime() {
            return generateAndSendDigest(ctx, s.discord)
        }
        return nil
    })

    // Fact-checking update task
    s.RegisterTask("fact_check_update", 6*time.Hour, func(ctx context.Context) error {
        return updateFactChecks(ctx, s.discord)
    })

    // Metrics collection task
    s.RegisterTask("metrics_collection", time.Duration(cfg.MetricsInterval), func(ctx context.Context) error {
        return collectMetrics(ctx)
    })

    // State backup task
    s.RegisterTask("state_backup", time.Duration(cfg.StateBackupInterval), func(ctx context.Context) error {
        return SaveState(state)
    })

    // Health check task
    s.RegisterTask("health_check", 5*time.Minute, func(ctx context.Context) error {
        return performHealthCheck(ctx)
    })
}

// runTask executes a task at its specified interval
func (s *Scheduler) runTask(name string, task *Task) {
    defer s.wg.Done()
    defer RecoverFromPanic(fmt.Sprintf("task-%s", name))

    ticker := time.NewTicker(task.Interval)
    defer ticker.Stop()

    Logger().Printf("Started task: %s (interval: %v)", name, task.Interval)

    for {
        select {
        case <-s.ctx.Done():
            Logger().Printf("Stopping task: %s", name)
            return
        case <-ticker.C:
            if err := s.executeTask(name, task); err != nil {
                Logger().Printf("Error in task %s: %v", name, err)
                
                // Update error stats
                if err := UpdateState(func(s *State) {
                    s.ErrorCount++
                    s.LastError = err.Error()
                    s.LastErrorTime = time.Now()
                }); err != nil {
                    Logger().Printf("Failed to update state after task error: %v", err)
                }
            }
        }
    }
}

// executeTask runs a single task with proper state management
func (s *Scheduler) executeTask(name string, task *Task) error {
    s.mutex.Lock()
    if task.IsRunning {
        s.mutex.Unlock()
        return fmt.Errorf("task %s is already running", name)
    }
    task.IsRunning = true
    task.LastRun = time.Now()
    s.mutex.Unlock()

    defer func() {
        s.mutex.Lock()
        task.IsRunning = false
        s.mutex.Unlock()
    }()

    // Create task context with timeout
    ctx, cancel := context.WithTimeout(s.ctx, DefaultTimeout)
    defer cancel()

    // Execute the task
    return task.Handler(ctx)
}

// Helper functions

func isDigestTime() bool {
    now := time.Now().UTC()
    targetHour := cfg.DigestHour
    targetMinute := cfg.DigestMinute

    return now.Hour() == targetHour && now.Minute() == targetMinute
}

func updateFactChecks(ctx context.Context, s *discordgo.Session) error {
    fc := NewFactChecker()
    state, err := LoadState()
    if err != nil {
        return err
    }

    // Update fact checks for recent articles
    for _, article := range state.RecentArticles {
        if time.Since(article.LastFactCheck) > 6*time.Hour {
            result, err := fc.CheckArticle(ctx, article)
            if err != nil {
                Logger().Printf("Error fact-checking article %s: %v", article.ID, err)
                continue
            }

            // Update article with new fact check results
            article.FactCheckResult = result
            article.LastFactCheck = time.Now()

            // Notify if reliability changed significantly
            if needsReliabilityUpdate(article) {
                notifyReliabilityChange(s, article)
            }
        }
    }

    return nil
}

func collectMetrics(ctx context.Context) error {
    metrics := &Metrics{
        Timestamp:     time.Now(),
        ArticleCount:  state.ArticleCount,
        ErrorCount:    state.ErrorCount,
        SourceCount:   len(state.Sources),
        UpTime:       time.Since(state.StartupTime),
    }

    // Save metrics to database if enabled
    if cfg.EnableDatabase {
        if err := saveMetrics(ctx, metrics); err != nil {
            return fmt.Errorf("failed to save metrics: %v", err)
        }
    }

    // Update dashboard if enabled
    if cfg.EnableDashboard {
        if err := dashboard.UpdateMetrics(metrics); err != nil {
            return fmt.Errorf("failed to update dashboard metrics: %v", err)
        }
    }

    return nil
}

func performHealthCheck(ctx context.Context) error {
    // Check critical services
    checks := map[string]bool{
        "discord": s.discord.State.User != nil,
        "database": checkDatabaseConnection(),
        "apis": checkExternalAPIs(),
    }

    // Update health status
    status := StatusOK
    for service, healthy := range checks {
        if !healthy {
            Logger().Printf("Health check failed for %s", service)
            status = StatusDegraded
        }
    }

    // Update state
    return UpdateState(func(s *State) {
        s.HealthStatus = status
        s.LastHealthCheck = time.Now()
    })
}

func checkDatabaseConnection() bool {
    if !cfg.EnableDatabase || db == nil {
        return true
    }
    return db.PingContext(context.Background()) == nil
}

func checkExternalAPIs() bool {
    apis := []string{
        "https://discord.com/api/v10",
        "https://factchecktools.googleapis.com",
        "https://idir.uta.edu/claimbuster",
    }

    for _, api := range apis {
        if _, err := http.Get(api); err != nil {
            return false
        }
    }
    return true
}
