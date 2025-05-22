package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	LastErrorTime   time.Time `json:"lastErrorTime" yaml:"lastErrorTime"`
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
	Language        string    `json:"language" yaml:"language"`
	UptimePercent   float64   `json:"uptimePercent" yaml:"uptimePercent"`
	AvgResponseTime int64     `json:"avgResponseTime" yaml:"avgResponseTime"`
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
	NewsAPIKey           string          `json:"newsapi_key"`
	GNewsAPIKey          string          `json:"gnews_api_key"`
	
	// Feature Flags
	EnableFactCheck      bool            `json:"enable_factcheck"`
	EnableSummarization  bool            `json:"enable_summarization"`
	EnableDatabase       bool            `json:"enable_database"`
	EnableDashboard      bool            `json:"enable_dashboard"`
	EnableMultiLanguage  bool            `json:"enable_multilanguage"`
	EnableImageEmbed     bool            `json:"enable_image_embed"`
	EnableKeywordTracking bool           `json:"enable_keyword_tracking"`
	EnableAnalytics      bool            `json:"enable_analytics"`
	FetchNewsOnStartup   bool            `json:"fetch_news_on_startup"`
	
	// Database Configuration
	DatabaseURL          string          `json:"database_url"`
	
	// Health Monitoring
	HealthAPIPort        int             `json:"health_api_port"`
	EnableHealthMonitor  bool            `json:"enable_health_monitor"`
	
	// Rate Limiting
	MaxAPIRequestsPerMinute int          `json:"max_api_requests_per_minute"`
	MaxRequestsPerUser      int          `json:"max_requests_per_user"`
	
	// User Preferences
	DefaultLanguage      string          `json:"default_language"`
	SupportedLanguages   []string        `json:"supported_languages"`
	DefaultDigestFormat  string          `json:"default_digest_format"`
	
	// Clustering & Analytics
	TopicClusterThreshold float64        `json:"topic_cluster_threshold"`
	EnableSentimentAnalysis bool         `json:"enable_sentiment_analysis"`
	
	// Error Recovery
	MaxRetryCount        int             `json:"max_retry_count"`
	RetryDelaySeconds    int             `json:"retry_delay_seconds"`
}

// ChannelConfig represents a Discord channel configuration
type ChannelConfig struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Language   string   `json:"language"`
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
			EnableDashboard:     true,
			MaxRetryCount:       3,
			RetryDelaySeconds:   30,
			DefaultLanguage:     "en",
			SupportedLanguages:  []string{"en", "es", "fr", "de", "ja"},
			DefaultDigestFormat: "compact",
			EnableHealthMonitor: true,
			FetchNewsOnStartup:  true,
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
	
	// Ensure version is set
	if config.Version == "" {
		config.Version = VERSION
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
	// Check if file exists, create if not
	if _, err := os.Stat(sourcesFilePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
			return nil, err
		}
		
		// Create empty file with empty YAML sources array
		if err := os.WriteFile(sourcesFilePath, []byte("sources: []"), 0644); err != nil {
			return nil, err
		}
		
		return []Source{}, nil
	}
	
	// Read sources file
	data, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}
	
	var sources struct {
		Sources []Source `yaml:"sources"`
	}
	
	if err := yaml.Unmarshal(data, &sources); err != nil {
		// Try with direct unmarshaling if the structure doesn't match
		var directSources []Source
		if err := yaml.Unmarshal(data, &directSources); err != nil {
			return nil, fmt.Errorf("failed to parse sources file: %w", err)
		}
		sources.Sources = directSources
	}
	
	// Ensure all sources have active flag set properly
	for i := range sources.Sources {
		if !sources.Sources[i].Paused && sources.Sources[i].Active == false {
			sources.Sources[i].Active = true
		}
	}
	
	return sources.Sources, nil
}

// SaveSources saves RSS feed sources to the sources.yml file
func SaveSources(sources []Source) error {
	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(sourcesFilePath), 0755); err != nil {
		return err
	}
	
	// Wrap sources in a proper YAML structure
	wrappedSources := struct {
		Sources []Source `yaml:"sources"`
	}{
		Sources: sources,
	}
	
	// Marshal sources to YAML
	data, err := yaml.Marshal(wrappedSources)
	if err != nil {
		return err
	}
	
	// Write to file
	return os.WriteFile(sourcesFilePath, data, 0644)
}

// ConfigManager watches for config changes and reloads when necessary
type ConfigManager struct {
	configPath    string
	checkInterval time.Duration
	lastModTime   time.Time
	reloadHandler func(*Config)
	stopChan      chan struct{}
}

// NewConfigManager creates a new config manager
func NewConfigManager(configPath string, checkInterval time.Duration) (*ConfigManager, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}
	
	return &ConfigManager{
		configPath:    configPath,
		checkInterval: checkInterval,
		lastModTime:   info.ModTime(),
		stopChan:      make(chan struct{}),
	}, nil
}

// SetReloadHandler sets the function to call when config is reloaded
func (m *ConfigManager) SetReloadHandler(handler func(*Config)) {
	m.reloadHandler = handler
}

// StartWatching starts watching for config changes
func (m *ConfigManager) StartWatching() {
	go func() {
		ticker := time.NewTicker(m.checkInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				m.checkForChanges()
			case <-m.stopChan:
				return
			}
		}
	}()
}

// Stop stops watching for config changes
func (m *ConfigManager) Stop() {
	close(m.stopChan)
}

// checkForChanges checks if the config file has changed
func (m *ConfigManager) checkForChanges() {
	info, err := os.Stat(m.configPath)
	if err != nil {
		Logger().Printf("Error checking config file: %v", err)
		return
	}
	
	if info.ModTime().After(m.lastModTime) {
		Logger().Println("Config file changed, reloading...")
		m.lastModTime = info.ModTime()
		
		config, err := LoadConfig()
		if err != nil {
			Logger().Printf("Error reloading config: %v", err)
			return
		}
		
		if m.reloadHandler != nil {
			m.reloadHandler(config)
		}
	}
}
