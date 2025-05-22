package main

import (
	"encoding/json"
	"os"
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
}

// Config holds application configuration
type Config struct {
	// Bot Configuration
	BotToken             string   `json:"bot_token"`
	AppID                string   `json:"app_id"`
	GuildID              string   `json:"guild_id"`
	Version              string   `json:"version"`
	MaxPostsPerSource    int      `json:"maxPostsPerSource"`
	OwnerIDs             []string `json:"ownerIDs"` // Discord User IDs who have owner permissions
	AdminRoleIDs         []string `json:"adminRoleIDs"` // Discord Role IDs that have admin permissions
	
	// Channels Configuration
	NewsChannelID        string   `json:"newsChannelId"`
	DigestChannelID      string   `json:"digestChannelId"` // Can be same as NewsChannelID
	AuditLogChannelID    string   `json:"auditLogChannelId"` 
	ErrorChannelID       string   `json:"errorChannelId"`
	
	// Schedule Configuration
	NewsIntervalMinutes  int      `json:"newsIntervalMinutes"` // Default: 120 (2 hours)
	DigestCronSchedule   string   `json:"digestCronSchedule"`  // Default: "0 8 * * *" (8 AM daily)
	
	// API Keys and Integration
	OpenAIAPIKey         string   `json:"openai_api_key"`
	GoogleFactCheckAPIKey string  `json:"google_factcheck_api_key"`
	ClaimBustersAPIKey   string   `json:"claimbusters_api_key"`
	
	// Feature Toggles
	EnableFactCheck      bool     `json:"enableFactCheck"`
	EnableSummarization  bool     `json:"enableSummarization"`
	EnableContentFiltering bool   `json:"enableContentFiltering"`
	
	// Advanced Configuration
	UserAgentString      string   `json:"userAgentString"`
	MaxRetries           int      `json:"maxRetries"`
	RetryDelaySeconds    int      `json:"retryDelaySeconds"`
	MaxErrorsBeforePause int      `json:"maxErrorsBeforePause"`
	HTTPTimeoutSeconds   int      `json:"httpTimeoutSeconds"`
	CacheExpiryHours     int      `json:"cacheExpiryHours"`
}

// State represents the bot's runtime state
type State struct {
	Paused          bool      `json:"paused"`
	LastDigest      time.Time `json:"lastDigest"`
	LastInterval    int       `json:"lastInterval"`
	LastError       string    `json:"lastError"`
	NewsNextTime    time.Time `json:"newsNextTime"`
	FeedCount       int       `json:"feedCount"`
	Lockdown        bool      `json:"lockdown"`
	LockdownSetBy   string    `json:"lockdownSetBy"`
	Version         string    `json:"version"`
	StartupTime     time.Time `json:"startupTime"`
	ErrorCount      int       `json:"errorCount"`
	TotalArticles   int       `json:"totalArticles"`
	ArticleCache    []string  `json:"articleCache"` // To prevent duplicates
	LastSummaryCost float64   `json:"lastSummaryCost"`
	DailyAPIUsage   float64   `json:"dailyAPIUsage"`
	LastAPIReset    time.Time `json:"lastAPIReset"`
}

// LoadConfig loads application configuration
func LoadConfig() (*Config, error) {
	if err := os.MkdirAll("config", 0755); err != nil {
		return nil, err
	}
	
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// Create default config
		defaultCfg := &Config{
			Version:             VERSION,
			MaxPostsPerSource:   5,
			NewsIntervalMinutes: 120, // 2 hours default
			DigestCronSchedule:  "0 8 * * *", // 8 AM daily
			MaxRetries:          3,
			RetryDelaySeconds:   30,
			HTTPTimeoutSeconds:  30,
			CacheExpiryHours:    24,
			UserAgentString:     "Sankarea-Bot/1.0",
			EnableFactCheck:     false, // Disabled until API keys are provided
			EnableSummarization: false, // Disabled until API keys are provided
			MaxErrorsBeforePause: 5,
		}
		
		if err := SaveConfig(defaultCfg); err != nil {
			return nil, err
		}
		return defaultCfg, nil
	}
	
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	
	// Ensure version is set
	if cfg.Version == "" {
		cfg.Version = VERSION
	}
	
	// Set default values if not present
	if cfg.MaxPostsPerSource <= 0 {
		cfg.MaxPostsPerSource = 5
	}
	
	if cfg.NewsIntervalMinutes <= 0 {
		cfg.NewsIntervalMinutes = 120 // 2 hours
	}
	
	if cfg.DigestCronSchedule == "" {
		cfg.DigestCronSchedule = "0 8 * * *"
	}

	return &cfg, nil
}

// SaveConfig persists configuration
func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll("config", 0755); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configFilePath, data, 0644)
}

// LoadSources loads RSS feed sources
func LoadSources() ([]Source, error) {
	if err := os.MkdirAll("config", 0755); err != nil {
		return nil, err
	}
	
	if _, err := os.Stat(sourcesFilePath); os.IsNotExist(err) {
		// Create empty sources file
		if err := SaveSources([]Source{}); err != nil {
			return nil, err
		}
		return []Source{}, nil
	}
	
	data, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}
	
	var srcs []Source
	if err := yaml.Unmarshal(data, &srcs); err != nil {
		return nil, err
	}
	
	// Ensure all sources have proper initialization
	for i := range srcs {
		if srcs[i].TrustScore == 0 {
			srcs[i].TrustScore = 0.5 // Default neutral trust score
		}
	}
	
	return srcs, nil
}

// SaveSources persists feed sources
func SaveSources(srcs []Source) error {
	if err := os.MkdirAll("config", 0755); err != nil {
		return err
	}
	
	data, err := yaml.Marshal(srcs)
	if err != nil {
		return err
	}
	
	return os.WriteFile(sourcesFilePath, data, 0644)
}

// LoadState loads bot runtime state
func LoadState() (*State, error) {
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, err
	}
	
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		// Create default state
		defaultState := &State{
			Version:     VERSION,
			StartupTime: time.Now(),
			LastAPIReset: time.Now(),
		}
		
		if err := SaveState(defaultState); err != nil {
			return nil, err
		}
		return defaultState, nil
	}
	
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}
	
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	
	// Check if API usage should be reset (daily)
	now := time.Now()
	if now.Sub(st.LastAPIReset).Hours() >= 24 {
		st.LastAPIReset = now
		st.DailyAPIUsage = 0
		if err := SaveState(&st); err != nil {
			Logger().Printf("Failed to save state after API usage reset: %v", err)
		}
	}
	
	return &st, nil
}

// SaveState persists bot runtime state
func SaveState(st *State) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(stateFilePath, data, 0644)
}

// AddToCacheAndCheck adds an article URL to cache and checks if it's already cached
// Returns true if article was already in the cache
func AddToCacheAndCheck(url string) (bool, error) {
	state, err := LoadState()
	if err != nil {
		return false, err
	}
	
	// Check if URL exists in cache
	for _, cachedURL := range state.ArticleCache {
		if cachedURL == url {
			return true, nil
		}
	}
	
	// Add to cache
	state.ArticleCache = append(state.ArticleCache, url)
	
	// Trim cache if it gets too large (keep last 1000 articles)
	if len(state.ArticleCache) > 1000 {
		state.ArticleCache = state.ArticleCache[len(state.ArticleCache)-1000:]
	}
	
	// Save state
	if err := SaveState(state); err != nil {
		return false, err
	}
	
	return false, nil
}
