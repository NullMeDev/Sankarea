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

// DashboardServer handles the web dashboard
type DashboardServer struct {
    templates  *template.Template
    mux       *http.ServeMux
    mutex     sync.RWMutex
    server    *http.Server
}

// NewDashboardServer creates a new dashboard server instance
func NewDashboardServer() *DashboardServer {
    return &DashboardServer{
        mux: http.NewServeMux(),
    }
}

// Initialize sets up the dashboard server
func (ds *DashboardServer) Initialize() error {
    // Load templates
    var err error
    ds.templates, err = template.ParseFS(templateFS, "templates/*.html")
    if err != nil {
        return fmt.Errorf("failed to parse templates: %v", err)
    }

    // Set up middleware
    ds.setupRoutes()

    // Configure server
    ds.server = &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.DashboardPort),
        Handler:      ds.loggingMiddleware(ds.mux),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    return nil
}

// Start begins serving the dashboard
func (ds *DashboardServer) Start() error {
    go func() {
        Logger().Printf("Starting dashboard server on port %d", cfg.DashboardPort)
        if err := ds.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            Logger().Printf("Dashboard server error: %v", err)
        }
    }()
    return nil
}

// setupRoutes configures all dashboard routes
func (ds *DashboardServer) setupRoutes() {
    // Static file handler
    ds.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(templateFS))))

    // API routes
    ds.mux.HandleFunc("/api/status", ds.handleAPIStatus)
    ds.mux.HandleFunc("/api/sources", ds.handleAPISources)
    ds.mux.HandleFunc("/api/articles", ds.handleAPIArticles)
    ds.mux.HandleFunc("/api/metrics", ds.handleAPIMetrics)
    ds.mux.HandleFunc("/api/health", ds.handleAPIHealth)
    ds.mux.HandleFunc("/api/config", ds.handleAPIConfig)

    // Page routes
    ds.mux.HandleFunc("/", ds.handleHome)
    ds.mux.HandleFunc("/logs", ds.handleLogs)
    ds.mux.HandleFunc("/sources", ds.handleSources)
    ds.mux.HandleFunc("/articles", ds.handleArticles)
    ds.mux.HandleFunc("/settings", ds.handleSettings)
}

// Middleware for logging requests
func (ds *DashboardServer) loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        Logger().Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
    })
}

// Page handlers
func (ds *DashboardServer) handleHome(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    ds.renderTemplate(w, "dashboard.html", ds.getDashboardData())
}

func (ds *DashboardServer) handleLogs(w http.ResponseWriter, r *http.Request) {
    ds.renderTemplate(w, "logs.html", ds.getDashboardData())
}

func (ds *DashboardServer) handleSources(w http.ResponseWriter, r *http.Request) {
    ds.renderTemplate(w, "sources.html", ds.getDashboardData())
}

func (ds *DashboardServer) handleArticles(w http.ResponseWriter, r *http.Request) {
    ds.renderTemplate(w, "articles.html", ds.getDashboardData())
}

func (ds *DashboardServer) handleSettings(w http.ResponseWriter, r *http.Request) {
    ds.renderTemplate(w, "settings.html", ds.getDashboardData())
}

// API handlers
func (ds *DashboardServer) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
    state := GetState()
    
    status := map[string]interface{}{
        "version":       cfg.Version,
        "status":        getStatusString(state),
        "uptime":        FormatDuration(time.Since(state.StartupTime)),
        "lastFetch":     state.LastFetchTime,
        "lastDigest":    state.LastDigest,
        "feedCount":     state.FeedCount,
        "errorCount":    state.ErrorCount,
        "paused":        state.Paused,
        "pausedBy":      state.PausedBy,
        "lockdown":      state.Lockdown,
        "lastError":     state.LastError,
        "lastErrorTime": state.LastErrorTime,
    }

    ds.respondWithJSON(w, http.StatusOK, status)
}

func (ds *DashboardServer) handleAPISources(w http.ResponseWriter, r *http.Request) {
    sources := loadSources()
    ds.respondWithJSON(w, http.StatusOK, sources)
}

func (ds *DashboardServer) handleAPIArticles(w http.ResponseWriter, r *http.Request) {
    articles, err := getRecentArticles(20)
    if err != nil {
        ds.respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch articles: %v", err))
        return
    }
    ds.respondWithJSON(w, http.StatusOK, articles)
}

func (ds *DashboardServer) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := collectMetrics()
    ds.respondWithJSON(w, http.StatusOK, metrics)
}

func (ds *DashboardServer) handleAPIHealth(w http.ResponseWriter, r *http.Request) {
    health := healthMonitor.GetHealthStatus()
    ds.respondWithJSON(w, http.StatusOK, health)
}

func (ds *DashboardServer) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
    ds.respondWithJSON(w, http.StatusOK, ds.getSafeConfig())
}

// Helper methods
func (ds *DashboardServer) renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
    ds.mutex.RLock()
    defer ds.mutex.RUnlock()

    if err := ds.templates.ExecuteTemplate(w, tmpl, data); err != nil {
        http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
    }
}

func (ds *DashboardServer) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    response, err := json.Marshal(payload)
    if err != nil {
        http.Error(w, "Failed to marshal JSON response", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
}

func (ds *DashboardServer) respondWithError(w http.ResponseWriter, code int, message string) {
    ds.respondWithJSON(w, code, map[string]string{"error": message})
}

func (ds *DashboardServer) getDashboardData() map[string]interface{} {
    state := GetState()
    
    return map[string]interface{}{
        "Version":        cfg.Version,
        "Uptime":        FormatDuration(time.Since(state.StartupTime)),
        "Status":        getStatusString(state),
        "Sources":       loadSources(),
        "RecentArticles": getRecentArticlesDigest(),
        "Stats":         getSystemStats(),
        "Metrics":       collectMetrics(),
        "Errors":        healthMonitor.GetRecentErrors(10),
        "Config":        ds.getSafeConfig(),
    }
}

func (ds *DashboardServer) getSafeConfig() map[string]interface{} {
    return map[string]interface{}{
        "version":               cfg.Version,
        "newsIntervalMinutes":   cfg.NewsIntervalMinutes,
        "maxPostsPerSource":     cfg.MaxPostsPerSource,
        "enableImageEmbed":      cfg.EnableImageEmbed,
        "enableFactCheck":       cfg.EnableFactCheck,
        "enableSummarization":   cfg.EnableSummarization,
        "enableContentFiltering": cfg.EnableContentFiltering,
        "enableKeywordTracking":  cfg.EnableKeywordTracking,
        "enableDatabase":        cfg.EnableDatabase,
        "enableDashboard":       cfg.EnableDashboard,
        "fetchNewsOnStartup":    cfg.FetchNewsOnStartup,
    }
}
