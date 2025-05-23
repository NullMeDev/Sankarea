package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Dashboard system constants
const (
	DashboardPort         = 8080
	DashboardTemplatesDir = "dashboard/templates"
	DashboardStaticDir    = "dashboard/static"
)

// dashboardData holds data for the dashboard
type dashboardData struct {
	Sources       []Source     `json:"sources"`
	Config        *Config      `json:"config"`
	State         *State       `json:"state"`
	Errors        []*ErrorEvent `json:"errors"`
	SystemMetrics Metrics      `json:"metrics"`
	Version       string       `json:"version"`
	UpSince       time.Time    `json:"upSince"`
}

var (
	dashboardRouter *mux.Router
	wsUpgrader     = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections in development
		},
	}
	wsClients = make(map[*websocket.Conn]bool)
)

// StartDashboard initializes and starts the admin dashboard
func StartDashboard() {
	// Create router
	dashboardRouter = mux.NewRouter()

	// Set up routes
	dashboardRouter.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir(DashboardStaticDir))))

	// API routes
	api := dashboardRouter.PathPrefix("/api").Subrouter()
	api.HandleFunc("/status", apiGetStatus).Methods("GET")
	api.HandleFunc("/sources", apiGetSources).Methods("GET")
	api.HandleFunc("/sources", apiAddSource).Methods("POST")
	api.HandleFunc("/sources/{name}", apiUpdateSource).Methods("PUT")
	api.HandleFunc("/sources/{name}", apiDeleteSource).Methods("DELETE")
	api.HandleFunc("/config", apiGetConfig).Methods("GET")
	api.HandleFunc("/config", apiUpdateConfig).Methods("PUT")
	api.HandleFunc("/admin/refresh", apiTriggerRefresh).Methods("POST")
	api.HandleFunc("/admin/pause", apiTogglePause).Methods("POST")
	api.HandleFunc("/logs", apiGetLogs).Methods("GET")
	api.HandleFunc("/metrics", apiGetMetrics).Methods("GET")
	api.HandleFunc("/ws", handleWebsocket)

	// Frontend routes
	dashboardRouter.HandleFunc("/", handleDashboardHome)
	dashboardRouter.HandleFunc("/sources", handleDashboardSources)
	dashboardRouter.HandleFunc("/config", handleDashboardConfig)
	dashboardRouter.HandleFunc("/logs", handleDashboardLogs)
	dashboardRouter.HandleFunc("/healthcheck", handleHealthCheck)

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", DashboardPort)
		Logger().Printf("Starting admin dashboard on http://localhost%s", addr)
		err := http.ListenAndServe(addr, dashboardRouter)
		if err != nil {
			Logger().Printf("Dashboard server failed: %v", err)
		}
	}()
}

// handleDashboardHome renders the dashboard home page
func handleDashboardHome(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.html", getDashboardData())
}

// handleDashboardSources renders the sources management page
func handleDashboardSources(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "sources.html", getDashboardData())
}

// handleDashboardConfig renders the configuration management page
func handleDashboardConfig(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "config.html", getDashboardData())
}

// handleDashboardLogs renders the logs page
func handleDashboardLogs(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "logs.html", getDashboardData())
}

// handleHealthCheck provides a simple health check endpoint
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	state, err := LoadState()
	if err != nil {
		respondWithJSON(w, http.StatusOK, map[string]string{
			"status": "degraded",
			"error":  "Failed to load state",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  getStatusString(state),
		"version": cfg.Version,
		"uptime":  time.Since(state.StartupTime).String(),
	})
}

// renderTemplate renders a dashboard template with data
func renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	// Check if template exists
	templatePath := filepath.Join(DashboardTemplatesDir, templateName)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	// Parse template
	tmpl, err := template.ParseFiles(
		filepath.Join(DashboardTemplatesDir, "layout.html"),
		templatePath,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing template: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute template
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
	}
}

// getDashboardData gathers data for the dashboard
func getDashboardData() dashboardData {
	sources, err := LoadSources()
	if err != nil {
		Logger().Printf("Error loading sources for dashboard: %v", err)
		sources = []Source{}
	}

	state, err := LoadState()
	if err != nil {
		Logger().Printf("Error loading state for dashboard: %v", err)
		state = &State{
			StartupTime: time.Now(),
			Version:     cfg.Version,
		}
	}

	// Get recent errors
	var errors []*ErrorEvent
	if errorBuffer != nil {
		errors = errorBuffer.GetRecent(10)
	}

	return dashboardData{
		Sources:       sources,
		Config:        cfg,
		State:         state,
		Errors:        errors,
		SystemMetrics: collectMetrics(),
		Version:       cfg.Version,
		UpSince:       state.StartupTime,
	}
}

