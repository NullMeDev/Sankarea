// cmd/sankarea/util.go
package main

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "html/template"
    "math"
    "net/http"
    "regexp"
    "runtime"
    "strings"
    "sync"
    "time"
    "unicode"

    "github.com/bwmarrin/discordgo"
)

const (
    // Time formats
    TimeFormatFull    = "2006-01-02 15:04:05 MST"
    TimeFormatCompact = "2006-01-02 15:04"
    TimeFormatDate    = "2006-01-02"
    
    // Size constants
    KB = 1024
    MB = 1024 * KB
    GB = 1024 * MB
)

// Regular expressions for various validations
var (
    urlRegex     = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
    markdownLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// HTTP Response Helpers (from original utils.go)
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

// Discord Permission Helpers (from original utils.go)
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

// Logging and Recovery (from original utils.go)
func RecoverFromPanic(component string) {
    if r := recover(); r != nil {
        stack := make([]byte, 4096)
        stack = stack[:runtime.Stack(stack, false)]
        Logger().Printf("[PANIC] %s: %v\n%s", component, r, stack)
        AuditLog(fmt.Sprintf("Recovered from panic in %s: %v", component, r))
    }
}

// AuditLog logs important actions
func AuditLog(message string) {
    Logger().Printf("[AUDIT] %s", message)
}

// Logger interface and implementation
type Logger interface {
    Printf(format string, v ...interface{})
}

var defaultLogger = &DefaultLogger{}

type DefaultLogger struct{}

func (l *DefaultLogger) Printf(format string, v ...interface{}) {
    fmt.Printf(time.Now().Format(TimeFormatFull)+" "+format+"\n", v...)
}

func Logger() Logger {
    return defaultLogger
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

// FormatTimeAgo returns a human-readable time difference
func FormatTimeAgo(t time.Time) string {
    diff := time.Since(t)
    
    switch {
    case diff < time.Minute:
        return "just now"
    case diff < time.Hour:
        minutes := int(diff.Minutes())
        return fmt.Sprintf("%dm ago", minutes)
    case diff < 24*time.Hour:
        hours := int(diff.Hours())
        return fmt.Sprintf("%dh ago", hours)
    case diff < 7*24*time.Hour:
        days := int(diff.Hours() / 24)
        return fmt.Sprintf("%dd ago", days)
    default:
        return t.Format(TimeFormatDate)
    }
}

// String Manipulation
func GenerateRandomString(length int) (string, error) {
    bytes := make([]byte, int(math.Ceil(float64(length)/1.33333333333)))
    
    if _, err := rand.Read(bytes); err != nil {
        return "", fmt.Errorf("failed to generate random string: %v", err)
    }
    
    return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

func SanitizeString(s string) string {
    return strings.Map(func(r rune) rune {
        if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
            return r
        }
        return -1
    }, s)
}

func TruncateString(s string, length int) string {
    if len(s) <= length {
        return s
    }
    return s[:length-3] + "..."
}

// URL Handling
func ExtractURLFromMarkdown(markdown string) string {
    matches := markdownLink.FindStringSubmatch(markdown)
    if len(matches) >= 3 {
        return matches[2]
    }
    return ""
}

func IsValidURL(url string) bool {
    return urlRegex.MatchString(url)
}

// Error Buffer (from original utils.go)
type ErrorBuffer struct {
    events []*ErrorEvent
    size   int
    mutex  sync.RWMutex
}

func NewErrorBuffer(size int) *ErrorBuffer {
    return &ErrorBuffer{
        events: make([]*ErrorEvent, 0, size),
        size:   size,
    }
}

func (eb *ErrorBuffer) Add(event *ErrorEvent) {
    eb.mutex.Lock()
    defer eb.mutex.Unlock()

    if len(eb.events) >= eb.size {
        eb.events = eb.events[1:]
    }
    eb.events = append(eb.events, event)
}

func (eb *ErrorBuffer) GetRecent(count int) []*ErrorEvent {
    eb.mutex.RLock()
    defer eb.mutex.RUnlock()

    if count > len(eb.events) {
        count = len(eb.events)
    }

    result := make([]*ErrorEvent, count)
    copy(result, eb.events[len(eb.events)-count:])
    return result
}

// Retry mechanism
func RetryWithBackoff(attempts int, initialDelay time.Duration, maxDelay time.Duration, f func() error) error {
    var err error
    delay := initialDelay
    
    for i := 0; i < attempts; i++ {
        if err = f(); err == nil {
            return nil
        }
        
        if i == attempts-1 {
            break
        }
        
        delay = time.Duration(float64(delay) * 2)
        if delay > maxDelay {
            delay = maxDelay
        }
        
        time.Sleep(delay)
    }
    
    return fmt.Errorf("failed after %d attempts: %v", attempts, err)
}

// Template function map
var TemplateHelpers = template.FuncMap{
    "formatTime":      func(t time.Time) string { return t.Format(TimeFormatFull) },
    "formatDuration":  FormatDuration,
    "formatTimeAgo":   FormatTimeAgo,
    "safeHTML":        func(s string) template.HTML { return template.HTML(s) },
}
