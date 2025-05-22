package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	configFilePath  = "config/config.json"
	sourcesFilePath = "config/sources.yml"
	stateFilePath   = "data/state.json"
	VERSION         = "1.0.0"
)

// Source represents an RSS feed source
type Source struct {
	Name            string    `json:"name" yaml:"name"`
	URL             string    `json:"url" yaml:"url"`
	Paused          bool      `json:"paused" yaml:"paused"`
	Active          bool      `json:"active" yaml:"active"`
	LastDigest      time.Time `json:"lastDigest" yaml:"lastDigest"`
	LastFetched     time.Time `json:"lastFetched" yaml:"lastFetched"`
	LastInterval    int       `json:"lastInterval" yaml:"lastInterval"`
	LastError       string    `json:"lastError" yaml:"lastError"`
	NewsNextTime    time.Time `json:"newsNextTime" yaml:"newsNextTime"`
	FeedCount       int       `json:"feedCount" yaml:"feedCount"`
	Lockdown        bool      `json:"lockdown" yaml:"lockdown"`
	LockdownSetBy   string    `json:"lockdownSetBy" yaml:"lockdownSetBy"`
	ErrorCount      int       `json:"errorCount" yaml:"errorCount"`
	Category        string    `json:"category" yaml:"category"`
	FactCheckAuto   bool      `json:"factCheckAuto" yaml:"factCheckAuto"`
	SummarizeAuto   bool      `json:"summarizeAuto" yaml:"summarizeAuto"`
	TrustScore      float64   `json:"trustScore" yaml:"trustScore"`
	ChannelOverride string    `json:"channelOverride" yaml:"channelOverride"`
	Bias            string    `json:"bias" yaml:"bias"`
}

// Config holds application configuration
type Config struct {
	// Bot Configuration
	BotToken             string          `json:"bot_token"`
	AppID                string          `json:"app_id"`
	GuildID              string          `json:"guild_id"`
	Version              string          `json:"version"`
	MaxPostsPerSource    int             `json:"maxPostsPerSource"`
	OwnerIDs             []string        `json:"ownerIDs"` // Discord User IDs who have owner permissions
	AdminRoleIDs         []string        `json:"adminRoleIDs"` // Discord Role IDs that have admin permissions
	
	// Channels Configuration
	NewsChannelID        string          `json:"newsChannelId"`
	DigestChannelID      string          `json:"digestChannelId"` // Can be same as NewsChannelID
	AuditLogChannelID    string          `json:"auditLogChannelId"` 
	ErrorChannelID       string          `json:"errorChannelId"`
	Channels             []ChannelConfig `json:"channels"` // Additional channel configurations
	
	// Schedule Configuration
	NewsIntervalMinutes  int             `json:"newsIntervalMinutes"` // Default: 120 (2 hours)
	DigestCronSchedule   string          `json:"digestCronSchedule"`  // Default: "0 8 * * *" (8 AM daily)
	News15MinCron        string          `json:"news15MinCron"`      // Cron schedule for news updates
	
	// API Keys and Integration
	OpenAIAPIKey         string          `json:"openai_api_key"`
	GoogleFactCheckAPIKey string         `json:"google_factcheck_api_key"`
	ClaimBustersAPIKey   string          `json:"claimbuster_api_key"`
	
	// Feature Flags
	EnableFactCheck      bool            `json:"enable_factcheck"`
	EnableSummarization  bool            `json:"enable_summarization"`
	EnableDatabase       bool            `json:"enable_database"`
	
	// Health Monitoring
	HealthAPIPort        int             `json:"health_api_port"`
	EnableHealthMonitor  bool            `json:"enable_health_monitor"`
}

// ChannelConfig represents a Discord channel configuration
type ChannelConfig struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
}

// LoadConfig loads the application configuration from file
func LoadConfig() (*Config, error) {
	// Load default configuration if file doesn't exist
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		defaultConfig := &Config{
			Version:             VERSION,
			MaxPostsPerSource:   5,
			NewsIntervalMinutes: 120,
			DigestCronSchedule:  "0 8 * * *", // 8 AM daily
			News15MinCron:       "*/15 * * * *", // Every 15 minutes
		}
		
		// Check for environment variables to override defaults
		if token := os.Getenv("DISCORD_BOT_TOKEN"); token != "" {
			defaultConfig.BotToken = token
		}
		
		if appID := os.Getenv("DISCORD_APPLICATION_ID"); appID != "" {
			defaultConfig.AppID = appID
		}
		
		if guildID := os.Getenv("DISCORD_GUILD_ID"); guildID != "" {
			defaultConfig.GuildID = guildID
		}
		
		if channelID := os.Getenv("DISCORD_CHANNEL_ID"); channelID != "" {
			defaultConfig.NewsChannelID = channelID
			defaultConfig.DigestChannelID = channelID
		}
		
		if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
			defaultConfig.OpenAIAPIKey = openAIKey
			defaultConfig.EnableSummarization = true
		}
		
		// Save default config
		if err := SaveConfig(defaultConfig); err != nil {
			return nil, err
		}
		
		return defaultConfig, nil
	}
	
	// Read config file
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// SaveConfig saves the configuration to file
func SaveConfig(config *Config) error {
	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
		return err
	}
	
	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file
	return os.WriteFile(configFilePath, data, 0644)
}

// LoadSources loads RSS feed sources from the sources.yml file
func LoadSources() ([]Source, error) {
	// Check if file exists, create empty file if not
	if _, err := os.Stat(sourcesFilePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
			return nil, err
		}
		
		// Create empty file with empty sources array
		if err := os.WriteFile(sourcesFilePath, []byte("[]"), 0644); err != nil {
			return nil, err
		}
		
		return []Source{}, nil
	}
	
	// Read sources file
	data, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}
	
	var sources []Source
	if err := yaml.Unmarshal(data, &sources); err != nil {
		return nil, err
	}
	
	return sources, nil
}

// SaveSources saves RSS feed sources to the sources.yml file
func SaveSources(sources []Source) error {
	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
		return err
	}
	
	// Marshal sources to YAML
	data, err := yaml.Marshal(sources)
	if err != nil {
		return err
	}
	
	// Write to file
	return os.WriteFile(sourcesFilePath, data, 0644)
}
