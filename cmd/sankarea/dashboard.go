// cmd/sankarea/dashboard.go
package main

import (
    "embed"
    "encoding/json"
    "fmt"
    "html/template"
    "net/http"
    "sync"
    "time"
)

//go:embed templates/*
var dashboardTemplates embed.FS

// Dashboard handles the web interface for monitoring
type Dashboard struct {
    server     *http.Server
    mutex      sync.RWMutex
    templates  *template.Template
    metrics    *Metrics
    lastUpdate time.Time
}

// DashboardData represents the data passed to dashboard templates
type DashboardData struct {
    Metrics      *Metrics
    Version      string
    BuildDate    string
    BuildTime    string
    LastUpdate   string
    HealthStatus string
}

var (
    dashboard     *Dashboard
    dashboardOnce sync.Once
)

// initDashboard initializes the dashboard server
func initDashboard() error {
    var err error
    dashboardOnce.Do(func() {
        // Parse templates
        tmpl, err := template.ParseFS(dashboardTemplates, "templates/*.html")
        if err != nil {
            err = fmt.Errorf("failed to parse dashboard templates: %v", err)
            return
        }

        dashboard = &Dashboard{
            templates: tmpl,
            metrics:   GetCurrentMetrics(),
        }

        // Initialize HTTP server
        mux := http.NewServeMux()
        mux.HandleFunc("/", dashboard.handleIndex)
        mux.HandleFunc("/api/metrics", dashboard.handleMetrics)
        mux.HandleFunc("/api/sources", dashboard.handleSources)
        mux.HandleFunc("/api/health", dashboard.handleHealth)

        dashboard.server = &http.Server{
            Addr:         fmt.Sprintf(":%d", cfg.DashboardPort),
            Handler:      mux,
            ReadTimeout:  10 * time.Second,
            WriteTimeout: 10 * time.Second,
        }
    })
    return err
}

// Start starts the dashboard server
func (d *Dashboard) Start() error {
    Logger().Printf("Starting dashboard on port %d", cfg.DashboardPort)
    return d.server.ListenAndServe()
}

// Stop gracefully stops the dashboard server
func (d *Dashboard) Stop() error {
    return d.server.Close()
}

// UpdateMetrics updates the dashboard metrics
func (d *Dashboard) UpdateMetrics(metrics *Metrics) error {
    d.mutex.Lock()
    defer d.mutex.Unlock()

    d.metrics = metrics
    d.lastUpdate = time.Now()
    return nil
}

// HTTP Handlers

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
    d.mutex.RLock()
    data := DashboardData{
        Metrics:      d.metrics,
        Version:      botVersion,
        BuildDate:    buildDate,
        BuildTime:    buildTime,
        LastUpdate:   d.lastUpdate.Format(time.RFC3339),
        HealthStatus: getHealthStatus(),
    }
    d.mutex.RUnlock()

    if err := d.templates.ExecuteTemplate(w, "index.html", data); err != nil {
        http.Error(w, "Failed to render template", http.StatusInternalServerError)
        Logger().Printf("Failed to render dashboard template: %v", err)
    }
}

func (d *Dashboard) handleMetrics(w http.ResponseWriter, r *http.Request) {
    d.mutex.RLock()
    metrics := d.metrics
    d.mutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(metrics); err != nil {
        http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
        Logger().Printf("Failed to encode metrics: %v", err)
    }
}

func (d *Dashboard) handleSources(w http.ResponseWriter, r *http.Request) {
    sources, err := LoadSources()
    if err != nil {
        http.Error(w, "Failed to load sources", http.StatusInternalServerError)
        Logger().Printf("Failed to load sources: %v", err)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(sources); err != nil {
        http.Error(w, "Failed to encode sources", http.StatusInternalServerError)
        Logger().Printf("Failed to encode sources: %v", err)
    }
}

func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
    state, err := LoadState()
    if err != nil {
        http.Error(w, "Failed to load state", http.StatusInternalServerError)
        Logger().Printf("Failed to load state: %v", err)
        return
    }

    health := struct {
        Status    string    `json:"status"`
        Timestamp time.Time `json:"timestamp"`
        Uptime    string    `json:"uptime"`
    }{
        Status:    state.HealthStatus,
        Timestamp: time.Now(),
        Uptime:    time.Since(state.StartupTime).String(),
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(health); err != nil {
        http.Error(w, "Failed to encode health status", http.StatusInternalServerError)
        Logger().Printf("Failed to encode health status: %v", err)
    }
}

func getHealthStatus() string {
    state, err := LoadState()
    if err != nil {
        return StatusUnhealthy
    }
    return state.HealthStatus
}
