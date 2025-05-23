// cmd/sankarea/config.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"

    "gopkg.in/yaml.v2"
)

// ConfigPaths contains default configuration file paths
const (
    DefaultConfigPath   = "config/config.json"
    DefaultSourcesPath = "config/sources.yml"
    DefaultStatePath   = "data/state.json"
    DefaultDataDir     = "data"
    DefaultLogDir      = "data/logs"
)

// Config validation constants
const (
    MinNewsInterval     = 5
    MaxNewsInterval     = 120
    MinPostsPerSource   = 1
    MaxPostsPerSource   = 50
    MaxSources         = 100
)

var (
    configMutex sync.RWMutex
    cfg         *Config
)

// loadConfiguration loads and validates the configuration
func loadConfiguration(configPath string) error {
    configMutex.Lock()
    defer configMutex.Unlock()

    // Ensure config directory exists
    configDir := filepath.Dir(configPath)
    if err := os.MkdirAll(configDir, 0755); err != nil {
        return fmt.Errorf("failed to create config directory: %v", err)
    }

    // Read config file
    data, err := os.ReadFile(configPath)
    if err != nil {
        if os.IsNotExist(err) {
            // Create default config if it doesn't exist
            cfg = getDefaultConfig()
            return saveConfiguration(configPath)
        }
        return fmt.Errorf("failed to read config file: %v", err)
    }

    // Parse config
    cfg = &Config{}
    if err := json.Unmarshal(data, cfg); err != nil {
        return fmt.Errorf("failed to parse config file: %v", err)
    }

    // Validate and set defaults
    if err := validateConfig(cfg); err != nil {
        return fmt.Errorf("invalid configuration: %v", err)
    }

    // Ensure required directories exist
    if err := createRequiredDirectories(); err != nil {
        return fmt.Errorf("failed to create required directories: %v", err)
    }

    return nil
}

// saveConfiguration saves the current configuration
func saveConfiguration(configPath string) error {
    configMutex.Lock()
    defer configMutex.Unlock()

    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal config: %v", err)
    }

    if err := os.WriteFile(configPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write config file: %v", err)
    }

    return nil
}

// getDefaultConfig returns a default configuration
func getDefaultConfig() *Config {
    return &Config{
        Version:              "1.0.0",
        NewsIntervalMinutes:  15,
        MaxPostsPerSource:    10,
        News15MinCron:        "*/15 * * * *",
        DigestCronSchedule:   "0 0 * * *", // Daily at midnight
        EnableImageEmbed:     true,
        EnableFactCheck:      false,
        EnableSummarization: false,
        EnableDatabase:      false,
        EnableDashboard:     true,
        DashboardPort:       8080,
        HealthAPIPort:       8081,
        UserAgentString:     "Sankarea News Bot/1.0",
        FetchNewsOnStartup:  true,
    }
}

// validateConfig validates the configuration and sets defaults
func validateConfig(cfg *Config) error {
    if cfg.BotToken == "" {
        return fmt.Errorf("bot token is required")
    }

    if cfg.AppID == "" {
        return fmt.Errorf("application ID is required")
    }

    // Validate intervals
    if cfg.NewsIntervalMinutes < MinNewsInterval || cfg.NewsIntervalMinutes > MaxNewsInterval {
        return fmt.Errorf("news interval must be between %d and %d minutes", MinNewsInterval, MaxNewsInterval)
    }

    if cfg.MaxPostsPerSource < MinPostsPerSource || cfg.MaxPostsPerSource > MaxPostsPerSource {
        return fmt.Errorf("max posts per source must be between %d and %d", MinPostsPerSource, MaxPostsPerSource)
    }

    // Set defaults if not specified
    if cfg.UserAgentString == "" {
        cfg.UserAgentString = fmt.Sprintf("Sankarea News Bot/%s", cfg.Version)
    }

    if cfg.DashboardPort == 0 {
        cfg.DashboardPort = 8080
    }

    if cfg.HealthAPIPort == 0 {
        cfg.HealthAPIPort = 8081
    }

    return nil
}

// createRequiredDirectories ensures all required directories exist
func createRequiredDirectories() error {
    dirs := []string{
        DefaultDataDir,
        DefaultLogDir,
        filepath.Dir(DefaultConfigPath),
        filepath.Dir(DefaultSourcesPath),
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", dir, err)
        }
    }

    return nil
}

// loadSources loads the news sources configuration
func loadSources() []Source {
    configMutex.RLock()
    defer configMutex.RUnlock()

    data, err := os.ReadFile(DefaultSourcesPath)
    if err != nil {
        Logger().Printf("Failed to read sources file: %v", err)
        return []Source{}
    }

    var sources []Source
    if err := yaml.Unmarshal(data, &sources); err != nil {
        Logger().Printf("Failed to parse sources file: %v", err)
        return []Source{}
    }

    // Validate source count
    if len(sources) > MaxSources {
        Logger().Printf("Warning: Number of sources (%d) exceeds maximum (%d)", len(sources), MaxSources)
        sources = sources[:MaxSources]
    }

    return sources
}

// saveSources saves the news sources configuration
func saveSources(sources []Source) error {
    configMutex.Lock()
    defer configMutex.Unlock()

    // Update last modified time
    for i := range sources {
        sources[i].LastFetched = time.Now()
    }

    data, err := yaml.Marshal(sources)
    if err != nil {
        return fmt.Errorf("failed to marshal sources: %v", err)
    }

    if err := os.WriteFile(DefaultSourcesPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write sources file: %v", err)
    }

    return nil
}

// GetConfig returns a copy of the current configuration
func GetConfig() Config {
    configMutex.RLock()
    defer configMutex.RUnlock()
    return *cfg
}

// UpdateConfig updates the configuration with new values
func UpdateConfig(updater func(*Config)) error {
    configMutex.Lock()
    defer configMutex.Unlock()

    updater(cfg)
    if err := validateConfig(cfg); err != nil {
        return err
    }

    return saveConfiguration(DefaultConfigPath)
}
