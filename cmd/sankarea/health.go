// cmd/sankarea/health.go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
    "runtime"
    "os"
    "path/filepath"
)

// HealthMonitor tracks system health
type HealthMonitor struct {
    client          *http.Client
    ticker          *time.Ticker
    mutex           sync.Mutex
    lastCheck       time.Time
    healthyStreak   int
    unhealthyStreak int
    metrics         Metrics
    errorLog        []ErrorEvent
}

var healthMonitor *HealthMonitor

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
    return &HealthMonitor{
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        errorLog: make([]ErrorEvent, 0),
    }
}

// handleSystemHealthCheck handles system-level health checks
func handleSystemHealthCheck(w http.ResponseWriter, r *http.Request) {
    mutex.RLock()
    currentState := *state
    mutex.RUnlock()

    metrics := collectMetrics()
    
    response := map[string]interface{}{
        "status":        getStatusString(&currentState),
        "version":       cfg.Version,
        "uptime":       FormatDuration(time.Since(currentState.StartupTime)),
        "lastCheck":     healthMonitor.lastCheck,
        "metrics":       metrics,
        "errorCount":    currentState.ErrorCount,
        "feedCount":     currentState.FeedCount,
        "lastError":     currentState.LastError,
        "lastErrorTime": currentState.LastErrorTime,
        "paused":       currentState.Paused,
        "lockdown":     currentState.Lockdown,
    }

    respondWithJSON(w, http.StatusOK, response)
}

// StartPeriodicChecks begins periodic health checks
func (hm *HealthMonitor) StartPeriodicChecks(interval time.Duration) {
    hm.ticker = time.NewTicker(interval)
    go func() {
        defer RecoverFromPanic("health-monitor")
        for range hm.ticker.C {
            hm.PerformChecks()
        }
    }()
}

// StopChecks stops the health check ticker
func (hm *HealthMonitor) StopChecks() {
    if hm.ticker != nil {
        hm.ticker.Stop()
    }
}

// PerformChecks runs health checks on various components
func (hm *HealthMonitor) PerformChecks() {
    hm.mutex.Lock()
    defer hm.mutex.Unlock()

    hm.lastCheck = time.Now()
    hm.metrics = collectMetrics()

    // Check memory usage
    if hm.metrics.MemoryUsageMB > 1000 { // 1GB threshold
        hm.logError("High memory usage detected", "warning")
    }

    // Check disk space
    if hm.metrics.DiskUsagePercent > 90 {
        hm.logError("Low disk space warning", "warning")
    }

    // Check error rate
    if hm.metrics.ErrorsPerHour > 100 {
        hm.logError("High error rate detected", "warning")
    }

    // Check API rate
    if hm.metrics.APICallsPerHour > 1000 {
        hm.logError("High API usage detected", "warning")
    }

    // Perform database health check if enabled
    if cfg.EnableDatabase {
        if err := checkDatabaseHealth(); err != nil {
            hm.logError(fmt.Sprintf("Database health check failed: %v", err), "error")
        }
    }

    // Clean up old log files
    hm.cleanupOldLogs()

    // Update streaks
    if len(hm.errorLog) == 0 {
        hm.healthyStreak++
        hm.unhealthyStreak = 0
    } else {
        hm.healthyStreak = 0
        hm.unhealthyStreak++
    }

    // Alert if unhealthy streak is too long
    if hm.unhealthyStreak >= 5 {
        AuditLog(fmt.Sprintf("System has been unhealthy for %d consecutive checks", hm.unhealthyStreak))
    }
}

// logError adds an error event to the log
func (hm *HealthMonitor) logError(message, severity string) {
    event := ErrorEvent{
        Time:      time.Now(),
        Component: "health-monitor",
        Message:   message,
        Severity:  severity,
    }

    hm.errorLog = append(hm.errorLog, event)
    if len(hm.errorLog) > 100 {
        // Keep only the last 100 errors
        hm.errorLog = hm.errorLog[len(hm.errorLog)-100:]
    }

    // Log to system logger
    Logger().Printf("[%s] %s", severity, message)
}

// cleanupOldLogs removes log files older than 7 days
func (hm *HealthMonitor) cleanupOldLogs() {
    logDir := "data/logs"
    threshold := time.Now().AddDate(0, 0, -7)

    files, err := os.ReadDir(logDir)
    if err != nil {
        hm.logError(fmt.Sprintf("Failed to read log directory: %v", err), "warning")
        return
    }

    for _, file := range files {
        if file.IsDir() {
            continue
        }

        info, err := file.Info()
        if err != nil {
            continue
        }

        if info.ModTime().Before(threshold) {
            path := filepath.Join(logDir, file.Name())
            if err := os.Remove(path); err != nil {
                hm.logError(fmt.Sprintf("Failed to remove old log file %s: %v", path, err), "warning")
            }
        }
    }
}

// getStatusString returns the current system status
func getStatusString(state *State) string {
    if state.Lockdown {
        return "lockdown"
    }
    if state.Paused {
        return "paused"
    }
    if state.ErrorCount > 0 {
        return "warning"
    }
    return "healthy"
}

// checkDatabaseHealth verifies database connectivity
func checkDatabaseHealth() error {
    if db == nil {
        return fmt.Errorf("database not initialized")
    }
    return db.Ping()
}

// GetHealthStatus returns the current health status
func GetHealthStatus() map[string]interface{} {
    mutex.RLock()
    currentState := *state
    mutex.RUnlock()

    healthMonitor.mutex.Lock()
    defer healthMonitor.mutex.Unlock()

    return map[string]interface{}{
        "status":          getStatusString(&currentState),
        "version":         cfg.Version,
        "uptime":         FormatDuration(time.Since(currentState.StartupTime)),
        "lastCheck":       healthMonitor.lastCheck,
        "metrics":         healthMonitor.metrics,
        "healthyStreak":   healthMonitor.healthyStreak,
        "unhealthyStreak": healthMonitor.unhealthyStreak,
        "errorCount":      len(healthMonitor.errorLog),
        "recentErrors":    healthMonitor.errorLog,
    }
}

// ResetHealthMonitor resets the health monitor state
func ResetHealthMonitor() {
    healthMonitor.mutex.Lock()
    defer healthMonitor.mutex.Unlock()

    healthMonitor.errorLog = make([]ErrorEvent, 0)
    healthMonitor.healthyStreak = 0
    healthMonitor.unhealthyStreak = 0
    healthMonitor.lastCheck = time.Now()
}
