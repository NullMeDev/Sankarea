// cmd/sankarea/main.go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
)

// VERSION represents the bot version
const VERSION = "1.0.0"

// BotConfig represents the bot's configuration
type BotConfig struct {
    Token            string   `json:"token"`
    OwnerID          string   `json:"owner_id"`
    GuildID          string   `json:"guild_id"`
    DatabasePath     string   `json:"database_path"`
    FetchInterval    int      `json:"fetch_interval"`
    MaxPostsPerRun   int      `json:"max_posts_per_run"`
    SourcesPath      string   `json:"sources_path"`
    CachePath        string   `json:"cache_path"`
    DashboardEnabled bool     `json:"dashboard_enabled"`
    DashboardPort    int      `json:"dashboard_port"`
    DashboardHost    string   `json:"dashboard_host"`
    LogPath          string   `json:"log_path"`
    LogLevel         string   `json:"log_level"`
    LogToConsole     bool     `json:"log_to_console"`
    Categories       []string `json:"categories"`
    CategoryChannels map[string]string
}

func main() {
    // Setup logging
    log.SetFlags(log.LstdFlags | log.Lshortfile)
    
    // Load configuration
    config, err := loadConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Ensure required directories exist
    if err := ensureDirectories(config); err != nil {
        log.Fatalf("Failed to create required directories: %v", err)
    }

    // Create and start bot
    bot, err := NewBot(config)
    if err != nil {
        log.Fatalf("Failed to create bot: %v", err)
    }

    // Start the bot
    if err := bot.Start(); err != nil {
        log.Fatalf("Failed to start bot: %v", err)
    }

    // Log successful startup
    log.Printf("Sankarea News Bot v%s is now running", VERSION)

    // Wait for interrupt signal
    sc := make(chan os.Signal, 1)
    signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
    <-sc

    // Cleanup and exit
    log.Println("Shutting down...")
    bot.Stop()
}

// loadConfig loads the bot configuration from file
func loadConfig() (*BotConfig, error) {
    // Try to load from environment first
    token := os.Getenv("DISCORD_BOT_TOKEN")
    if token == "" {
        return nil, fmt.Errorf("DISCORD_BOT_TOKEN environment variable not set")
    }

    // Load config file
    configPath := "config.json"
    if envConfig := os.Getenv("BOT_CONFIG_PATH"); envConfig != "" {
        configPath = envConfig
    }

    file, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %v", err)
    }

    var config BotConfig
    if err := json.Unmarshal(file, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %v", err)
    }

    // Override token from environment
    config.Token = token

    // Set defaults if not specified
    if config.FetchInterval == 0 {
        config.FetchInterval = 15 // 15 minutes default
    }
    if config.MaxPostsPerRun == 0 {
        config.MaxPostsPerRun = 5
    }
    if config.LogLevel == "" {
        config.LogLevel = "info"
    }
    if config.DatabasePath == "" {
        config.DatabasePath = "data/sankarea.db"
    }
    if config.SourcesPath == "" {
        config.SourcesPath = "config/sources.yml"
    }
    if config.CachePath == "" {
        config.CachePath = "data/cache"
    }

    return &config, nil
}

// ensureDirectories creates required directories if they don't exist
func ensureDirectories(config *BotConfig) error {
    dirs := []string{
        filepath.Dir(config.DatabasePath),
        config.CachePath,
        "logs",
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", dir, err)
        }
    }

    return nil
}
