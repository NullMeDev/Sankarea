// cmd/sankarea/env.go
package main

import (
    "os"
    "strconv"
    "strings"
)

// GetEnvString gets a string from environment variables with a default value
func GetEnvString(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}

// GetEnvInt gets an integer from environment variables with a default value
func GetEnvInt(key string, defaultValue int) int {
    if value, exists := os.LookupEnv(key); exists {
        if intValue, err := strconv.Atoi(value); err == nil {
            return intValue
        }
    }
    return defaultValue
}

// GetEnvBool gets a boolean from environment variables with a default value
func GetEnvBool(key string, defaultValue bool) bool {
    if value, exists := os.LookupEnv(key); exists {
        if boolValue, err := strconv.ParseBool(value); err == nil {
            return boolValue
        }
    }
    return defaultValue
}

// GetEnvFloat gets a float64 from environment variables with a default value
func GetEnvFloat(key string, defaultValue float64) float64 {
    if value, exists := os.LookupEnv(key); exists {
        if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
            return floatValue
        }
    }
    return defaultValue
}

// GetEnvStringSlice gets a string slice from environment variables with a default value
func GetEnvStringSlice(key string, defaultValue []string) []string {
    if value, exists := os.LookupEnv(key); exists {
        if value == "" {
            return defaultValue
        }
        return strings.Split(value, ",")
    }
    return defaultValue
}

// LoadEnvConfig loads configuration from environment variables
func LoadEnvConfig() *Config {
    return &Config{
        Version:               GetEnvString("BOT_VERSION", "1.0.0"),
        BotToken:             GetEnvString("BOT_TOKEN", ""),
        AppID:                GetEnvString("APP_ID", ""),
        GuildID:              GetEnvString("GUILD_ID", ""),
        OwnerIDs:             GetEnvStringSlice("OWNER_IDS", []string{}),
        NewsChannelID:        GetEnvString("NEWS_CHANNEL_ID", ""),
        ErrorChannelID:       GetEnvString("ERROR_CHANNEL_ID", ""),
        NewsIntervalMinutes:  GetEnvInt("NEWS_INTERVAL_MINUTES", 15),
        News15MinCron:        GetEnvString("NEWS_15MIN_CRON", "*/15 * * * *"),
        DigestCronSchedule:   GetEnvString("DIGEST_CRON_SCHEDULE", "0 8 * * *"),
        MaxPostsPerSource:    GetEnvInt("MAX_POSTS_PER_SOURCE", 5),
        EnableImageEmbed:     GetEnvBool("ENABLE_IMAGE_EMBED", true),
        EnableFactCheck:      GetEnvBool("ENABLE_FACT_CHECK", false),
        EnableSummarization: GetEnvBool("ENABLE_SUMMARIZATION", false),
        EnableContentFiltering: GetEnvBool("ENABLE_CONTENT_FILTERING", true),
        EnableKeywordTracking: GetEnvBool("ENABLE_KEYWORD_TRACKING", true),
        EnableDatabase:       GetEnvBool("ENABLE_DATABASE", true),
        EnableDashboard:      GetEnvBool("ENABLE_DASHBOARD", true),
        DashboardPort:        GetEnvInt("DASHBOARD_PORT", 8080),
        HealthAPIPort:        GetEnvInt("HEALTH_API_PORT", 8081),
        UserAgentString:      GetEnvString("USER_AGENT", "SankareaBotNews/1.0"),
        FetchNewsOnStartup:   GetEnvBool("FETCH_NEWS_ON_STARTUP", true),
        OpenAIAPIKey:         GetEnvString("OPENAI_API_KEY", ""),
        GoogleFactCheckAPIKey: GetEnvString("GOOGLE_FACT_CHECK_API_KEY", ""),
        ClaimBustersAPIKey:   GetEnvString("CLAIM_BUSTERS_API_KEY", ""),
    }
}

// ValidateEnvConfig validates the environment configuration
func ValidateEnvConfig(cfg *Config) error {
    if cfg.BotToken == "" {
        return fmt.Errorf("BOT_TOKEN is required")
    }
    if cfg.AppID == "" {
        return fmt.Errorf("APP_ID is required")
    }
    if cfg.NewsChannelID == "" {
        return fmt.Errorf("NEWS_CHANNEL_ID is required")
    }
    if len(cfg.OwnerIDs) == 0 {
        return fmt.Errorf("at least one OWNER_ID is required")
    }
    return nil
}

// InitializeEnvironment sets up the environment
func InitializeEnvironment() error {
    // Create required directories
    dirs := []string{
        "data",
        "data/images",
        "data/cache",
        "data/logs",
        "data/analytics",
        "data/user_filters",
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %v", dir, err)
        }
    }

    return nil
}
