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

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
    return &HealthMonitor{
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        errorLog: make([]ErrorEvent, 0),
    }
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

        // Alert if unhealthy streak is too long
        if hm.unhealthyStreak >= 5 {
            AuditLog(fmt.Sprintf("System has been unhealthy for %d consecutive checks", hm.unhealthyStreak))
            
            // Send alert to error channel if configured
            if cfg.ErrorChannelID != "" {
                msg := fmt.Sprintf("⚠️ System Health Alert: Unhealthy for %d consecutive checks", hm.unhealthyStreak)
                if err := sendErrorChannelMessage(msg); err != nil {
                    Logger().Printf("Failed to send health alert: %v", err)
                }
            }
        }
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

    // Update state error count
    mutex.Lock()
    state.ErrorCount++
    state.LastError = message
    state.LastErrorTime = event.Time
    mutex.Unlock()

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

// sendErrorChannelMessage sends a message to the error channel
func sendErrorChannelMessage(message string) error {
    if cfg.ErrorChannelID == "" {
        return fmt.Errorf("error channel ID not configured")
    }
    
    // This function would use the Discord session to send the message
    // The actual implementation would depend on how the Discord session is managed
    return nil
}
