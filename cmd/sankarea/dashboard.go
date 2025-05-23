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

    // Create router and set up middleware
    mux := http.NewServeMux()
    
    // Static file handler
    mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(templateFS))))

    // API routes
    mux.HandleFunc("/api/status", handleDashboardStatus)
    mux.HandleFunc("/api/sources", handleDashboardSources)
    mux.HandleFunc("/api/articles", handleDashboardArticles)
    mux.HandleFunc("/api/metrics", handleDashboardMetrics)
    mux.HandleFunc("/api/health", handleDashboardHealthCheck)
    mux.HandleFunc("/api/config", handleDashboardConfig)

    // Page routes
    mux.HandleFunc("/", handleDashboardHome)
    mux.HandleFunc("/logs", handleDashboardLogs)
    mux.HandleFunc("/sources", handleDashboardSourcesPage)
    mux.HandleFunc("/articles", handleDashboardArticlesPage)
    mux.HandleFunc("/settings", handleDashboardSettingsPage)

    // Start server
    addr := fmt.Sprintf(":%d", cfg.DashboardPort)
    server := &http.Server{
        Addr:         addr,
        Handler:      loggingMiddleware(mux),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        Logger().Printf("Starting dashboard server on %s", addr)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            Logger().Printf("Dashboard server error: %v", err)
        }
    }()

    return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        Logger().Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
    })
}

func handleDashboardHome(w http.ResponseWriter, r *http.Request) {
    dashboardMutex.RLock()
    defer dashboardMutex.RUnlock()

    data := getDashboardData()
    if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

func handleDashboardLogs(w http.ResponseWriter, r *http.Request) {
    data := getDashboardData()
    if err := templates.ExecuteTemplate(w, "logs.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

func handleDashboardSourcesPage(w http.ResponseWriter, r *http.Request) {
    data := getDashboardData()
    if err := templates.ExecuteTemplate(w, "sources.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

func handleDashboardArticlesPage(w http.ResponseWriter, r *http.Request) {
    data := getDashboardData()
    if err := templates.ExecuteTemplate(w, "articles.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

func handleDashboardSettingsPage(w http.ResponseWriter, r *http.Request) {
    data := getDashboardData()
    if err := templates.ExecuteTemplate(w, "settings.html", data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
        return
    }
}

func handleDashboardStatus(w http.ResponseWriter, r *http.Request) {
    mutex.RLock()
    currentState := *state
    mutex.RUnlock()

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

func handleDashboardSources(w http.ResponseWriter, r *http.Request) {
    sources := loadSources()
    respondWithJSON(w, http.StatusOK, sources)
}

func handleDashboardArticles(w http.ResponseWriter, r *http.Request) {
    articles, err := getRecentArticles(20)
    if err != nil {
        respondWithHTTPError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch articles: %v", err))
        return
    }
    respondWithJSON(w, http.StatusOK, articles)
}

func handleDashboardMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := collectMetrics()
    respondWithJSON(w, http.StatusOK, metrics)
}

func handleDashboardHealthCheck(w http.ResponseWriter, r *http.Request) {
    health := GetHealthStatus()
    respondWithJSON(w, http.StatusOK, health)
}

func handleDashboardConfig(w http.ResponseWriter, r *http.Request) {
    safeConfig := getSafeConfig()
    respondWithJSON(w, http.StatusOK, safeConfig)
}

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
