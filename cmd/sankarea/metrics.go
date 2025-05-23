// cmd/sankarea/metrics.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// Metrics represents collected bot metrics
type Metrics struct {
    Timestamp     time.Time         `json:"timestamp"`
    ArticleCount  int              `json:"article_count"`
    ErrorCount    int              `json:"error_count"`
    SourceCount   int              `json:"source_count"`
    UpTime        time.Duration    `json:"uptime"`
    CategoryStats map[string]int   `json:"category_stats"`
    SourceStats   map[string]int   `json:"source_stats"`
    Reliability   ReliabilityStats `json:"reliability"`
}

// ReliabilityStats tracks fact-checking statistics
type ReliabilityStats struct {
    HighCount    int     `json:"high_count"`
    MediumCount  int     `json:"medium_count"`
    LowCount     int     `json:"low_count"`
    AverageScore float64 `json:"average_score"`
}

// MetricsManager handles metrics collection and storage
type MetricsManager struct {
    mutex       sync.RWMutex
    metricsPath string
    current     *Metrics
    history     []*Metrics
}

var (
    metricsManager *MetricsManager
    metricsOnce    sync.Once
)

// initMetricsManager initializes the metrics manager
func initMetricsManager() error {
    var err error
    metricsOnce.Do(func() {
        metricsPath := filepath.Join(cfg.DataDir, "metrics")
        if err = os.MkdirAll(metricsPath, 0755); err != nil {
            return
        }

        metricsManager = &MetricsManager{
            metricsPath: metricsPath,
            current: &Metrics{
                Timestamp:     time.Now(),
                CategoryStats: make(map[string]int),
                SourceStats:   make(map[string]int),
            },
            history: make([]*Metrics, 0),
        }

        // Load historical metrics
        if err = metricsManager.loadHistory(); err != nil {
            Logger().Printf("Failed to load metrics history: %v", err)
        }
    })
    return err
}

// CollectMetrics gathers current metrics
func CollectMetrics(ctx context.Context) error {
    if metricsManager == nil {
        if err := initMetricsManager(); err != nil {
            return fmt.Errorf("failed to initialize metrics manager: %v", err)
        }
    }

    state, err := LoadState()
    if err != nil {
        return fmt.Errorf("failed to load state for metrics: %v", err)
    }

    sources, err := LoadSources()
    if err != nil {
        return fmt.Errorf("failed to load sources for metrics: %v", err)
    }

    metrics := &Metrics{
        Timestamp:     time.Now().UTC(),
        ArticleCount:  state.ArticleCount,
        ErrorCount:    state.ErrorCount,
        SourceCount:   len(sources),
        UpTime:        time.Since(state.StartupTime),
        CategoryStats: make(map[string]int),
        SourceStats:   make(map[string]int),
    }

    // Collect category and source stats
    for _, article := range state.RecentArticles {
        metrics.CategoryStats[article.Category]++
        metrics.SourceStats[article.Source]++

        // Collect reliability stats
        if article.FactCheckResult != nil {
            switch article.FactCheckResult.ReliabilityTier {
            case "High":
                metrics.Reliability.HighCount++
            case "Medium":
                metrics.Reliability.MediumCount++
            case "Low":
                metrics.Reliability.LowCount++
            }
            metrics.Reliability.AverageScore += article.FactCheckResult.Score
        }
    }

    // Calculate average reliability score
    totalChecked := metrics.Reliability.HighCount + metrics.Reliability.MediumCount + metrics.Reliability.LowCount
    if totalChecked > 0 {
        metrics.Reliability.AverageScore /= float64(totalChecked)
    }

    // Update current metrics
    metricsManager.mutex.Lock()
    metricsManager.current = metrics
    metricsManager.history = append(metricsManager.history, metrics)
    metricsManager.mutex.Unlock()

    // Save metrics
    if err := metricsManager.saveMetrics(metrics); err != nil {
        return fmt.Errorf("failed to save metrics: %v", err)
    }

    return nil
}

// GetCurrentMetrics returns the most recent metrics
func GetCurrentMetrics() *Metrics {
    metricsManager.mutex.RLock()
    defer metricsManager.mutex.RUnlock()
    return metricsManager.current
}

// GetMetricsHistory returns historical metrics for the specified duration
func GetMetricsHistory(duration time.Duration) []*Metrics {
    metricsManager.mutex.RLock()
    defer metricsManager.mutex.RUnlock()

    cutoff := time.Now().Add(-duration)
    var filtered []*Metrics

    for _, m := range metricsManager.history {
        if m.Timestamp.After(cutoff) {
            filtered = append(filtered, m)
        }
    }

    return filtered
}

// saveMetrics saves metrics to disk
func (mm *MetricsManager) saveMetrics(metrics *Metrics) error {
    filename := filepath.Join(mm.metricsPath, 
        fmt.Sprintf("metrics_%s.json", metrics.Timestamp.Format("2006-01-02")))

    data, err := json.MarshalIndent(metrics, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal metrics: %v", err)
    }

    if err := os.WriteFile(filename, data, 0644); err != nil {
        return fmt.Errorf("failed to write metrics file: %v", err)
    }

    return nil
}

// loadHistory loads historical metrics from disk
func (mm *MetricsManager) loadHistory() error {
    files, err := os.ReadDir(mm.metricsPath)
    if err != nil {
        return fmt.Errorf("failed to read metrics directory: %v", err)
    }

    for _, file := range files {
        if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
            path := filepath.Join(mm.metricsPath, file.Name())
            data, err := os.ReadFile(path)
            if err != nil {
                Logger().Printf("Failed to read metrics file %s: %v", path, err)
                continue
            }

            var metrics Metrics
            if err := json.Unmarshal(data, &metrics); err != nil {
                Logger().Printf("Failed to unmarshal metrics from %s: %v", path, err)
                continue
            }

            mm.history = append(mm.history, &metrics)
        }
    }

    // Sort history by timestamp
    sort.Slice(mm.history, func(i, j int) bool {
        return mm.history[i].Timestamp.Before(mm.history[j].Timestamp)
    })

    return nil
}
