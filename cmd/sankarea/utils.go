package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// LoadEnv loads environment variables from .env file
func LoadEnv() {
	godotenv.Load()
}

// FileMustExist checks if a file exists and is readable
// If not, it creates a default version of the file
func FileMustExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			Logger().Printf("Failed to create directory %s: %v", dir, err)
			os.Exit(1)
		}

		// Create empty file
		file, err := os.Create(path)
		if err != nil {
			Logger().Printf("Failed to create file %s: %v", path, err)
			os.Exit(1)
		}
		file.Close()

		Logger().Printf("Created empty file: %s", path)
	}
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() {
	if err := os.MkdirAll("data", 0755); err != nil {
		Logger().Printf("Failed to create data directory: %v", err)
		os.Exit(1)
	}
}

// TimeAgo returns a friendly string describing how long ago a time was
func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	
	duration := time.Since(t)
	
	if duration.Seconds() < 60 {
		return fmt.Sprintf("%d seconds ago", int(duration.Seconds()))
	}
	if duration.Minutes() < 60 {
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	}
	if duration.Hours() < 24 {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	}
	if duration.Hours() < 48 {
		return "yesterday"
	}
	if duration.Hours() < 168 {
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
	if duration.Hours() < 720 {
		return fmt.Sprintf("%d weeks ago", int(duration.Hours()/168))
	}
	
	return t.Format("Jan 2, 2006")
}

// GenerateUUID generates a random UUID string
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	
	return hex.EncodeToString(uuid)
}

// TruncateString safely truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	
	// Try to truncate at a word boundary
	pos := strings.LastIndex(s[:maxLen], " ")
	if pos > maxLen/2 {
		return s[:pos] + "..."
	}
	
	// Otherwise just truncate
	return s[:maxLen] + "..."
}

// SanitizeHTML strips HTML tags from a string
func SanitizeHTML(input string) string {
	// First unescape any HTML entities
	input = html.UnescapeString(input)
	
	// Define regular expression patterns for HTML elements
	tagPattern := regexp.MustCompile("<[^>]*>")
	
	// Remove all HTML tags
	noTags := tagPattern.ReplaceAllString(input, "")
	
	// Remove excessive whitespace
	whiteSpacePattern := regexp.MustCompile(`\s+`)
	cleanText := whiteSpacePattern.ReplaceAllString(noTags, " ")
	
	return strings.TrimSpace(cleanText)
}

// FormatDuration formats a duration into a human-readable string
func FormatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	var parts []string
	
	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	
	if minutes > 0 && days == 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	
	if seconds > 0 && days == 0 && hours == 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}
	
	if len(parts) == 0 {
		return "0 seconds"
	}
	
	return strings.Join(parts, " ")
}

// User agent for HTTP requests
const userAgent = "Sankarea/1.0 RSS Reader Bot"

// GetHTTPClient returns a configured HTTP client
func GetHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// IsValidURL checks if a string is a valid URL
func IsValidURL(url string) bool {
	_, err := http.NewRequest("GET", url, nil)
	return err == nil
}

// SafeGet performs a GET request with proper error handling
func SafeGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", userAgent)
	
	// Increment API request counter
	IncrementAPIRequestCount()
	
	// Perform the request
	client := GetHTTPClient()
	return client.Do(req)
}

// ParseCronSchedule validates a cron schedule string
func ParseCronSchedule(schedule string) (string, error) {
	if schedule == "" {
		return "0 8 * * *", nil // Default to 8 AM daily
	}
	
	// Validate by creating a cron schedule
	_, err := cron.ParseStandard(schedule)
	if err != nil {
		return "", fmt.Errorf("invalid cron schedule: %w", err)
	}
	
	return schedule, nil
}

