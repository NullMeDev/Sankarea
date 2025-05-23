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

const (
    defaultErrorBufferSize = 100
    highMemoryThresholdMB = 1000
    highDiskUsagePercent  = 90
    highErrorRatePerHour  = 100
    highAPICallsPerHour   = 1000
    unhealthyStreakLimit  = 5
    logRetentionDays     = 7
)

// HealthMonitor tracks system health
type HealthMonitor struct {
    client          *http.Client
    ticker          *time.Ticker
    mutex           sync.RWMutex
    lastCheck       time.Time
    healthyStreak   int
    unhealthyStreak int
    metrics         Metrics
    errorLog        []*ErrorEvent
    started         bool
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
    return &HealthMonitor{
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        errorLog: make([]*ErrorEvent, 0, defaultErrorBufferSize),
    }
}

// StartPeriodicChecks begins periodic health checks
func (hm *HealthMonitor) StartPeriodicChecks(interval time.Duration) {
    hm.mutex.Lock()
    if hm.started {
        hm.mutex.Unlock()
        return
    }
    hm.started = true
    hm.ticker = time.NewTicker(interval)
    hm.mutex.Unlock()

    go func() {
        defer RecoverFromPanic("health-monitor")
        for range hm.ticker.C {
            hm.PerformChecks()
        }
    }()
}

// StopChecks stops the health check ticker
func (hm *HealthMonitor) StopChecks() {
    hm.mutex.Lock()
    defer hm.mutex.Unlock()

    if hm.ticker != nil {
        hm.ticker.Stop()
    }
    hm.started = false
}

// PerformChecks runs health checks on various components
func (hm *HealthMonitor) PerformChecks() {
    hm.mutex.Lock()
    defer hm.mutex.Unlock()

    hm.lastCheck = time.Now()
    hm.metrics = collectMetrics()

    // Check memory usage
    if hm.metrics.MemoryUsageMB > highMemoryThresholdMB {
        hm.logError("High memory usage detected", "warning")
    }

    // Check disk space
    if hm.metrics.DiskUsagePercent > highDiskUsagePercent {
        hm.logError("Low disk space warning", "warning")
    }

    // Check error rate
    if hm.metrics.ErrorsPerHour > highErrorRatePerHour {
        hm.logError("High error rate detected", "warning")
    }

    // Check API rate
    if hm.metrics.APICallsPerHour > highAPICallsPerHour {
        hm.logError("High API usage detected", "warning")
    }

    // Perform database health check if enabled
    if cfg.EnableDatabase {
        if err := checkDatabaseHealth(); err != nil {
            hm.logError(fmt.Sprintf("Database health check failed: %v", err), "error")
        }
    }

    // Clean up old log files
    if err := hm.cleanupOldLogs(); err != nil {
        hm.logError(fmt.Sprintf("Failed to clean up old logs: %v", err), "warning")
    }

    // Update streaks
    if len(hm.errorLog) == 0 {
        hm.healthyStreak++
        hm.unhealthyStreak = 0
    } else {
        hm.healthyStreak = 0
        hm.unhealthyStreak++

        // Alert if unhealthy streak is too long
        if hm.unhealthyStreak >= unhealthyStreakLimit {
            msg := fmt.Sprintf("⚠️ System Health Alert: Unhealthy for %d consecutive checks", hm.unhealthyStreak)
            AuditLog(fmt.Sprintf("System has been unhealthy for %d consecutive checks", hm.unhealthyStreak))
            
            if err := sendErrorChannelMessage(msg); err != nil {
                Logger().Printf("Failed to send health alert: %v", err)
            }
        }
    }

    // Update state with new metrics
    if err := UpdateState(func(s *State) {
        s.ErrorCount = len(hm.errorLog)
        if len(hm.errorLog) > 0 {
            lastError := hm.errorLog[len(hm.errorLog)-1]
            s.LastError = lastError.Message
            s.LastErrorTime = lastError.Time
        }
    }); err != nil {
        Logger().Printf("Failed to update state with health metrics: %v", err)
    }
}

// logError adds an error event to the log
func (hm *HealthMonitor) logError(message, severity string) {
    event := &ErrorEvent{
        Time:      time.Now(),
        Component: "health-monitor",
        Message:   message,
        Severity:  severity,
    }

    hm.errorLog = append(hm.errorLog, event)
    if len(hm.errorLog) > defaultErrorBufferSize {
        hm.errorLog = hm.errorLog[len(hm.errorLog)-defaultErrorBufferSize:]
    }

    Logger().Printf("[%s] %s", severity, message)
}

// cleanupOldLogs removes log files older than the retention period
func (hm *HealthMonitor) cleanupOldLogs() error {
    logDir := "data/logs"
    threshold := time.Now().AddDate(0, 0, -logRetentionDays)

    files, err := os.ReadDir(logDir)
    if err != nil {
        return fmt.Errorf("failed to read log directory: %v", err)
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
                Logger().Printf("Failed to remove old log file %s: %v", path, err)
            }
        }
    }

    return nil
}

// GetHealthStatus returns the current health status
func (hm *HealthMonitor) GetHealthStatus() map[string]interface{} {
    hm.mutex.RLock()
    defer hm.mutex.RUnlock()

    currentState := GetState()

    return map[string]interface{}{
        "status":          getStatusString(currentState),
        "version":         cfg.Version,
        "uptime":         FormatDuration(time.Since(currentState.StartupTime)),
        "lastCheck":       hm.lastCheck,
        "metrics":         hm.metrics,
        "healthyStreak":   hm.healthyStreak,
        "unhealthyStreak": hm.unhealthyStreak,
        "errorCount":      len(hm.errorLog),
        "recentErrors":    hm.GetRecentErrors(10),
    }
}

// GetRecentErrors returns the most recent error events
func (hm *HealthMonitor) GetRecentErrors(count int) []*ErrorEvent {
    hm.mutex.RLock()
    defer hm.mutex.RUnlock()

    if count > len(hm.errorLog) {
        count = len(hm.errorLog)
    }

    result := make([]*ErrorEvent, count)
    copy(result, hm.errorLog[len(hm.errorLog)-count:])
    return result
}

// checkDatabaseHealth verifies database connectivity
func checkDatabaseHealth() error {
    if !cfg.EnableDatabase {
        return nil
    }
    if db == nil {
        return fmt.Errorf("database not initialized")
    }
    return db.Ping()
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

// sendErrorChannelMessage sends a message to the error channel
func sendErrorChannelMessage(message string) error {
    if cfg.ErrorChannelID == "" {
        return fmt.Errorf("error channel ID not configured")
    }
    
    // This function would use the Discord session to send the message
    // The actual implementation would depend on how the Discord session is managed
    return nil
}
