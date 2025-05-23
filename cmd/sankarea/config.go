// cmd/sankarea/config.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
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
    FetchInterval   int    `json:"fetch_interval"`   // in minutes
    MaxPostsPerRun  int    `json:"max_posts_per_run"`
    SourcesPath     string `json:"sources_path"`
    CachePath       string `json:"cache_path"`

    // Fact checking configuration
    EnableFactCheck bool    `json:"enable_fact_check"`
    FactCheckAPI    string `json:"fact_check_api,omitempty"`

    // Dashboard configuration
    DashboardEnabled bool   `json:"dashboard_enabled"`
    DashboardPort   int    `json:"dashboard_port,omitempty"`
    DashboardHost   string `json:"dashboard_host,omitempty"`
}

var (
    cfg *Config
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
    if c.DashboardPort <= 0 {
        c.DashboardPort = 8080
    }
    if c.DashboardHost == "" {
        c.DashboardHost = "localhost"
    }
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
