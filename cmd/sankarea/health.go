// cmd/sankarea/utils.go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
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
    // Allow all basic commands
    switch i.ApplicationCommandData().Name {
    case "ping", "status", "version", "help":
        return true
    }

    // For admin commands, check if user is admin
    if i.ApplicationCommandData().Name == "admin" {
        return IsAdmin(i.Member.User.ID)
    }

    // Allow all other commands by default
    return true
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

    result := ""
    for i, part := range parts {
        if i > 0 {
            result += " "
        }
        result += part
    }
    return result
}
