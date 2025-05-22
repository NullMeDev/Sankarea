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
	ClaimBustersAPIKey   string          `json:"claimbusters_api_key"`
	YouTubeAPIKey        string          `json:"youtube_api_key"` 
	TwitterBearerToken   string          `json:"twitter_bearer_token"`
	
	// Feature Toggles
	EnableFactCheck      bool            `json:"enableFactCheck"`
	EnableSummarization  bool            `json:"enableSummarization"`
	EnableContentFiltering bool          `json:"enableContentFiltering"`
	EnableDatabase       bool            `json:"enableDatabase"`
	
	// Advanced Configuration
	UserAgentString      string          `json:"userAgentString"`
	HealthAPIPort        int             `json:"healthAPIPort"`
	
	// Report Configuration
	Reports              ReportConfig    `json:"reports"`
}

// LoadConfig loads the application configuration from file and environment variables
func LoadConfig() (*Config, error) {
	// Default config
	cfg := &Config{
		Version:             VERSION,
		MaxPostsPerSource:   5,
		NewsIntervalMinutes: 120,
		DigestCronSchedule:  "0 8 * * *", // 8 AM daily
		News15MinCron:       "*/15 * * * *", // Every 15 minutes
		UserAgentString:     "Sankarea News Bot v" + VERSION,
		Reports: ReportConfig{
			Enabled:     true,
			WeeklyCron:  "0 9 * * 1", // Monday at 9 AM
			MonthlyCron: "0 9 1 * *", // 1st day of month at 9 AM
		},
	}

	// Load from file if it exists
	data, err := ioutil.ReadFile(configFilePath)
	if err == nil {
		// File exists, try to parse it
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	if token := os.Getenv("DISCORD_BOT_TOKEN"); token != "" {
		cfg.BotToken = token
	}
	if appID := os.Getenv("DISCORD_APPLICATION_ID"); appID != "" {
		cfg.AppID = appID
	}
	if guildID := os.Getenv("DISCORD_GUILD_ID"); guildID != "" {
		cfg.GuildID = guildID
	}
	if channelID := os.Getenv("DISCORD_CHANNEL_ID"); channelID != "" {
		cfg.NewsChannelID = channelID
	}
	if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
		cfg.OpenAIAPIKey = openAIKey
	}
	if googleFactCheckKey := os.Getenv("GOOGLE_FACTCHECK_API_KEY"); googleFactCheckKey != "" {
		cfg.GoogleFactCheckAPIKey = googleFactCheckKey
	}
	if claimBusterKey := os.Getenv("CLAIMBUSTER_API_KEY"); claimBusterKey != "" {
		cfg.ClaimBustersAPIKey = claimBusterKey
	}
	if youtubeKey := os.Getenv("YOUTUBE_API_KEY"); youtubeKey != "" {
		cfg.YouTubeAPIKey = youtubeKey
	}
	if twitterToken := os.Getenv("TWITTER_BEARER_TOKEN"); twitterToken != "" {
		cfg.TwitterBearerToken = twitterToken
	}

	// Parse webhook URLs
	if webhooks := os.Getenv("DISCORD_WEBHOOKS"); webhooks != "" {
		// This would be used for additional notifications
	}

	return cfg, nil
}

// LoadSources loads all news sources from the sources file
func LoadSources() ([]Source, error) {
	data, err := ioutil.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}

	var sources []Source
	if err := yaml.Unmarshal(data, &sources); err != nil {
		return nil, err
	}

	// Set default values for any sources
	for i := range sources {
		if !sources[i].Paused && sources[i].Active == false {
			sources[i].Active = true
		}
	}

	return sources, nil
}

// SaveSources saves the sources back to the sources file
func SaveSources(sources []Source) error {
	// If we received a subset of sources, merge with existing
	if len(sources) < 10 { // Assuming we have more than 10 total sources
		existingSources, err := LoadSources()
		if err == nil {
			// This is very simplistic and would need to be improved
			// to properly handle merging with existing sources
			sourceMap := make(map[string]Source)
			
			// Add existing sources to map
			for _, src := range existingSources {
				sourceMap[src.Name] = src
			}
			
			// Update with new sources
			for _, src := range sources {
				sourceMap[src.Name] = src
			}
			
			// Convert back to slice
			sources = []Source{}
			for _, src := range sourceMap {
				sources = append(sources, src)
			}
		}
	}

	data, err := yaml.Marshal(sources)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(sourcesFilePath, data, 0644)
}

// LoadEnv loads environment variables from .env file if it exists
func LoadEnv() {
	// No need for external libraries, just check if file exists
	data, err := ioutil.ReadFile(".env")
	if err != nil {
		return
	}

	// Parse simple KEY=VALUE format
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) > 1 && (value[0] == '"' || value[0] == '\'') && value[0] == value[len(value)-1] {
			value = value[1 : len(value)-1]
		}
		
		os.Setenv(key, value)
	}
}

// FileMustExist checks if a required file exists, creates it with defaults if not
func FileMustExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create empty file
		switch path {
		case configFilePath:
			// Create default config
			cfg := &Config{
				Version:             VERSION,
				MaxPostsPerSource:   5,
				NewsIntervalMinutes: 120,
				DigestCronSchedule:  "0 8 * * *", // 8 AM daily
				News15MinCron:       "*/15 * * * *", // Every 15 minutes
				UserAgentString:     "Sankarea News Bot v" + VERSION,
				EnableFactCheck:     true,
				EnableSummarization: true,
			}
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				log.Fatalf("Failed to create default config: %v", err)
			}
			if err := ioutil.WriteFile(path, data, 0644); err != nil {
				log.Fatalf("Failed to write default config: %v", err)
			}
		case sourcesFilePath:
			// Create example sources
			sources := []Source{
				{
					Name:     "Example News",
					URL:      "https://example.com/rss",
					Paused:   true,
					Category: "General",
					Active:   true,
				},
			}
			data, err := yaml.Marshal(sources)
			if err != nil {
				log.Fatalf("Failed to create default sources: %v", err)
			}
			if err := ioutil.WriteFile(path, data, 0644); err != nil {
				log.Fatalf("Failed to write default sources: %v", err)
			}
		default:
			// Create empty file
			if err := ioutil.WriteFile(path, []byte{}, 0644); err != nil {
				log.Fatalf("Failed to create file %s: %v", path, err)
			}
		}
	}
}
