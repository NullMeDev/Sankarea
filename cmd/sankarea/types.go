package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	
	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

// Error severity levels
const (
	ErrorSeverityLow    = 1
	ErrorSeverityMedium = 2
	ErrorSeverityHigh   = 3
	ErrorSeverityFatal  = 4
)

// ErrorSystem handles error tracking and reporting
type ErrorSystem struct {
	errors      []ErrorRecord
	maxRecords  int
	errorsMutex sync.Mutex
}

// ErrorRecord represents a single error occurrence
type ErrorRecord struct {
	Message   string    `json:"message"`
	Error     string    `json:"error"`
	Component string    `json:"component"`
	Severity  int       `json:"severity"`
	Time      time.Time `json:"time"`
}

// NewErrorSystem creates a new error tracking system
func NewErrorSystem(maxRecords int) *ErrorSystem {
	return &ErrorSystem{
		errors:     make([]ErrorRecord, 0, maxRecords),
		maxRecords: maxRecords,
	}
}

// HandleError records and handles an error based on its severity
func (es *ErrorSystem) HandleError(message string, err error, component string, severity int) {
	if es == nil {
		// In case the error system itself is nil, log to stderr
		log.Printf("ERROR SYSTEM NOT INITIALIZED: %s: %v", message, err)
		if severity == ErrorSeverityFatal {
			log.Fatal(fmt.Sprintf("FATAL ERROR: %s: %v", message, err))
		}
		return
	}

	es.errorsMutex.Lock()
	defer es.errorsMutex.Unlock()

	errorMsg := "unknown error"
	if err != nil {
		errorMsg = err.Error()
	}

	// Create error record
	record := ErrorRecord{
		Message:   message,
		Error:     errorMsg,
		Component: component,
		Severity:  severity,
		Time:      time.Now(),
	}

	// Add to records
	es.errors = append(es.errors, record)
	if len(es.errors) > es.maxRecords {
		// Remove oldest error
		es.errors = es.errors[1:]
	}

	// Log the error
	Logger().Printf("[%s] %s: %v", severityString(severity), message, err)

	// Increment error count in global state
	IncrementErrorCount()
	RecordError(message + ": " + errorMsg)

	// Handle fatal errors by exiting the application
	if severity == ErrorSeverityFatal {
		Logger().Fatalf("FATAL ERROR: %s: %v", message, err)
	}
}

// GetErrors returns the recorded errors
func (es *ErrorSystem) GetErrors() []ErrorRecord {
	es.errorsMutex.Lock()
	defer es.errorsMutex.Unlock()

	// Return a copy to avoid race conditions
	result := make([]ErrorRecord, len(es.errors))
	copy(result, es.errors)
	return result
}

// severityString converts severity level to string representation
func severityString(severity int) string {
	switch severity {
	case ErrorSeverityLow:
		return "LOW"
	case ErrorSeverityMedium:
		return "MEDIUM"
	case ErrorSeverityHigh:
		return "HIGH"
	case ErrorSeverityFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
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

// UserFilterManager handles user preferences for filtering news
type UserFilterManager struct {
	filters     map[string]*UserFilter
	filtersDir  string
	filterMutex sync.Mutex
}

// UserFilter represents a user's filtering preferences
type UserFilter struct {
	UserID         string   `json:"userId"`
	DisabledSources []string `json:"disabledSources"`
	DisabledCategories []string `json:"disabledCategories"`
	IncludeKeywords []string `json:"includeKeywords"`
	ExcludeKeywords []string `json:"excludeKeywords"`
	LastUpdated    time.Time `json:"lastUpdated"`
}

// NewUserFilterManager creates a new user filter manager
func NewUserFilterManager() *UserFilterManager {
	return &UserFilterManager{
		filters:    make(map[string]*UserFilter),
		filtersDir: "data/user_filters",
	}
}

// Initialize sets up the user filter manager
func (ufm *UserFilterManager) Initialize() error {
	return os.MkdirAll(ufm.filtersDir, 0755)
}

// HealthMonitor monitors the health of the bot and its dependencies
type HealthMonitor struct {
	lastCheck time.Time
	status    map[string]bool
	mutex     sync.Mutex
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		status: make(map[string]bool),
	}
}

// StartPeriodicChecks starts periodic health checks
func (hm *HealthMonitor) StartPeriodicChecks(interval time.Duration) {
	go func() {
		defer RecoverFromPanic("health-monitor")
		
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				hm.PerformHealthCheck()
			}
		}
	}()
}

