package main

import (
	"fmt"
	"net/http"
	"time"
)

// User agent for HTTP requests
const userAgent = "Sankarea/1.0 RSS Reader Bot"

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

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

// TruncateString truncates a string to the specified length and adds ellipsis if needed
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
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
