// cmd/sankarea/util.go
package main

import (
    "encoding/json"
    "net/http"
    "runtime"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

const (
    TimeFormatFull = "2006-01-02 15:04:05 MST"
)

// HTTP Response Helpers
func respondWithHTTPError(w http.ResponseWriter, code int, message string) {
    respondWithJSON(w, code, map[string]string{"error": message})
}

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

// Discord Permission Helpers
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

// IsAdmin checks if a user ID belongs to an admin
func IsAdmin(userID string) bool {
    for _, ownerID := range cfg.OwnerIDs {
        if userID == ownerID {
            return true
        }
    }
    return false
}

// Recovery Helper
func RecoverFromPanic(component string) {
    if r := recover(); r != nil {
        stack := make([]byte, 4096)
        stack = stack[:runtime.Stack(stack, false)]
        
        // Use new logger for panic logging
        logger := Logger()
        logger.Error("Panic in %s: %v\n%s", component, r, stack)
    }
}

// Time and Duration Formatting
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
