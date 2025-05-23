// cmd/sankarea/dashboard.go
package main

import (
    "embed"
    "encoding/json"
    "fmt"
    "html/template"
    "net/http"
    "path"
    "sync"
    "time"
)

//go:embed templates/*
var templateFS embed.FS

var (
    dashboardMutex sync.RWMutex
    templates      *template.Template
)

// DashboardData represents the data displayed on the dashboard
type DashboardData struct {
    Version        string                   `json:"version"`
    Uptime         string                   `json:"uptime"`
    Status         string                   `json:"status"`
    Sources        []Source                 `json:"sources"`
    RecentArticles []ArticleDigest          `json:"recentArticles"`
    Stats          map[string]interface{}   `json:"stats"`
    Metrics        Metrics                  `json:"metrics"`
    Errors         []ErrorEvent            `json:"errors"`
    Config         map[string]interface{}   `json:"config"`
}

// StartDashboard initializes and starts the dashboard server
func StartDashboard() error {
    // Load templates
    var err error
    templates, err = template.ParseFS(templateFS, "templates/*.html")
    if err != nil {
        return fmt.Errorf("failed to parse templates: %v", err)
    }

    // Set up routes
    http.HandleFunc("/", handleDashboardHome)
    http.HandleFunc("/api/status", handleDashboardStatus)
    http.HandleFunc("/api/sources", handleDashboardSources)
    http.HandleFunc("/api/articles", handleDashboardArticles)
    http.HandleFunc("/api/metrics", handleDashboardMetrics)
    http.HandleFunc("/api/health", handleDashboardHealthCheck)
    http.HandleFunc("/api/config", handleDashboardConfig)

    // Start server
    go func() {
        addr := fmt.Sprintf(":%d", cfg.DashboardPort)
        Logger().Printf("Starting dashboard server on %s", addr)
        if err := http.ListenAndServe(addr, nil); err != nil {
            Logger().Printf("Dashboard server error: %v", err)
        }
    }()

    return nil
}

// handleDashboardHome renders the main dashboard page
func handleDashboardHome(w http.ResponseWriter, r *http.Request) {
    dashboardMutex.RLock()
    defer dashboardMutex.RUnlock()

    data := getDashboardData()
    
    if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

// handleDashboardStatus returns the current system status
func handleDashboardStatus(w http.ResponseWriter, r *http.Request) {
    dashboardMutex.RLock()
    currentState := *state
    dashboardMutex.RUnlock()

    status := map[string]interface{}{
        "version":      cfg.Version,
        "status":       getStatusString(&currentState),
        "uptime":       FormatDuration(time.Since(currentState.StartupTime)),
        "lastFetch":    currentState.LastFetchTime,
        "lastDigest":   currentState.LastDigest,
        "feedCount":    currentState.FeedCount,
        "errorCount":   currentState.ErrorCount,
        "paused":       currentState.Paused,
        "pausedBy":     currentState.PausedBy,
        "lockdown":     currentState.Lockdown,
        "lastError":    currentState.LastError,
        "lastErrorTime": currentState.LastErrorTime,
    }

    respondWithJSON(w, http.StatusOK, status)
}

// handleDashboardSources returns the list of news sources
func handleDashboardSources(w http.ResponseWriter, r *http.Request) {
    sources := loadSources()
    respondWithJSON(w, http.StatusOK, sources)
}

// handleDashboardArticles returns recent articles
func handleDashboardArticles(w http.ResponseWriter, r *http.Request) {
    articles, err := getRecentArticles(20) // Get last 20 articles
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch articles: %v", err))
        return
    }
    respondWithJSON(w, http.StatusOK, articles)
}

// handleDashboardMetrics returns system metrics
func handleDashboardMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := collectMetrics()
    respondWithJSON(w, http.StatusOK, metrics)
}

// handleDashboardHealthCheck returns detailed health status
func handleDashboardHealthCheck(w http.ResponseWriter, r *http.Request) {
    health := GetHealthStatus()
    respondWithJSON(w, http.StatusOK, health)
}

// handleDashboardConfig returns sanitized configuration
func handleDashboardConfig(w http.ResponseWriter, r *http.Request) {
    // Create a sanitized version of config (removing sensitive data)
    safeConfig := map[string]interface{}{
        "version":               cfg.Version,
        "newsIntervalMinutes":   cfg.NewsIntervalMinutes,
        "maxPostsPerSource":     cfg.MaxPostsPerSource,
        "enableImageEmbed":      cfg.EnableImageEmbed,
        "enableFactCheck":       cfg.EnableFactCheck,
        "enableSummarization":   cfg.EnableSummarization,
        "enableContentFiltering": cfg.EnableContentFiltering,
        "enableKeywordTracking": cfg.EnableKeywordTracking,
        "enableDatabase":        cfg.EnableDatabase,
        "enableDashboard":       cfg.EnableDashboard,
        "fetchNewsOnStartup":    cfg.FetchNewsOnStartup,
    }

    respondWithJSON(w, http.StatusOK, safeConfig)
}

// getDashboardData collects all data needed for the dashboard
func getDashboardData() DashboardData {
    mutex.RLock()
    currentState := *state
    mutex.RUnlock()

    return DashboardData{
        Version: cfg.Version,
        Uptime:  FormatDuration(time.Since(currentState.StartupTime)),
        Status:  getStatusString(&currentState),
        Sources: loadSources(),
        RecentArticles: getRecentArticlesDigest(),
        Stats:   getSystemStats(),
        Metrics: collectMetrics(),
        Errors:  getRecentErrors(),
        Config:  getSafeConfig(),
    }
}

// getRecentArticlesDigest returns recent articles in digest format
func getRecentArticlesDigest() []ArticleDigest {
    articles, err := getRecentArticles(10)
    if err != nil {
        Logger().Printf("Failed to get recent articles: %v", err)
        return []ArticleDigest{}
    }
    return articles
}

// getSystemStats returns various system statistics
func getSystemStats() map[string]interface{} {
    mutex.RLock()
    currentState := *state
    mutex.RUnlock()

    return map[string]interface{}{
        "totalArticles":   currentState.TotalArticles,
        "totalErrors":     currentState.TotalErrors,
        "totalAPICalls":   currentState.TotalAPICalls,
        "feedCount":       currentState.FeedCount,
        "digestCount":     currentState.DigestCount,
        "errorCount":      currentState.ErrorCount,
        "lastInterval":    currentState.LastInterval,
        "lastFetchTime":   currentState.LastFetchTime,
        "lastDigestTime":  currentState.LastDigest,
    }
}

// getRecentErrors returns recent error events
func getRecentErrors() []ErrorEvent {
    healthMonitor.mutex.Lock()
    defer healthMonitor.mutex.Unlock()

    return append([]ErrorEvent{}, healthMonitor.errorLog...)
}

// getSafeConfig returns a sanitized version of the configuration
func getSafeConfig() map[string]interface{} {
    return map[string]interface{}{
        "version":               cfg.Version,
        "newsIntervalMinutes":   cfg.NewsIntervalMinutes,
        "maxPostsPerSource":     cfg.MaxPostsPerSource,
        "enableImageEmbed":      cfg.EnableImageEmbed,
        "enableFactCheck":       cfg.EnableFactCheck,
        "enableSummarization":   cfg.EnableSummarization,
        "enableContentFiltering": cfg.EnableContentFiltering,
        "enableKeywordTracking": cfg.EnableKeywordTracking,
        "enableDatabase":        cfg.EnableDatabase,
        "enableDashboard":       cfg.EnableDashboard,
        "fetchNewsOnStartup":    cfg.FetchNewsOnStartup,
    }
}
