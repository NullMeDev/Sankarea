package main

import (
	"net/http"
	"sync"
	"time"
)

// Source represents a news source
type Source struct {
	Name            string    `json:"name" yaml:"name"`
	URL             string    `json:"url" yaml:"url"`
	Category        string    `json:"category" yaml:"category"`
	Description     string    `json:"description" yaml:"description"`
	Bias           string     `json:"bias" yaml:"bias"`
	TrustScore     float64    `json:"trustScore" yaml:"trustScore"`
	ChannelOverride string    `json:"channelOverride" yaml:"channelOverride"`
	Paused         bool       `json:"paused" yaml:"paused"`
	Active         bool       `json:"active" yaml:"active"`
	Tags           []string   `json:"tags" yaml:"tags"`
	LastFetched    time.Time  `json:"lastFetched" yaml:"lastFetched"`
	LastError      string     `json:"lastError" yaml:"lastError"`
	LastErrorTime  time.Time  `json:"lastErrorTime" yaml:"lastErrorTime"`
	ErrorCount     int        `json:"errorCount" yaml:"errorCount"`
	FeedCount      int        `json:"feedCount" yaml:"feedCount"`
	UptimePercent  float64    `json:"uptimePercent" yaml:"uptimePercent"`
	AvgResponseTime int64     `json:"avgResponseTime" yaml:"avgResponseTime"`
}

// Config represents the application configuration
type Config struct {
	Version              string   `json:"version"`
	BotToken             string   `json:"botToken"`
	AppID                string   `json:"appId"`
	GuildID              string   `json:"guildId"`
	OwnerIDs             []string `json:"ownerIds"`
	NewsChannelID        string   `json:"newsChannelId"`
	ErrorChannelID       string   `json:"errorChannelId"`
	NewsIntervalMinutes  int      `json:"newsIntervalMinutes"`
	News15MinCron        string   `json:"news15MinCron"`
	DigestCronSchedule   string   `json:"digestCronSchedule"`
	MaxPostsPerSource    int      `json:"maxPostsPerSource"`
	EnableImageEmbed     bool     `json:"enableImageEmbed"`
	EnableFactCheck      bool     `json:"enableFactCheck"`
	EnableSummarization  bool     `json:"enableSummarization"`
	EnableContentFiltering bool   `json:"enableContentFiltering"`
	EnableKeywordTracking bool    `json:"enableKeywordTracking"`
	EnableDatabase       bool     `json:"enableDatabase"`
	EnableDashboard      bool     `json:"enableDashboard"`
	DashboardPort        int      `json:"dashboardPort"`
	HealthAPIPort        int      `json:"healthApiPort"`
	UserAgentString      string   `json:"userAgentString"`
	FetchNewsOnStartup   bool     `json:"fetchNewsOnStartup"`
	GoogleFactCheckAPIKey string  `json:"googleFactCheckApiKey"`
	ClaimBustersAPIKey    string  `json:"claimBustersApiKey"`
}

// State represents the application state
type State struct {
	Paused         bool      `json:"paused"`
	PausedBy       string    `json:"pausedBy"`
	LastFetchTime  time.Time `json:"lastFetchTime"`
	LastDigest     time.Time `json:"lastDigest"`
	NewsNextTime   time.Time `json:"newsNextTime"`
	DigestNextTime time.Time `json:"digestNextTime"`
	StartupTime    time.Time `json:"startupTime"`
	ShutdownTime   time.Time `json:"shutdownTime"`
	Version        string    `json:"version"`
	FeedCount      int       `json:"feedCount"`
	DigestCount    int       `json:"digestCount"`
	ErrorCount     int       `json:"errorCount"`
	TotalArticles  int       `json:"totalArticles"`
	TotalErrors    int       `json:"totalErrors"`
	TotalAPICalls  int       `json:"totalApiCalls"`
	LastError      string    `json:"lastError"`
	LastErrorTime  time.Time `json:"lastErrorTime"`
	LastInterval   int       `json:"lastInterval"`
	Lockdown       bool      `json:"lockdown"`
	LockdownSetBy  string    `json:"lockdownSetBy"`
}

// ImageDownloader handles downloading and caching images from feeds
type ImageDownloader struct {
	cacheDir string
	client   *http.Client
	mutex    sync.Mutex
}

