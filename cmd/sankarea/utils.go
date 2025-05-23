// cmd/sankarea/utils.go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "strings"
    "runtime"
    
    "github.com/bwmarrin/discordgo"
)

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

// CheckCommandPermissions checks if the user has permission to use the command
func CheckCommandPermissions(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
    // Check if user is owner
    for _, ownerID := range cfg.OwnerIDs {
        if i.Member.User.ID == ownerID {
            return true
        }
    }

    // Allow all basic commands
    switch i.ApplicationCommandData().Name {
    case "ping", "status", "version", "help", "news", "digest":
        return true
    }

    // For admin commands, check if user is admin
    if i.ApplicationCommandData().Name == "admin" {
        return IsAdmin(i.Member.User.ID)
    }

    // Check guild permissions for other commands
    if i.Member.Permissions&discordgo.PermissionAdministrator != 0 {
        return true
    }

    return false
}

// AuditLog logs important actions
func AuditLog(message string) {
    Logger().Printf("[AUDIT] %s", message)
}

// FormatDuration formats a duration into a human-readable string
func FormatDuration(d time.Duration) string {
    days := int(d.Hours() / 24)
    hours := int(d.Hours()) % 24
    minutes := int(d.Minutes()) % 60
    seconds := int(d.Seconds()) % 60

    parts := []string{}
    if days > 0 {
        parts = append(parts, fmt.Sprintf("%dd", days))
    }
    if hours > 0 {
        parts = append(parts, fmt.Sprintf("%dh", hours))
    }
    if minutes > 0 {
        parts = append(parts, fmt.Sprintf("%dm", minutes))
    }
    if seconds > 0 || len(parts) == 0 {
        parts = append(parts, fmt.Sprintf("%ds", seconds))
    }

    return strings.Join(parts, " ")
}

// IsAdmin checks if a user ID belongs to an admin
func IsAdmin(userID string) bool {
    // Check owner IDs first
    for _, ownerID := range cfg.OwnerIDs {
        if userID == ownerID {
            return true
        }
    }

    // Additional admin checks could be added here
    return false
}

// RecoverFromPanic recovers from panics in goroutines
func RecoverFromPanic(component string) {
    if r := recover(); r != nil {
        stack := make([]byte, 4096)
        stack = stack[:runtime.Stack(stack, false)]
        Logger().Printf("[PANIC] %s: %v\n%s", component, r, stack)
        AuditLog(fmt.Sprintf("Recovered from panic in %s: %v", component, r))
    }
}

// collectMetrics gathers system metrics
func collectMetrics() Metrics {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    return Metrics{
        MemoryUsageMB:     float64(m.Alloc) / 1024 / 1024,
        CPUUsagePercent:   getCPUUsage(),
        DiskUsagePercent:  getDiskUsage(),
        UptimeSeconds:     time.Since(state.StartupTime).Seconds(),
        ArticlesPerMinute: getArticleRate(),
        ErrorsPerHour:     getErrorRate(),
        APICallsPerHour:   getAPICallRate(),
    }
}

// Helper functions for metrics collection
func getCPUUsage() float64 {
    // Implementation would go here
    return 0.0
}

func getDiskUsage() float64 {
    // Implementation would go here
    return 0.0
}

func getArticleRate() float64 {
    // Implementation would go here
    return 0.0
}

func getErrorRate() float64 {
    // Implementation would go here
    return 0.0
}

func getAPICallRate() float64 {
    // Implementation would go here
    return 0.0
}

// Logger returns the application logger
func Logger() interface {
    Printf(format string, v ...interface{})
} {
    // This would typically return a configured logger
    return defaultLogger
}

var defaultLogger = &DefaultLogger{}

// DefaultLogger provides basic logging capabilities
type DefaultLogger struct{}

// Printf implements basic printf logging
func (l *DefaultLogger) Printf(format string, v ...interface{}) {
    fmt.Printf(time.Now().Format("2006-01-02 15:04:05")+" "+format+"\n", v...)
}
