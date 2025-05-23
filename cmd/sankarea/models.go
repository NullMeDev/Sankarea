// cmd/sankarea/models.go
package main

import (
    "fmt"
    "time"
)

// NewsSource represents a news feed source configuration
type NewsSource struct {
    Name      string `json:"name"`
    URL       string `json:"url"`
    Category  string `json:"category"`
    FactCheck bool   `json:"fact_check"`
    Paused    bool   `json:"paused"`
}

// NewsArticle represents a processed news article
type NewsArticle struct {
    ID             string          `json:"id"`
    Title          string          `json:"title"`
    Content        string          `json:"content"`
    URL            string          `json:"url"`
    Source         string          `json:"source"`
    Category       string          `json:"category"`
    PublishedAt    time.Time       `json:"published_at"`
    FetchedAt      time.Time       `json:"fetched_at"`
    Citations      []string        `json:"citations,omitempty"`
    FactCheckResult *FactCheckResult `json:"fact_check_result,omitempty"`
}

// ErrorEvent represents an error occurrence for tracking
type ErrorEvent struct {
    Time      time.Time `json:"time"`
    Component string    `json:"component"`
    Message   string    `json:"message"`
    Severity  string    `json:"severity"`
}

// ErrorBuffer maintains a circular buffer of recent errors
type ErrorBuffer struct {
    Events    []*ErrorEvent
    MaxSize   int
    Position  int
}

// BotConfig represents the bot's configuration
type BotConfig struct {
    Token              string   `json:"token"`
    OwnerIDs          []string `json:"owner_ids"`
    Prefix            string   `json:"prefix"`
    NewsIntervalMinutes int     `json:"news_interval_minutes"`
    LogPath           string   `json:"log_path"`
    LogLevel          LogLevel `json:"log_level"`
    MaxLogSize        int64    `json:"max_log_size"`
    DatabasePath      string   `json:"database_path"`
}

// Category constants
const (
    CategoryTechnology = "Technology"
    CategoryBusiness  = "Business"
    CategoryScience   = "Science"
    CategoryHealth    = "Health"
    CategoryPolitics  = "Politics"
    CategorySports    = "Sports"
    CategoryWorld     = "World"
)

// NewErrorBuffer creates a new error buffer with specified size
func NewErrorBuffer(size int) *ErrorBuffer {
    return &ErrorBuffer{
        Events:   make([]*ErrorEvent, size),
        MaxSize:  size,
        Position: 0,
    }
}

// Add adds a new error event to the buffer
func (eb *ErrorBuffer) Add(event *ErrorEvent) {
    eb.Events[eb.Position] = event
    eb.Position = (eb.Position + 1) % eb.MaxSize
}

// GetErrors returns all stored errors in chronological order
func (eb *ErrorBuffer) GetErrors() []*ErrorEvent {
    result := make([]*ErrorEvent, 0, eb.MaxSize)
    
    // Add events from position to end
    for i := eb.Position; i < eb.MaxSize; i++ {
        if eb.Events[i] != nil {
            result = append(result, eb.Events[i])
        }
    }
    
    // Add events from start to position
    for i := 0; i < eb.Position; i++ {
        if eb.Events[i] != nil {
            result = append(result, eb.Events[i])
        }
    }
    
    return result
}

// GetRecentErrors returns the n most recent errors
func (eb *ErrorBuffer) GetRecentErrors(n int) []*ErrorEvent {
    allErrors := eb.GetErrors()
    if len(allErrors) <= n {
        return allErrors
    }
    return allErrors[len(allErrors)-n:]
}

// ClearErrors removes all stored errors
func (eb *ErrorBuffer) ClearErrors() {
    eb.Events = make([]*ErrorEvent, eb.MaxSize)
    eb.Position = 0
}

// LoadSources loads news sources from configuration
func LoadSources() ([]NewsSource, error) {
    // TODO: Implement loading from configuration file or database
    // For now, return default sources
    return []NewsSource{
        {
            Name:      "Tech News",
            URL:       "https://example.com/tech.rss",
            Category:  CategoryTechnology,
            FactCheck: true,
        },
        {
            Name:      "World News",
            URL:       "https://example.com/world.rss",
            Category:  CategoryWorld,
            FactCheck: true,
        },
    }, nil
}

// ValidateConfig checks if the configuration is valid
func (c *BotConfig) ValidateConfig() error {
    if c.Token == "" {
        return fmt.Errorf("bot token is required")
    }
    
    if len(c.OwnerIDs) == 0 {
        return fmt.Errorf("at least one owner ID is required")
    }
    
    if c.NewsIntervalMinutes < 1 {
        return fmt.Errorf("news interval must be at least 1 minute")
    }
    
    if c.LogPath == "" {
        c.LogPath = "data/logs/sankarea.log"
    }
    
    if c.DatabasePath == "" {
        c.DatabasePath = "data/sankarea.db"
    }
    
    return nil
}
