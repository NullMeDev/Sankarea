// cmd/sankarea/types.go
package main

import (
    "encoding/json"
    "time"

    "github.com/bwmarrin/discordgo"
)

// NewsArticle represents a news article
type NewsArticle struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    URL         string    `json:"url"`
    Source      string    `json:"source"`
    PublishedAt time.Time `json:"published_at"`
    FetchedAt   time.Time `json:"fetched_at"`
    Summary     string    `json:"summary"`
    Category    string    `json:"category"`
    Tags        []string  `json:"tags"`
    ImageURL    string    `json:"image_url,omitempty"`
}

// NewsDigest represents a collection of news articles
type NewsDigest struct {
    Articles    []*NewsArticle `json:"articles"`
    GeneratedAt time.Time      `json:"generated_at"`
    Period      string         `json:"period"`
}

// Metrics represents application metrics
type Metrics struct {
    UptimeSeconds     float64            `json:"uptime_seconds"`
    MemoryUsageMB     float64            `json:"memory_usage_mb"`
    CPUUsagePercent   float64            `json:"cpu_usage_percent"`
    DiskUsagePercent  float64            `json:"disk_usage_percent"`
    ArticlesPerMinute float64            `json:"articles_per_minute"`
    ErrorsPerHour     float64            `json:"errors_per_hour"`
    APICallsPerHour   float64            `json:"api_calls_per_hour"`
    ConnectedGuilds   int                `json:"connected_guilds"`
    ActiveSources     int                `json:"active_sources"`
    HealthStatus      string             `json:"health_status"`
    ComponentStatuses map[string]string  `json:"component_statuses"`
}

// Config represents the application configuration
type Config struct {
    // Discord configuration
    Token           string   `json:"token"`
    OwnerIDs       []string `json:"owner_ids"`
    CommandPrefix   string   `json:"command_prefix"`
    StatusMessage   string   `json:"status_message"`
    DefaultChannel  string   `json:"default_channel"`
    
    // News configuration
    NewsAPIKey     string            `json:"news_api_key"`
    NewsSources    []string          `json:"news_sources"`
    UpdateInterval time.Duration     `json:"update_interval"`
    Categories     map[string]string `json:"categories"`
    
    // Database configuration
    DatabaseURL    string `json:"database_url"`
    DatabaseName   string `json:"database_name"`
    
    // API configuration
    APIPort        int    `json:"api_port"`
    APITokens      map[string]APIToken `json:"api_tokens"`
    
    // Logging configuration
    LogLevel       string `json:"log_level"`
    LogFile        string `json:"log_file"`
    
    // Feature flags
    Features       map[string]bool `json:"features"`
    
    // Version information
    Version        string `json:"version"`
    BuildTime      string `json:"build_time"`
    GitCommit      string `json:"git_commit"`
}

// APIToken represents an API access token
type APIToken struct {
    Name        string    `json:"name"`
    Token       string    `json:"token"`
    Permissions []string  `json:"permissions"`
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// GuildConfig represents per-guild configuration
type GuildConfig struct {
    GuildID          string            `json:"guild_id"`
    NewsChannel      string            `json:"news_channel"`
    DigestChannel    string            `json:"digest_channel"`
    DigestSchedule   string            `json:"digest_schedule"`
    EnabledSources   []string          `json:"enabled_sources"`
    CategoryChannels map[string]string `json:"category_channels"`
    Prefix          string            `json:"prefix"`
    Language        string            `json:"language"`
    Timezone        string            `json:"timezone"`
    UpdatedAt       time.Time         `json:"updated_at"`
}

// CommandContext represents the context for command execution
type CommandContext struct {
    Session     *discordgo.Session
    Interaction *discordgo.InteractionCreate
    Guild       *GuildConfig
    User        *discordgo.User
    Command     string
    Args        []string
    RawArgs     string
}

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
    ID          string          `json:"id"`
    Type        string          `json:"type"`
    Schedule    string          `json:"schedule"`
    LastRun     time.Time       `json:"last_run"`
    NextRun     time.Time       `json:"next_run"`
    Parameters  map[string]any  `json:"parameters"`
    Enabled     bool            `json:"enabled"`
}

// APIResponse represents a standardized API response
type APIResponse struct {
    Success bool            `json:"success"`
    Data    interface{}     `json:"data,omitempty"`
    Error   *APIError       `json:"error,omitempty"`
    Meta    *APIMetadata    `json:"meta,omitempty"`
}

// APIError represents an API error response
type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// APIMetadata represents metadata for API responses
type APIMetadata struct {
    Page       int       `json:"page,omitempty"`
    PerPage    int       `json:"per_page,omitempty"`
    TotalPages int       `json:"total_pages,omitempty"`
    TotalItems int       `json:"total_items,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
}

// Custom JSON marshaling/unmarshaling for Config
func (c *Config) UnmarshalJSON(data []byte) error {
    type Alias Config
    aux := &struct {
        UpdateInterval string `json:"update_interval"`
        *Alias
    }{
        Alias: (*Alias)(c),
    }
    
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    
    // Parse update interval
    if aux.UpdateInterval != "" {
        duration, err := time.ParseDuration(aux.UpdateInterval)
        if err != nil {
            return err
        }
        c.UpdateInterval = duration
    }
    
    return nil
}

func (c Config) MarshalJSON() ([]byte, error) {
    type Alias Config
    return json.Marshal(&struct {
        UpdateInterval string `json:"update_interval"`
        Alias
    }{
        UpdateInterval: c.UpdateInterval.String(),
        Alias:         Alias(c),
    })
}