// apiGetStatus returns the current status as JSON
func apiGetStatus(w http.ResponseWriter, r *http.Request) {
	state, err := LoadState()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load state")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":      getStatusString(state),
		"version":     cfg.Version,
		"uptime":      time.Since(state.StartupTime).String(),
		"paused":      state.Paused,
		"error_count": state.ErrorCount,
		"next_update": state.NewsNextTime,
		"last_digest": state.LastDigest,
	})
}

// apiGetSources returns all sources as JSON
func apiGetSources(w http.ResponseWriter, r *http.Request) {
	sources, err := LoadSources()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load sources")
		return
	}

	respondWithJSON(w, http.StatusOK, sources)
}

// apiAddSource adds a new source
func apiAddSource(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var source Source
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&source); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Validate source
	if source.Name == "" || source.URL == "" {
		respondWithError(w, http.StatusBadRequest, "Name and URL are required")
		return
	}

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load sources")
		return
	}

	// Check if source already exists
	for _, src := range sources {
		if strings.EqualFold(src.Name, source.Name) {
			respondWithError(w, http.StatusConflict, "Source with that name already exists")
			return
		}
	}

	// Add source
	sources = append(sources, source)
	if err := SaveSources(sources); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save sources")
		return
	}

	// Notify websocket clients of update
	notifyWebSocketClients("source_added", source)

	respondWithJSON(w, http.StatusCreated, source)
}

// apiUpdateSource updates an existing source
func apiUpdateSource(w http.ResponseWriter, r *http.Request) {
	// Get source name from URL
	vars := mux.Vars(r)
	name := vars["name"]

	// Parse request body
	var updatedSource Source
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updatedSource); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load sources")
		return
	}

	// Find and update source
	found := false
	for i, src := range sources {
		if strings.EqualFold(src.Name, name) {
			updatedSource.Name = name // Preserve original name
			sources[i] = updatedSource
			found = true
			break
		}
	}

	if !found {
		respondWithError(w, http.StatusNotFound, "Source not found")
		return
	}

	// Save updated sources
	if err := SaveSources(sources); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save sources")
		return
	}

	// Notify websocket clients of update
	notifyWebSocketClients("source_updated", updatedSource)

	respondWithJSON(w, http.StatusOK, updatedSource)
}

// apiDeleteSource deletes a source
func apiDeleteSource(w http.ResponseWriter, r *http.Request) {
	// Get source name from URL
	vars := mux.Vars(r)
	name := vars["name"]

	// Load existing sources
	sources, err := LoadSources()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load sources")
		return
	}

	// Find and remove source
	found := false
	var deletedSource Source
	for i, src := range sources {
		if strings.EqualFold(src.Name, name) {
			deletedSource = src
			sources = append(sources[:i], sources[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		respondWithError(w, http.StatusNotFound, "Source not found")
		return
	}

	// Save updated sources
	if err := SaveSources(sources); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save sources")
		return
	}

	// Notify websocket clients of update
	notifyWebSocketClients("source_deleted", map[string]string{"name": name})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Source deleted successfully"})
}

// apiGetConfig returns the current configuration
func apiGetConfig(w http.ResponseWriter, r *http.Request) {
	// Remove sensitive data from config
	configCopy := *cfg
	configCopy.BotToken = "[REDACTED]"
	configCopy.OpenAIAPIKey = "[REDACTED]"
	configCopy.GoogleFactCheckAPIKey = "[REDACTED]"
	configCopy.ClaimBustersAPIKey = "[REDACTED]"

	respondWithJSON(w, http.StatusOK, configCopy)
}

// apiUpdateConfig updates the configuration
func apiUpdateConfig(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var updatedConfig Config
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&updatedConfig); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Don't allow updating sensitive fields if they contain placeholder values
	if updatedConfig.BotToken == "[REDACTED]" {
		updatedConfig.BotToken = cfg.BotToken
	}
	if updatedConfig.OpenAIAPIKey == "[REDACTED]" {
		updatedConfig.OpenAIAPIKey = cfg.OpenAIAPIKey
	}
	if updatedConfig.GoogleFactCheckAPIKey == "[REDACTED]" {
		updatedConfig.GoogleFactCheckAPIKey = cfg.GoogleFactCheckAPIKey
	}
	if updatedConfig.ClaimBustersAPIKey == "[REDACTED]" {
		updatedConfig.ClaimBustersAPIKey = cfg.ClaimBustersAPIKey
	}

	// Save updated config
	if err := SaveConfig(&updatedConfig); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}

	// Update global config
	cfg = &updatedConfig

	// Notify websocket clients of update
	notifyWebSocketClients("config_updated", map[string]string{"message": "Configuration updated"})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Configuration updated successfully"})
}

