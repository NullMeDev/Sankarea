package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// HealthStatus represents the current health status of the bot
type HealthStatus string

const (
	// HealthStatusHealthy indicates the bot is functioning normally
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusWarning indicates the bot has non-critical issues
	HealthStatusWarning HealthStatus = "warning"
	// HealthStatusDegraded indicates the bot is functioning with reduced capabilities
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusCritical indicates the bot has critical issues
	HealthStatusCritical HealthStatus = "critical"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message"`
	LastChecked time.Time    `json:"last_checked"`
}

// HealthMonitor manages health checks and metrics
type HealthMonitor struct {
	Status       HealthStatus               `json:"status"`
	Checks       map[string]HealthCheckResult `json:"checks"`
	StartTime    time.Time                  `json:"start_time"`
	LastWarning  time.Time                  `json:"last_warning"`
	LastCritical time.Time                  `json:"last_critical"`
	mutex        sync.RWMutex
}

// Metrics holds runtime metrics for the bot
type Metrics struct {
	Uptime             string  `json:"uptime"`
	GoroutineCount     int     `json:"goroutine_count"`
	MemoryUsageMB      float64 `json:"memory_usage_mb"`
	TotalArticles      int     `json:"total_articles"`
	ErrorCount         int     `json:"error_count"`
	SuccessfulChecks   int     `json:"successful_checks"`
	UnsuccessfulChecks int     `json:"unsuccessful_checks"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	APICallsCount      int     `json:"api_calls_count"`
}

// Global health monitor
var healthMonitor *HealthMonitor

// InitHealthMonitor initializes the health monitoring system
func InitHealthMonitor() *HealthMonitor {
	monitor := &HealthMonitor{
		Status:    HealthStatusHealthy,
		Checks:    make(map[string]HealthCheckResult),
		StartTime: time.Now(),
		mutex:     sync.RWMutex{},
	}
	
	healthMonitor = monitor
	return monitor
}

// StartHealthServer starts an HTTP server for health checks
func StartHealthServer(port int) {
	if healthMonitor == nil {
		InitHealthMonitor()
	}
	
	http.HandleFunc("/health", handleHealthCheck)
	http.HandleFunc("/metrics", handleMetrics)
	
	go func() {
		addr := fmt.Sprintf(":%d", port)
		Logger().Printf("Starting health server on %s", addr)
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			Logger().Printf("Health server failed: %v", err)
		}
	}()
}

// handleHealthCheck responds to HTTP health check requests
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if healthMonitor == nil {
		http.Error(w, "Health monitor not initialized", http.StatusServiceUnavailable)
		return
	}
	
	healthMonitor.mutex.RLock()
	status := healthMonitor.Status
	healthMonitor.mutex.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	switch status {
	case HealthStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case HealthStatusWarning:
		w.WriteHeader(http.StatusOK) // Still OK but with warning
	case HealthStatusDegraded:
		w.WriteHeader(http.StatusOK) // Still functioning but degraded
	case HealthStatusCritical:
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	
	// Write JSON response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      status,
		"version":     cfg.Version,
		"uptime":      time.Since(healthMonitor.StartTime).String(),
		"checks":      healthMonitor.Checks,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// handleMetrics responds to HTTP metrics requests
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := collectMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// collectMetrics gathers runtime metrics
func collectMetrics() Metrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Load state for application metrics
	state, err := LoadState()
	totalArticles := 0
	errorCount := 0
	if err == nil {
		totalArticles = state.TotalArticles
		errorCount = state.ErrorCount
	}
	
	return Metrics{
		Uptime:           time.Since(healthMonitor.StartTime).String(),
		GoroutineCount:   runtime.NumGoroutine(),
		MemoryUsageMB:    float64(memStats.Alloc) / 1024 / 1024,
		TotalArticles:    totalArticles,
		ErrorCount:       errorCount,
		CPUUsagePercent:  0, // Would require additional dependencies to measure
		APICallsCount:    0, // Would need to track API calls
	}
}

// UpdateHealthCheck updates a specific health check
func (hm *HealthMonitor) UpdateHealthCheck(name string, status HealthStatus, message string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	hm.Checks[name] = HealthCheckResult{
		Status:      status,
		Message:     message,
		LastChecked: time.Now(),
	}
	
	// Update overall status based on worst check
	worstStatus := HealthStatusHealthy
	for _, check := range hm.Checks {
		if check.Status == HealthStatusCritical {
			worstStatus = HealthStatusCritical
			break
		} else if check.Status == HealthStatusDegraded && worstStatus != HealthStatusCritical {
			worstStatus = HealthStatusDegraded
		} else if check.Status == HealthStatusWarning && worstStatus != HealthStatusCritical && worstStatus != HealthStatusDegraded {
			worstStatus = HealthStatusWarning
		}
	}
	
	// Update overall status
	prevStatus := hm.Status
	hm.Status = worstStatus
	
	// Record timestamps for status changes
	if worstStatus == HealthStatusWarning && prevStatus != HealthStatusWarning {
		hm.LastWarning = time.Now()
	} else if worstStatus == HealthStatusCritical && prevStatus != HealthStatusCritical {
		hm.LastCritical = time.Now()
	}
}

// PerformHealthChecks runs all health checks
func (hm *HealthMonitor) PerformHealthChecks() {
	// Check disk space
	hm.CheckDiskSpace()
	
	// Check Discord connection
	hm.CheckDiscordConnection()
	
	// Check database connection if enabled
	if db != nil {
		hm.CheckDatabaseConnection()
	}
	
	// Check API keys
	hm.CheckAPIKeys()
}

// CheckDiskSpace checks available disk space
func (hm *HealthMonitor) CheckDiskSpace() {
	// Get file system statistics
	var stat syscall.Statfs_t
	err := syscall.Statfs(".", &stat)
	if err != nil {
		hm.UpdateHealthCheck("disk_space", HealthStatusWarning, "Failed to check disk space")
		return
	}
	
	// Calculate available disk space in GB
	available := float64(stat.Bavail * uint64(stat.Bsize)) / 1024 / 1024 / 1024
	
	if available < 0.1 { // Less than 100MB
		hm.UpdateHealthCheck("disk_space", HealthStatusCritical, fmt.Sprintf("Critical disk space: %.2f GB available", available))
	} else if available < 1.0 { // Less than 1GB
		hm.UpdateHealthCheck("disk_space", HealthStatusWarning, fmt.Sprintf("Low disk space: %.2f GB available", available))
	} else {
		hm.UpdateHealthCheck("disk_space", HealthStatusHealthy, fmt.Sprintf("%.2f GB available", available))
	}
}

// CheckDiscordConnection checks Discord gateway connection
func (hm *HealthMonitor) CheckDiscordConnection() {
	if dg == nil {
		hm.UpdateHealthCheck("discord_connection", HealthStatusCritical, "Discord client not initialized")
		return
	}
	
	// Check if connected to gateway
	if !dg.DataReady {
		hm.UpdateHealthCheck("discord_connection", HealthStatusCritical, "Not connected to Discord gateway")
		return
	}
	
	hm.UpdateHealthCheck("discord_connection", HealthStatusHealthy, "Connected to Discord gateway")
}

// CheckDatabaseConnection checks database connection
func (hm *HealthMonitor) CheckDatabaseConnection() {
	if db == nil {
		hm.UpdateHealthCheck("database", HealthStatusWarning, "Database not initialized")
		return
	}
	
	// Try a simple query
	err := db.Ping()
	if err != nil {
		hm.UpdateHealthCheck("database", HealthStatusCritical, fmt.Sprintf("Database connection error: %v", err))
		return
	}
	
	hm.UpdateHealthCheck("database", HealthStatusHealthy, "Database connection established")
}

// CheckAPIKeys checks if necessary API keys are configured
func (hm *HealthMonitor) CheckAPIKeys() {
	if cfg == nil {
		hm.UpdateHealthCheck("api_keys", HealthStatusWarning, "Configuration not loaded")
		return
	}
	
	missingKeys := []string{}
	
	// Check if features are enabled but keys are missing
	if cfg.EnableFactCheck {
		if cfg.GoogleFactCheckAPIKey == "" && cfg.ClaimBustersAPIKey == "" && cfg.OpenAIAPIKey == "" {
			missingKeys = append(missingKeys, "Fact Check API keys")
		}
	}
	
	if cfg.EnableSummarization && cfg.OpenAIAPIKey == "" {
		missingKeys = append(missingKeys, "OpenAI API key")
	}
	
	if len(missingKeys) > 0 {
		hm.UpdateHealthCheck("api_keys", HealthStatusWarning, 
			fmt.Sprintf("Missing API keys: %v", missingKeys))
	} else {
		hm.UpdateHealthCheck("api_keys", HealthStatusHealthy, "All required API keys configured")
	}
}

// RunPeriodicHealthChecks schedules regular health checks
func RunPeriodicHealthChecks(interval time.Duration) {
	if healthMonitor == nil {
		InitHealthMonitor()
	}
	
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				healthMonitor.PerformHealthChecks()
			}
		}
	}()
}