// SplitStringList splits a comma-separated string into a list of strings
func SplitStringList(s string) []string {
	if s == "" {
		return []string{}
	}
	
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

// JoinStringList joins a list of strings with commas
func JoinStringList(list []string) string {
	return strings.Join(list, ", ")
}

// IsAdmin checks if a user has admin permissions
func IsAdmin(userID string) bool {
	if cfg == nil || userID == "" {
		return false
	}
	
	for _, ownerID := range cfg.OwnerIDs {
		if ownerID == userID {
			return true
		}
	}
	
	return false
}

// SendErrorMessage sends an error message to the configured error channel
func SendErrorMessage(s *discordgo.Session, errorMsg string, err error) {
	if s == nil || cfg == nil || cfg.ErrorChannelID == "" {
		return
	}
	
	embed := &discordgo.MessageEmbed{
		Title:       "Error Occurred",
		Description: errorMsg,
		Color:       0xE74C3C, // Red color
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Error",
				Value:  err.Error(),
				Inline: false,
			},
		},
	}
	
	_, err = s.ChannelMessageSendEmbed(cfg.ErrorChannelID, embed)
	if err != nil {
		Logger().Printf("Failed to send error message: %v", err)
	}
}

// Logger returns the application logger
func Logger() *Log {
	// If we haven't initialized the logger yet, return a basic logger
	if appLogger == nil {
		return &Log{
			logger: log.New(os.Stderr, "SANKAREA: ", log.LstdFlags),
		}
	}
	return appLogger
}

// Log is a wrapper around the standard logger
type Log struct {
	logger *log.Logger
	file   *os.File
}

// Printf logs a formatted message
func (l *Log) Printf(format string, v ...interface{}) {
	l.logger.Printf(format, v...)
}

// Println logs a message
func (l *Log) Println(v ...interface{}) {
	l.logger.Println(v...)
}

// Fatalf logs a fatal message and exits
func (l *Log) Fatalf(format string, v ...interface{}) {
	l.logger.Fatalf(format, v...)
}

// Close closes the log file
func (l *Log) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// SetupLogging sets up logging to file and console
func SetupLogging() error {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	
	// Create log file with date in name
	logFileName := filepath.Join(logDir, fmt.Sprintf("sankarea_%s.log", 
		time.Now().Format("2006-01-02")))
	
	logFile, err := os.OpenFile(logFileName, 
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	
	// Create multi-writer for both console and file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	
	// Create logger
	logger := log.New(multiWriter, "SANKAREA: ", 
		log.Ldate|log.Ltime|log.Lshortfile)
	
	// Set global logger
	appLogger = &Log{
		logger: logger,
		file:   logFile,
	}
	
	return nil
}

// IncrementAPIRequestCount increases the API request counter
func IncrementAPIRequestCount() {
	// This is a placeholder - would actually update a counter in state
	// or analytics system in a full implementation
}

// getCurrentDateFormatted returns the current date formatted for reports
func GetCurrentDateFormatted() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// validateCronExpression validates a cron expression string
func ValidateCronExpression(expression string) bool {
	_, err := cron.ParseStandard(expression)
	return err == nil
}

// EncodeToBase64 encodes a string to base64
func EncodeToBase64(input string) string {
	return input // Placeholder - would use proper base64 encoding
}

// RecoverFromPanic handles panics gracefully
func RecoverFromPanic(component string) {
	if r := recover(); r != nil {
		Logger().Printf("PANIC RECOVERED in %s: %v", component, r)
		if errorSystem != nil {
			errorSystem.HandleError(
				fmt.Sprintf("Panic in %s", component),
				fmt.Errorf("%v", r),
				component,
				ErrorSeverityHigh,
			)
		} else {
			log.Printf("ERROR SYSTEM NOT INITIALIZED: Panic in %s: %v", component, r)
		}
	}
}

// Error severity levels
const (
	ErrorSeverityLow    = 1
	ErrorSeverityMedium = 2
	ErrorSeverityHigh   = 3
	ErrorSeverityFatal  = 4
)

// Global logger
var appLogger *Log

// Version constant
const VERSION = "1.0.0"