// apiTriggerRefresh triggers a manual refresh of news sources
func apiTriggerRefresh(w http.ResponseWriter, r *http.Request) {
	sources, err := LoadSources()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load sources")
		return
	}

	// Trigger refresh in background
	go forceFetchAllSources(dg, sources)

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Refresh triggered successfully"})
}

// apiTogglePause toggles the paused state
func apiTogglePause(w http.ResponseWriter, r *http.Request) {
	state, err := LoadState()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load state")
		return
	}

	// Parse request body to get pause status
	var request struct {
		Paused bool `json:"paused"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Update state
	state.Paused = request.Paused
	if err := SaveState(state); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save state")
		return
	}

	// Notify websocket clients of update
	notifyWebSocketClients("state_updated", map[string]interface{}{
		"paused": state.Paused,
	})

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"paused":  state.Paused,
		"message": fmt.Sprintf("System %s", getStatusText(state.Paused)),
	})
}

// apiGetLogs returns recent log entries
func apiGetLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // Default limit

	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Get log file path
	timestamp := time.Now().Format("2006-01-02")
	logFilePath := filepath.Join("logs", "sankarea-"+timestamp+".log")

	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		respondWithError(w, http.StatusNotFound, "Log file not found")
		return
	}

	// Open log file
	file, err := os.Open(logFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to open log file")
		return
	}
	defer file.Close()

	// Read log file (this is inefficient for large files, but simple)
	content, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read log file")
		return
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")

	// Get last N lines (with safety check for empty slices)
	lastLines := []string{}
	if len(lines) > 0 {
		if limit > len(lines) {
			limit = len(lines)
		}
		lastLines = lines[len(lines)-limit:]
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"logs":      lastLines,
		"file_path": logFilePath,
		"count":     len(lastLines),
	})
}

// apiGetMetrics returns system metrics
func apiGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := collectMetrics()
	respondWithJSON(w, http.StatusOK, metrics)
}

// handleWebsocket handles WebSocket connections for real-time updates
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		Logger().Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	wsClients[conn] = true
	defer delete(wsClients, conn)

	// Send initial data
	data := getDashboardData()
	initData, err := json.Marshal(map[string]interface{}{
		"type": "init",
		"data": data,
	})
	if err != nil {
		Logger().Printf("Error marshaling init data: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, initData); err != nil {
		Logger().Printf("Error sending init data: %v", err)
		return
	}

	// Handle incoming messages
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			break // Client disconnected
		}

		if messageType == websocket.PingMessage {
			conn.WriteMessage(websocket.PongMessage, nil)
		}
	}
}

// notifyWebSocketClients sends a message to all connected WebSocket clients
func notifyWebSocketClients(eventType string, data interface{}) {
	message := map[string]interface{}{
		"type": eventType,
		"data": data,
		"time": time.Now().Format(time.RFC3339),
	}
	
	messageJSON, err := json.Marshal(message)
	if err != nil {
		Logger().Printf("Error marshaling websocket message: %v", err)
		return
	}

	for client := range wsClients {
		err := client.WriteMessage(websocket.TextMessage, messageJSON)
		if err != nil {
			Logger().Printf("Error sending to websocket client: %v", err)
			client.Close()
			delete(wsClients, client)
		}
	}
}

// respondWithError sends a JSON error response
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Failed to marshal JSON response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// getStatusString returns a string representation of the current status
func getStatusString(state *State) string {
	if state.Paused {
		return "paused"
	}
	if state.ErrorCount > 0 {
		return "warning"
	}
	return "running"
}

// getStatusText returns a text representation of pause status
func getStatusText(paused bool) string {
	if paused {
		return "paused"
	}
	return "running"
}

// Create basic dashboard templates
func CreateDefaultDashboardTemplates() error {
	// Create directories
	if err := os.MkdirAll(DashboardTemplatesDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(DashboardStaticDir, 0755); err != nil {
		return err
	}

	// Create layout template
	layoutTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sankarea Admin Dashboard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.2.3/dist/css/bootstrap.min.css" rel="stylesheet">
    <link rel="stylesheet" href="/static/style.css">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <nav class="navbar navbar-expand-lg navbar-dark bg-dark">
        <div class="container-fluid">
            <a class="navbar-brand" href="/">Sankarea Dashboard</a>
            <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNav">
                <span class="navbar-toggler-icon"></span>
            </button>
            <div class="collapse navbar-collapse" id="navbarNav">
                <ul class="navbar-nav">
                    <li class="nav-item">
                        <a class="nav-link" href="/">Home</a>
                    </li>
                    <li class="nav-item">
                        <a class="nav-link" href="/sources">Sources</a>
                    </li>
                    <li class="nav-item">
                        <a class="nav-link" href="/config">Config</a>
                    </li>
                    <li class="nav-item">
                        <a class="nav-link" href="/logs">Logs</a>
                    </li>
                </ul>
            </div>
        </div>
    </nav>

    <div class="container mt-4">
        {{template "content" .}}
    </div>

    <footer class="footer mt-auto py-3 bg-light">
        <div class="container text-center">
            <span class="text-muted">Sankarea Bot v{{.Version}}</span>
        </div>
    </footer>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.2.3/dist/js/bootstrap.bundle.min.js"></script>
    <script src="/static/dashboard.js"></script>
</body>
</html>
`

	// Create index template
	indexTemplate := `
{{define "content"}}
<div class="row">
    <div class="col-md-12">
        <h1>Sankarea Dashboard</h1>
        <div class="status-indicator {{if eq .State.Paused true}}bg-warning{{else}}bg-success{{end}}">
            Status: {{if .State.Paused}}Paused{{else}}Running{{end}}
        </div>
    </div>
</div>

<div class="row mt-4">
    <div class="col-md-6">
        <div class="card">
            <div class="card-header">
                System Status
            </div>
            <div class="card-body">
                <p><strong>Version:</strong> {{.Version}}</p>
                <p><strong>Uptime:</strong> <span id="uptime"></span></p>
                <p><strong>Memory Usage:</strong> {{.SystemMetrics.MemoryUsageMB}} MB</p>
                <p><strong>Articles Processed:</strong> {{.State.TotalArticles}}</p>
                <p><strong>Error Count:</strong> {{.State.ErrorCount}}</p>
                <p><strong>Next Update:</strong> <span id="next-update" data-time="{{.State.NewsNextTime}}"></span></p>
                <div class="d-flex gap-2">
                    <button id="refresh-btn" class="btn btn-primary">Refresh News</button>
                    <button id="toggle-pause-btn" class="btn {{if .State.Paused}}btn-success{{else}}btn-warning{{end}}">
                        {{if .State.Paused}}Resume{{else}}Pause{{end}} News
                    </button>
                </div>
            </div>
        </div>
    </div>
    
    <div class="col-md-6">
        <div class="card">
            <div class="card-header">
                News Sources
            </div>
            <div class="card-body">
                <p><strong>Total Sources:</strong> {{len .Sources}}</p>
                <p><strong>Active Sources:</strong> <span id="active-sources"></span></p>
                <p><strong>Categories:</strong> <span id="categories"></span></p>
                <canvas id="sourcesChart" width="100" height="100"></canvas>
            </div>
        </div>
    </div>
</div>

<div class="row mt-4">
    <div class="col-md-12">
        <div class="card">
            <div class="card-header">
                Recent Errors
            </div>
            <div class="card-body">
                <div class="table-responsive">
                    <table class="table table-striped">
                        <thead>
                            <tr>
                                <th>Time</th>
                                <th>Component</th>
                                <th>Severity</th>
                                <th>Message</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Errors}}
                            <tr>
                                <td>{{.Time.Format "2006-01-02 15:04:05"}}</td>
                                <td>{{.Component}}</td>
                                <td>{{.Severity}}</td>
                                <td>{{.Message}}</td>
                            </tr>
                            {{else}}
                            <tr>
                                <td colspan="4" class="text-center">No recent errors</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    </div>
</div>

<script>
document.addEventListener('DOMContentLoaded', function() {
    // Calculate active sources
    let activeCount = 0;
    const categoriesSet = new Set();
    {{range .Sources}}
        {{if not .Paused}}
            activeCount++;
        {{end}}
        {{if .Category}}
            categoriesSet.add({{.Category | printf "%q"}});
        {{end}}
    {{end}}
    
    document.getElementById('active-sources').textContent = activeCount;
    document.getElementById('categories').textContent = Array.from(categoriesSet).join(", ") || "None";
    
    // Initialize chart
    const ctx = document.getElementById('sourcesChart').getContext('2d');
    new Chart(ctx, {
        type: 'pie',
        data: {
            labels: ['Active', 'Paused'],
            datasets: [{
                data: [activeCount, {{len .Sources}} - activeCount],
                backgroundColor: ['#28a745', '#ffc107']
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: {
                    position: 'bottom',
                }
            }
        }
    });
    
    // Update time displays
    function updateTimes() {
        const uptime = Math.floor((Date.now() - new Date("{{.UpSince}}").getTime()) / 1000);
        document.getElementById('uptime').textContent = formatDuration(uptime);
        
        const nextUpdate = document.getElementById('next-update');
        if (nextUpdate.dataset.time) {
            const timeUntil = Math.floor((new Date(nextUpdate.dataset.time).getTime() - Date.now()) / 1000);
            nextUpdate.textContent = timeUntil > 0 ? "in " + formatDuration(timeUntil) : "now";
        }
    }
    
    function formatDuration(seconds) {
        const days = Math.floor(seconds / 86400);
        seconds %= 86400;
        const hours = Math.floor(seconds / 3600);
        seconds %= 3600;
        const minutes = Math.floor(seconds / 60);
        const secs = seconds % 60;
        
        let result = '';
        if (days > 0) result += days + 'd ';
        if (hours > 0) result += hours + 'h ';
        if (minutes > 0) result += minutes + 'm ';
        result += secs + 's';
        return result;
    }
    
    updateTimes();
    setInterval(updateTimes, 1000);
    
    // Add button event handlers
    document.getElementById('refresh-btn').addEventListener('click', function() {
        fetch('/api/admin/refresh', { method: 'POST' })
            .then(response => response.json())
            .then(data => alert(data.message))
            .catch(error => console.error('Error:', error));
    });
    
    document.getElementById('toggle-pause-btn').addEventListener('click', function() {
        const isPaused = {{.State.Paused}};
        fetch('/api/admin/pause', { 
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ paused: !isPaused })
        })
        .then(response => response.json())
        .then(data => location.reload())
        .catch(error => console.error('Error:', error));
    });
});
</script>
{{end}}
`

	// Write templates
	if err := os.WriteFile(filepath.Join(DashboardTemplatesDir, "layout.html"), []byte(layoutTemplate), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(DashboardTemplatesDir, "index.html"), []byte(indexTemplate), 0644); err != nil {
		return err
	}

	// Create basic CSS file
	cssContent := `
body {
    padding-bottom: 70px;
}

.status-indicator {
    padding: 10px;
    border-radius: 5px;
    color: white;
    text-align: center;
    font-weight: bold;
}

.bg-success {
    background-color: #28a745;
}

.bg-warning {
    background-color: #ffc107;
}

.bg-danger {
    background-color: #dc3545;
}
`
	if err := os.WriteFile(filepath.Join(DashboardStaticDir, "style.css"), []byte(cssContent), 0644); err != nil {
		return err
	}

	// Create basic JavaScript file
	jsContent := `
// Dashboard WebSocket connection
let socket;

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    socket = new WebSocket(protocol + '//' + window.location.host + '/api/ws');
    
    socket.onopen = function() {
        console.log('WebSocket connection established');
    };
    
    socket.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('WebSocket message received:', data);
            
            // Handle different event types
            switch (data.type) {
                case 'source_added':
                case 'source_updated':
                case 'source_deleted':
                case 'config_updated':
                case 'state_updated':
                    // Refresh the page to show updated data
                    location.reload();
                    break;
            }
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
        }
    };
    
    socket.onclose = function() {
        console.log('WebSocket connection closed');
        // Try to reconnect after a delay
        setTimeout(connectWebSocket, 5000);
    };
    
    socket.onerror = function(error) {
        console.error('WebSocket error:', error);
    };
}

// Connect when page loads
document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket();
    
    // Keep connection alive with ping
    setInterval(function() {
        if (socket && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'ping' }));
        }
    }, 30000);
});
`
	if err := os.WriteFile(filepath.Join(DashboardStaticDir, "dashboard.js"), []byte(jsContent), 0644); err != nil {
		return err
	}

	return nil
}
