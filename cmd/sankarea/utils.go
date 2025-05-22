package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
	
	if minutes > 0 && days == 0 { // Only show minutes if less than
