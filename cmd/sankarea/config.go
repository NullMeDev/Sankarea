// cmd/sankarea/config.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// Config represents the bot's configuration
type Config struct {
    // Discord configuration
    Token     string `json:"token"`
    OwnerID   string `json:"owner_id"`
    GuildID   string `json:"guild_id,omitempty"` // Optional: for development in a specific guild

    // Database configuration
    DatabasePath string `json:"database_path"`

    // News fetching configuration
    FetchInterval   int      `json:"fetch_interval"`   // in minutes
    MaxPostsPerRun  int      `json:"max_posts_per_run"`
    SourcesPath     string   `json:"sources_path"`
    CachePath       string   `json:"cache_path"`
    Categories      []string `json:"categories"`

    // Fact checking configuration
    EnableFactCheck bool    `json:"enable_fact_check"`
    FactCheckAPI    string `json:"fact_check_api,omitempty"`
    FactCheckKey    string `json:"fact_check_key,omitempty"`

    // Dashboard configuration
    DashboardEnabled bool   `json:"dashboard_enabled"`
    DashboardPort   int    `json:"dashboard_port,omitempty"`
    DashboardHost   string `json:"dashboard_host,omitempty"`

    // Logging configuration
    LogPath      string `json:"log_path"`
    LogLevel     string `json:"log_level"`
    LogToConsole bool   `json:"log_to_console"`

    // Runtime configuration
    StartTime time.Time `json:"-"` // Not stored in JSON
}

var (
    cfg *Config
    DefaultCategories = []string{
        "Technology",
        "Business",
        "Science",
        "Health",
        "Politics",
        "Sports",
        "World",
    }
)

// LoadConfig loads the configuration from the specified file
func LoadConfig(path string) (*Config, error) {
    // Ensure absolute path
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %v", err)
    }

    // Read config file
    data, err := os.ReadFile(absPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %v", err)
    }

    // Parse config
    config := &Config{}
    if err := json.Unmarshal(data, config); err != nil {
        return nil, fmt.Errorf("failed to parse config: %v", err)
    }

    // Validate required fields
    if err := validateConfig(config); err != nil {
        return nil, err
    }

    // Set default values if not specified
    setDefaults(config)

    // Set runtime values
    config.StartTime = time.Now()

    // Create necessary directories
    if err := createDirectories(config); err != nil {
        return nil, err
    }

    cfg = config
    return config, nil
}

// validateConfig checks if all required configuration fields are set
func validateConfig(c *Config) error {
    if c.Token == "" {
        return fmt.Errorf("Discord token is required")
    }
    if c.OwnerID == "" {
        return fmt.Errorf("owner ID is required")
    }
    if c.DatabasePath == "" {
        return fmt.Errorf("database path is required")
    }
    if c.SourcesPath == "" {
        return fmt.Errorf("sources path is required")
    }
    if c.EnableFactCheck && c.FactCheckAPI == "" {
        return fmt.Errorf("fact check API is required when fact checking is enabled")
    }
    return nil
}

// setDefaults sets default values for optional configuration fields
func setDefaults(c *Config) {
    if c.FetchInterval <= 0 {
        c.FetchInterval = 15 // 15 minutes default
    }
    if c.MaxPostsPerRun <= 0 {
        c.MaxPostsPerRun = 5 // 5 posts per run default
    }
    if c.CachePath == "" {
        c.CachePath = "cache"
    }
    if c.LogPath == "" {
        c.LogPath = "logs"
    }
    if c.LogLevel == "" {
        c.LogLevel = "info"
    }
    if c.Categories == nil || len(c.Categories) == 0 {
        c.Categories = DefaultCategories
    }
    if c.DashboardPort <= 0 {
        c.DashboardPort = 8080
    }
    if c.DashboardHost == "" {
        c.DashboardHost = "localhost"
    }
}

// createDirectories ensures all necessary directories exist
func createDirectories(c *Config) error {
    dirs := []string{
        filepath.Dir(c.DatabasePath),
        filepath.Dir(c.SourcesPath),
        c.CachePath,
        c.LogPath,
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", dir, err)
        }
    }

    return nil
}

// SaveConfig saves the current configuration to file
func SaveConfig(path string) error {
    if cfg == nil {
        return fmt.Errorf("no configuration loaded")
    }

    data, err := json.MarshalIndent(cfg, "", "    ")
    if err != nil {
        return fmt.Errorf("failed to marshal config: %v", err)
    }

    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to write config file: %v", err)
    }

    return nil
}

// GetConfig returns the current configuration
func GetConfig() *Config {
    return cfg
}

// GetUptime returns the duration since the bot started
func (c *Config) GetUptime() time.Duration {
    return time.Since(c.StartTime)
}
