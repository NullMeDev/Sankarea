// cmd/sankarea/scheduler.go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/robfig/cron/v3"
)

// Scheduler manages scheduled tasks
type Scheduler struct {
    cron      *cron.Cron
    mutex     sync.RWMutex
    jobs      map[string]cron.EntryID
    ctx       context.Context
    cancel    context.CancelFunc
    isRunning bool
}

// NewScheduler creates a new scheduler instance
func NewScheduler() *Scheduler {
    ctx, cancel := context.WithCancel(context.Background())
    return &Scheduler{
        cron:  cron.New(cron.WithSeconds()),
        jobs:  make(map[string]cron.EntryID),
        ctx:   ctx,
        cancel: cancel,
    }
}

// Start begins the scheduler
func (s *Scheduler) Start() error {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    if s.isRunning {
        return fmt.Errorf("scheduler is already running")
    }

    // Schedule news fetching
    if cfg.News15MinCron != "" {
        if err := s.scheduleNewsUpdate(cfg.News15MinCron); err != nil {
            return fmt.Errorf("failed to schedule news updates: %v", err)
        }
    }

    // Schedule daily digest
    if cfg.DigestCronSchedule != "" {
        if err := s.scheduleDigest(cfg.DigestCronSchedule); err != nil {
            return fmt.Errorf("failed to schedule digest: %v", err)
        }
    }

    // Start the cron scheduler
    s.cron.Start()
    s.isRunning = true

    // Update next run times in state
    if err := s.updateNextRunTimes(); err != nil {
        Logger().Printf("Failed to update next run times: %v", err)
    }

    return nil
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
    s.mutex.Lock()
    defer s.mutex.Unlock()

    if !s.isRunning {
        return
    }

    s.cancel()
    ctx := s.cron.Stop()
    <-ctx.Done()
    s.isRunning = false
}

// scheduleNewsUpdate sets up the news fetching schedule
func (s *Scheduler) scheduleNewsUpdate(schedule string) error {
    id, err := s.cron.AddFunc(schedule, func() {
        if err := s.runNewsUpdate(); err != nil {
            Logger().Printf("Scheduled news update failed: %v", err)
        }
    })
    if err != nil {
        return fmt.Errorf("failed to schedule news update: %v", err)
    }
    s.jobs["news"] = id
    return nil
}

// scheduleDigest sets up the daily digest schedule
func (s *Scheduler) scheduleDigest(schedule string) error {
    id, err := s.cron.AddFunc(schedule, func() {
        if err := s.runDailyDigest(); err != nil {
            Logger().Printf("Scheduled digest failed: %v", err)
        }
    })
    if err != nil {
        return fmt.Errorf("failed to schedule digest: %v", err)
    }
    s.jobs["digest"] = id
    return nil
}

// runNewsUpdate executes the news update task
func (s *Scheduler) runNewsUpdate() error {
    // Check if system is in lockdown or paused
    state := GetState()
    if state.Lockdown {
        return fmt.Errorf("system is in lockdown mode")
    }
    if state.Paused {
        return fmt.Errorf("system is paused")
    }

    // Create a context with timeout for the news fetch
    ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
    defer cancel()

    // Update state before starting
    if err := UpdateState(func(s *State) {
        s.LastFetchTime = time.Now()
    }); err != nil {
        return fmt.Errorf("failed to update state: %v", err)
    }

    // Perform news fetch
    if err := fetchNewsWithContext(ctx); err != nil {
        return fmt.Errorf("news fetch failed: %v", err)
    }

    return nil
}

// runDailyDigest executes the daily digest task
func (s *Scheduler) runDailyDigest() error {
    state := GetState()
    if state.Lockdown {
        return fmt.Errorf("system is in lockdown mode")
    }
    if state.Paused {
        return fmt.Errorf("system is paused")
    }

    // Create a context with timeout for the digest generation
    ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
    defer cancel()

    // Update state before starting
    if err := UpdateState(func(s *State) {
        s.LastDigest = time.Now()
    }); err != nil {
        return fmt.Errorf("failed to update state: %v", err)
    }

    // Generate and send digest
    if err := generateDigestWithContext(ctx); err != nil {
        return fmt.Errorf("digest generation failed: %v", err)
    }

    return nil
}

// updateNextRunTimes updates the state with next scheduled run times
func (s *Scheduler) updateNextRunTimes() error {
    s.mutex.RLock()
    defer s.mutex.RUnlock()

    entries := s.cron.Entries()
    nextNews := time.Time{}
    nextDigest := time.Time{}

    for _, entry := range entries {
        if jobID, exists := s.jobs["news"]; exists && entry.ID == jobID {
            nextNews = entry.Next
        }
        if jobID, exists := s.jobs["digest"]; exists && entry.ID == jobID {
            nextDigest = entry.Next
        }
    }

    return UpdateState(func(s *State) {
        s.NewsNextTime = nextNews
        s.DigestNextTime = nextDigest
    })
}

// IsRunning returns whether the scheduler is currently active
func (s *Scheduler) IsRunning() bool {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    return s.isRunning
}

// GetNextRunTimes returns the next scheduled run times for news and digest
func (s *Scheduler) GetNextRunTimes() (news, digest time.Time) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()

    state := GetState()
    return state.NewsNextTime, state.DigestNextTime
}
