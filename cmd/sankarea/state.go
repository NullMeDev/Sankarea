// cmd/sankarea/state.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"
)

// State represents the runtime state of the application
type State struct {
    StartupTime     time.Time         `json:"startup_time"`
    LastUpdate      time.Time         `json:"last_update"`
    ArticleCount    int               `json:"article_count"`
    ErrorCount      int               `json:"error_count"`
    APICallCount    int               `json:"api_call_count"`
    ConnectedGuilds int               `json:"connected_guilds"`
    ActiveSources   int               `json:"active_sources"`
    HealthStatus    string            `json:"health_status"`
    Components      map[string]Status `json:"components"`
    mutex           sync.RWMutex
}

// Status represents the status of a component
type Status struct {
    Status      string    `json:"status"`  // "ok", "degraded", "error"
    LastCheck   time.Time `json:"last_check"`
    LastError   string    `json:"last_error,omitempty"`
    Description string    `json:"description"`
}

var (
    state     *State
    stateMux  sync.RWMutex
    stateFile = "data/state.json"
)

// InitState initializes the application state
func InitState() error {
    stateMux.Lock()
    defer stateMux.Unlock()

    state = &State{
        StartupTime: time.Now(),
        LastUpdate:  time.Now(),
        Components:  make(map[string]Status),
        HealthStatus: "starting",
    }

    // Ensure data directory exists
    if err := os.MkdirAll("data", 0755); err != nil {
        return fmt.Errorf("failed to create data directory: %v", err)
    }

    // Try to load existing state
    if err := loadState(); err != nil {
        Logger().Printf("Could not load existing state: %v", err)
        // Continue with fresh state
    }

    // Initialize component statuses
    state.Components["discord"] = Status{Status: "starting", Description: "Discord connection"}
    state.Components["database"] = Status{Status: "starting", Description: "Database connection"}
    state.Components["news_fetcher"] = Status{Status: "starting", Description: "News fetching service"}
    state.Components["scheduler"] = Status{Status: "starting", Description: "Task scheduler"}

    return saveState()
}

// GetState returns a copy of the current state
func GetState() State {
    stateMux.RLock()
    defer stateMux.RUnlock()

    return *state
}

// UpdateComponentStatus updates the status of a specific component
func UpdateComponentStatus(component string, status string, err error) {
    stateMux.Lock()
    defer stateMux.Unlock()

    state.Components[component] = Status{
        Status:    status,
        LastCheck: time.Now(),
        LastError: err.Error(),
    }

    // Update overall health status
    updateHealthStatus()
    saveState()
}

// updateHealthStatus updates the overall health status based on component statuses
func updateHealthStatus() {
    hasError := false
    hasDegraded := false

    for _, status := range state.Components {
        switch status.Status {
        case "error":
            hasError = true
        case "degraded":
            hasDegraded = true
        }
    }

    if hasError {
        state.HealthStatus = "error"
    } else if hasDegraded {
        state.HealthStatus = "degraded"
    } else {
        state.HealthStatus = "ok"
    }
}

// IncrementCounter safely increments a counter in the state
func IncrementCounter(counter string) {
    stateMux.Lock()
    defer stateMux.Unlock()

    switch counter {
    case "article":
        state.ArticleCount++
    case "error":
        state.ErrorCount++
    case "api_call":
        state.APICallCount++
    }

    state.LastUpdate = time.Now()
    saveState()
}

// UpdateGuildCount updates the number of connected guilds
func UpdateGuildCount(count int) {
    stateMux.Lock()
    defer stateMux.Unlock()

    state.ConnectedGuilds = count
    state.LastUpdate = time.Now()
    saveState()
}

// UpdateSourceCount updates the number of active sources
func UpdateSourceCount(count int) {
    stateMux.Lock()
    defer stateMux.Unlock()

    state.ActiveSources = count
    state.LastUpdate = time.Now()
    saveState()
}

// saveState saves the current state to disk
func saveState() error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal state: %v", err)
    }

    if err := os.WriteFile(stateFile, data, 0644); err != nil {
        return fmt.Errorf("failed to write state file: %v", err)
    }

    return nil
}

// loadState loads the state from disk
func loadState() error {
    data, err := os.ReadFile(stateFile)
    if err != nil {
        return fmt.Errorf("failed to read state file: %v", err)
    }

    if err := json.Unmarshal(data, state); err != nil {
        return fmt.Errorf("failed to unmarshal state: %v", err)
    }

    // Reset volatile fields
    state.StartupTime = time.Now()
    state.HealthStatus = "starting"

    return nil
}

// GetMetrics returns current metrics from the state
func GetMetrics() Metrics {
    stateMux.RLock()
    defer stateMux.RUnlock()

    return Metrics{
        UptimeSeconds:     time.Since(state.StartupTime).Seconds(),
        ArticlesPerMinute: calculateRate(state.ArticleCount, state.StartupTime),
        ErrorsPerHour:     calculateRate(state.ErrorCount, state.StartupTime) * 60,
        APICallsPerHour:   calculateRate(state.APICallCount, state.StartupTime) * 60,
        ConnectedGuilds:   state.ConnectedGuilds,
        ActiveSources:     state.ActiveSources,
        HealthStatus:      state.HealthStatus,
    }
}

// calculateRate calculates the rate of events per minute
func calculateRate(count int, since time.Time) float64 {
    duration := time.Since(since)
    if duration == 0 {
        return 0
    }
    return float64(count) / duration.Minutes()
}