// NewImageDownloader creates a new image downloader
func NewImageDownloader() *ImageDownloader {
	return &ImageDownloader{
		cacheDir: "data/images",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Initialize sets up the image downloader
func (id *ImageDownloader) Initialize() error {
	return os.MkdirAll(id.cacheDir, 0755)
}

// UserFilterManager handles user-specific news filtering
type UserFilterManager struct {
	filterDir string
	filters   map[string]*UserFilter
	mutex     sync.RWMutex
}

// UserFilter represents a user's news filtering preferences
type UserFilter struct {
	UserID           string            `json:"userId"`
	DisabledSources  []string          `json:"disabledSources"`
	DisabledCategories []string        `json:"disabledCategories"`
	IncludeKeywords  []string          `json:"includeKeywords"`
	ExcludeKeywords  []string          `json:"excludeKeywords"`
	LastUpdated      time.Time         `json:"lastUpdated"`
}

// NewUserFilterManager creates a new user filter manager
func NewUserFilterManager() *UserFilterManager {
	return &UserFilterManager{
		filterDir: "data/user_filters",
		filters:   make(map[string]*UserFilter),
	}
}

// Initialize loads all user filters
func (ufm *UserFilterManager) Initialize() error {
	return os.MkdirAll(ufm.filterDir, 0755)
}

// HealthMonitor tracks system health
type HealthMonitor struct {
	client *http.Client
	ticker *time.Ticker
	mutex  sync.Mutex
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// StartPeriodicChecks begins periodic health checks
func (hm *HealthMonitor) StartPeriodicChecks(interval time.Duration) {
	hm.ticker = time.NewTicker(interval)
	go func() {
		for range hm.ticker.C {
			hm.PerformChecks()
		}
	}()
}

// StopChecks stops the health check ticker
func (hm *HealthMonitor) StopChecks() {
	if hm.ticker != nil {
		hm.ticker.Stop()
	}
}

// PerformChecks runs health checks on various components
func (hm *HealthMonitor) PerformChecks() {
	// In a real implementation, this would check various system metrics
	// and report issues to the error system
}

// KeywordTracker tracks news keywords
type KeywordTracker struct {
	dataFile     string
	keywords     map[string]*KeywordStats
	mutex        sync.RWMutex
}

// KeywordStats tracks statistics for a keyword
type KeywordStats struct {
	Keyword      string    `json:"keyword"`
	Count        int       `json:"count"`
	LastSeen     time.Time `json:"lastSeen"`
	Sources      []string  `json:"sources"`
	Categories   []string  `json:"categories"`
}

// NewKeywordTracker creates a new keyword tracker
func NewKeywordTracker() *KeywordTracker {
	return &KeywordTracker{
		dataFile: "data/keywords.json",
		keywords: make(map[string]*KeywordStats),
	}
}

// Initialize loads keyword tracking data
func (kt *KeywordTracker) Initialize() error {
	// In a real implementation, this would load keywords from JSON
	return nil
}

// Save persists keyword data
func (kt *KeywordTracker) Save() error {
	// In a real implementation, this would save keywords to JSON
	return nil
}

// CheckForKeywords scans text for tracked keywords
func (kt *KeywordTracker) CheckForKeywords(text string) {
	// In a real implementation, this would check for matches
}

// DigestManager handles digest creation and scheduling
type DigestManager struct {
	cronSchedule  string
	cronJob       cron.EntryID
	mutex         sync.Mutex
}

// NewDigestManager creates a new digest manager
func NewDigestManager() *DigestManager {
	return &DigestManager{
		cronSchedule: "0 8 * * *", // Default to 8 AM daily
	}
}

// StartScheduler starts the digest scheduler
func (dm *DigestManager) StartScheduler(s *discordgo.Session) error {
	// In a real implementation, this would add a cron job
	return nil
}

// LanguageManager handles translations
type LanguageManager struct {
	translations map[string]map[string]string
	mutex        sync.RWMutex
}

// NewLanguageManager creates a new language manager
func NewLanguageManager() *LanguageManager {
	return &LanguageManager{
		translations: make(map[string]map[string]string),
	}
}

// Initialize loads translation data
func (lm *LanguageManager) Initialize() error {
	// In a real implementation, this would load translations
	return nil
}

// CredibilityScorer scores news source credibility
type CredibilityScorer struct {
	scores map[string]float64
	mutex  sync.RWMutex
}

// NewCredibilityScorer creates a new credibility scorer
func NewCredibilityScorer() *CredibilityScorer {
	return &CredibilityScorer{
		scores: make(map[string]float64),
	}
}

// Initialize loads credibility data
func (cs *CredibilityScorer) Initialize() error {
	// In a real implementation, this would load credibility data
	return nil
}

// Save persists credibility scores
func (cs *CredibilityScorer) Save() error {
	// In a real implementation, this would save credibility data
	return nil
}

// AnalyticsEngine tracks bot usage
type AnalyticsEngine struct {
	dataDir string
	mutex   sync.Mutex
}

// NewAnalyticsEngine creates a new analytics engine
func NewAnalyticsEngine() *AnalyticsEngine {
	return &AnalyticsEngine{
		dataDir: "data/analytics",
	}
}

// Initialize sets up the analytics engine
func (ae *AnalyticsEngine) Initialize() error {
	return os.MkdirAll(ae.dataDir, 0755)
}

// Save persists analytics data
func (ae *AnalyticsEngine) Save() error {
	// In a real implementation, this would save analytics data
	return nil
}

// ConfigManager handles automatic configuration reloading
type ConfigManager struct {
	configPath   string
	interval     time.Duration
	ticker       *time.Ticker
	reloadFunc   func(*Config)
	mutex        sync.Mutex
}

// NewConfigManager creates a new config manager
func NewConfigManager(configPath string, interval time.Duration) (*ConfigManager, error) {
	if interval < time.Second {
		interval = time.Minute
	}
	
	return &ConfigManager{
		configPath: configPath,
		interval:   interval,
	}, nil
}

// SetReloadHandler sets the function to call when config is reloaded
func (cm *ConfigManager) SetReloadHandler(fn func(*Config)) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.reloadFunc = fn
}

// StartWatching begins watching the config file for changes
func (cm *ConfigManager) StartWatching() {
	cm.ticker = time.NewTicker(cm.interval)
	go func() {
		for range cm.ticker.C {
			cm.checkConfigChanged()
		}
	}()
}

// StopWatching stops watching the config file
func (cm *ConfigManager) StopWatching() {
	if cm.ticker != nil {
		cm.ticker.Stop()
	}
}

// checkConfigChanged checks if the config file has changed
func (cm *ConfigManager) checkConfigChanged() {
	// In a real implementation, this would check file modification time
	// and reload the config if it has changed
}
