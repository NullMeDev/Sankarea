// cmd/sankarea/metrics.go
package main

import (
    "runtime"
    "sync"
    "time"
    
    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/disk"
    "github.com/shirou/gopsutil/v3/mem"
)

// Metrics holds system and application metrics
type Metrics struct {
    Timestamp        time.Time `json:"timestamp"`
    MemoryUsageMB    float64   `json:"memory_usage_mb"`
    CPUUsagePercent  float64   `json:"cpu_usage_percent"`
    DiskUsagePercent float64   `json:"disk_usage_percent"`
    GoroutineCount   int       `json:"goroutine_count"`
    
    // Application metrics
    ArticlesPerMinute float64 `json:"articles_per_minute"`
    ErrorsPerHour     float64 `json:"errors_per_hour"`
    APICallsPerHour   float64 `json:"api_calls_per_hour"`
    UptimeHours       float64 `json:"uptime_hours"`
    
    // Cache metrics
    CacheSize        int     `json:"cache_size"`
    CacheHitRate     float64 `json:"cache_hit_rate"`
    
    // Database metrics
    DBConnections    int     `json:"db_connections"`
    DBQueryLatencyMS float64 `json:"db_query_latency_ms"`
}

var (
    metricsMutex sync.RWMutex
    lastMetrics  *Metrics
    startTime    = time.Now()
)

// collectMetrics gathers all system and application metrics
func collectMetrics() *Metrics {
    metricsMutex.Lock()
    defer metricsMutex.Unlock()

    metrics := &Metrics{
        Timestamp:      time.Now(),
        GoroutineCount: runtime.NumGoroutine(),
        UptimeHours:    time.Since(startTime).Hours(),
    }

    // Collect memory metrics
    collectMemoryMetrics(metrics)

    // Collect CPU metrics
    collectCPUMetrics(metrics)

    // Collect disk metrics
    collectDiskMetrics(metrics)

    // Collect application metrics
    collectAppMetrics(metrics)

    // Store metrics
    lastMetrics = metrics

    // Save to database if enabled
    if cfg.EnableDatabase {
        go func(m *Metrics) {
            if err := storeMetrics(m); err != nil {
                Logger().Printf("Failed to store metrics: %v", err)
            }
        }(metrics)
    }

    return metrics
}

// collectMemoryMetrics gathers memory-related metrics
func collectMemoryMetrics(metrics *Metrics) {
    // Runtime memory stats
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    metrics.MemoryUsageMB = float64(memStats.Alloc) / 1024 / 1024

    // System memory stats
    if vmem, err := mem.VirtualMemory(); err == nil {
        metrics.MemoryUsagePercent = vmem.UsedPercent
    }
}

// collectCPUMetrics gathers CPU-related metrics
func collectCPUMetrics(metrics *Metrics) {
    if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
        metrics.CPUUsagePercent = cpuPercent[0]
    }
}

// collectDiskMetrics gathers disk-related metrics
func collectDiskMetrics(metrics *Metrics) {
    if usage, err := disk.Usage("."); err == nil {
        metrics.DiskUsagePercent = usage.UsedPercent
    }
}

// collectAppMetrics gathers application-specific metrics
func collectAppMetrics(metrics *Metrics) {
    state := GetState()
    now := time.Now()

    // Calculate articles per minute
    timeWindow := now.Add(-time.Hour)
    articles, _ := getArticlesInTimeRange(timeWindow, now)
    metrics.ArticlesPerMinute = float64(len(articles)) / 60

    // Calculate errors per hour
    metrics.ErrorsPerHour = float64(state.ErrorCount)

    // Database metrics if enabled
    if cfg.EnableDatabase && db != nil {
        metrics.DBConnections = db.Stats().OpenConnections
        metrics.DBQueryLatencyMS = calculateAverageQueryLatency()
    }

    // Cache metrics
    metrics.CacheSize = len(processor.cache)
    metrics.CacheHitRate = calculateCacheHitRate()
}

// getArticlesInTimeRange retrieves articles within a time range
func getArticlesInTimeRange(start, end time.Time) ([]Article, error) {
    if !cfg.EnableDatabase || db == nil {
        return nil, ErrDatabaseNotInitialized
    }

    query := `
        SELECT *
        FROM articles
        WHERE timestamp BETWEEN $1 AND $2
    `

    var articles []Article
    err := db.Select(&articles, query, start, end)
    return articles, err
}

// calculateAverageQueryLatency calculates average database query latency
func calculateAverageQueryLatency() float64 {
    if !cfg.EnableDatabase || db == nil {
        return 0
    }

    // Execute a simple query to measure latency
    start := time.Now()
    db.Query("SELECT 1")
    return float64(time.Since(start).Milliseconds())
}

// calculateCacheHitRate calculates the cache hit rate
func calculateCacheHitRate() float64 {
    if processor == nil {
        return 0
    }

    totalRequests := float64(processor.cacheHits + processor.cacheMisses)
    if totalRequests == 0 {
        return 0
    }

    return float64(processor.cacheHits) / totalRequests * 100
}

// GetLastMetrics returns the most recently collected metrics
func GetLastMetrics() *Metrics {
    metricsMutex.RLock()
    defer metricsMutex.RUnlock()
    
    if lastMetrics == nil {
        return collectMetrics()
    }
    return lastMetrics
}

// StartMetricsCollection begins periodic metrics collection
func StartMetricsCollection(interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                collectMetrics()
            case <-ctx.Done():
                return
            }
        }
    }()
}