// PerformHealthCheck runs a health check on all components
func (hm *HealthMonitor) PerformHealthCheck() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	hm.lastCheck = time.Now()
	
	// Add health checks for various components here
	// For now, just mark the bot as healthy
	hm.status["bot"] = true
}

// KeywordTracker tracks keyword occurrences in news articles
type KeywordTracker struct {
	keywords     map[string]KeywordStats
	keywordsFile string
	mutex        sync.Mutex
}

// KeywordStats represents statistics for a tracked keyword
type KeywordStats struct {
	Keyword     string    `json:"keyword"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	Sources     []string  `json:"sources"`
	Categories  []string  `json:"categories"`
}

// NewKeywordTracker creates a new keyword tracker
func NewKeywordTracker() *KeywordTracker {
	return &KeywordTracker{
		keywords:     make(map[string]KeywordStats),
		keywordsFile: "data/keywords.json",
	}
}

// Initialize sets up the keyword tracker
func (kt *KeywordTracker) Initialize() error {
	// Create the data directory if it doesn't exist
	dir := filepath.Dir(kt.keywordsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Check if keywords file exists
	if _, err := os.Stat(kt.keywordsFile); os.IsNotExist(err) {
		// Create empty keywords file
		return kt.Save()
	}
	
	// Load existing keywords
	data, err := os.ReadFile(kt.keywordsFile)
	if err != nil {
		return err
	}
	
	if len(data) == 0 {
		// File exists but is empty
		return nil
	}
	
	// Parse keywords
	var keywords map[string]KeywordStats
	if err := json.Unmarshal(data, &keywords); err != nil {
		return err
	}
	
	kt.keywords = keywords
	return nil
}

// Save persists the keywords to disk
func (kt *KeywordTracker) Save() error {
	kt.mutex.Lock()
	defer kt.mutex.Unlock()
	
	// Marshal keywords to JSON
	data, err := json.MarshalIndent(kt.keywords, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file atomically to prevent corruption
	tempFile := kt.keywordsFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	
	// Then rename to the actual file
	return os.Rename(tempFile, kt.keywordsFile)
}

// CheckForKeywords checks a text for tracked keywords
func (kt *KeywordTracker) CheckForKeywords(text string) {
	kt.mutex.Lock()
	defer kt.mutex.Unlock()
	
	// Simple implementation: just check if each keyword is in the text
	lowerText := strings.ToLower(text)
	
	for keyword, stats := range kt.keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			// Update stats
			stats.Count++
			stats.LastSeen = time.Now()
			if stats.FirstSeen.IsZero() {
				stats.FirstSeen = time.Now()
			}
			kt.keywords[keyword] = stats
		}
	}
}

// DigestManager handles creation and scheduling of news digests
type DigestManager struct {
	scheduler *cron.Cron
	settings  map[string]*DigestSettings
}

// DigestSettings represents a user's digest settings
type DigestSettings struct {
	UserID     string   `json:"userId"`
	Enabled    bool     `json:"enabled"`
	Schedule   string   `json:"schedule"`
	MaxStories int      `json:"maxStories"`
	Categories []string `json:"categories"`
}

// NewDigestManager creates a new digest manager
func NewDigestManager() *DigestManager {
	return &DigestManager{
		scheduler: cron.New(),
		settings:  make(map[string]*DigestSettings),
	}
}

// StartScheduler starts the digest scheduler
func (dm *DigestManager) StartScheduler(session *discordgo.Session) error {
	// Start the scheduler
	dm.scheduler.Start()
	
	// Schedule default digest if configured
	if cfg != nil && cfg.DigestCronSchedule != "" {
		_, err := dm.scheduler.AddFunc(cfg.DigestCronSchedule, func() {
			if err := dm.GenerateDigest(session, cfg.DigestChannelID, nil); err != nil {
				Logger().Printf("Error generating scheduled digest: %v", err)
				if errorSystem != nil {
					errorSystem.HandleError("Scheduled digest generation failed", err, "digest", ErrorSeverityMedium)
				}
			}
		})
		
		if err != nil {
			return fmt.Errorf("failed to schedule digest: %w", err)
		}
		
		Logger().Printf("Scheduled digest generation with cron: %s", cfg.DigestCronSchedule)
	}
	
	return nil
}

// GenerateDigest generates a news digest
func (dm *DigestManager) GenerateDigest(session *discordgo.Session, channelID string, settings *DigestSettings) error {
	// Basic implementation
	sources, err := LoadSources()
	if err != nil {
		return fmt.Errorf("failed to load sources: %w", err)
	}
	
	// Filter active sources
	var activeSources []Source
	for _, source := range sources {
		if !source.Paused {
			activeSources = append(activeSources, source)
		}
	}
	
	// Create digest message
	
