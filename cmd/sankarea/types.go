package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	// Implementation omitted for brevity
	// Should scan text for keywords and update stats
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
	// Implementation omitted for brevity
	// Should generate a digest and send it to the specified channel
	
	// Record digest generation in state
	RecordDigestGeneration()
	
	return nil
}

// LanguageManager handles multi-language support
type LanguageManager struct {
	translations map[string]map[string]string
	langFile     string
	mutex        sync.Mutex
}

// NewLanguageManager creates a new language manager
func NewLanguageManager() *LanguageManager {
	return &LanguageManager{
		translations: make(map[string]map[string]string),
		langFile:     "data/translations.json",
	}
}

// Initialize sets up the language manager
func (lm *LanguageManager) Initialize() error {
	// Create the data directory if it doesn't exist
	dir := filepath.Dir(lm.langFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Check if translations file exists
	if _, err := os.Stat(lm.langFile); os.IsNotExist(err) {
		// Create default translations
		lm.translations["en"] = map[string]string{
			"welcome": "Welcome to Sankarea!",
			"help":    "Use /help for more information",
		}
		
		// Save translations
		return lm.Save()
	}
	
	// Load existing translations
	data, err := os.ReadFile(lm.langFile)
	if err != nil {
		return err
	}
	
	if len(data) == 0 {
		// File exists but is empty
		return nil
	}
	
	// Parse translations
	if err := json.Unmarshal(data, &lm.translations); err != nil {
		return err
	}
	
	return nil
}

// Save persists the translations to disk
func (lm *LanguageManager) Save() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	
	// Marshal translations to JSON
	data, err := json.MarshalIndent(lm.translations, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file atomically to prevent corruption
	tempFile := lm.langFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	
	// Then rename to the actual file
	return os.Rename(tempFile, lm.langFile)
}

// CredibilityScorer scores news sources on credibility
type CredibilityScorer struct {
	scores    map[string]float64
	scoresFile string
	mutex     sync.Mutex
}

// NewCredibilityScorer creates a new credibility scorer
func NewCredibilityScorer() *CredibilityScorer {
	return &CredibilityScorer{
		scores:     make(map[string]float64),
		scoresFile: "data/credibility.json",
	}
}

// Initialize sets up the credibility scorer
func (cs *CredibilityScorer) Initialize() error {
	// Create the data directory if it doesn't exist
	dir := filepath.Dir(cs.scoresFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Check if scores file exists
	if _, err := os.Stat(cs.scoresFile); os.IsNotExist(err) {
		// Create empty scores file
		return cs.Save()
	}
	
	// Load existing scores
	data, err := os.ReadFile(cs.scoresFile)
	if err != nil {
		return err
	}
	
	if len(data) == 0 {
		// File exists but is empty
		return nil
	}
	
	// Parse scores
	if err := json.Unmarshal(data, &cs.scores); err != nil {
		return err
	}
	
	return nil
}

// Save persists the scores to disk
func (cs *CredibilityScorer) Save() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	// Marshal scores to JSON
	data, err := json.MarshalIndent(cs.scores, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file atomically to prevent corruption
	tempFile := cs.scoresFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	
	// Then rename to the actual file
	return os.Rename(tempFile, cs.scoresFile)
}

// AnalyticsEngine tracks usage statistics
type AnalyticsEngine struct {
	data      map[string]interface{}
	dataFile  string
	mutex     sync.Mutex
}

// NewAnalyticsEngine creates a new analytics engine
func NewAnalyticsEngine() *AnalyticsEngine {
	return &AnalyticsEngine{
		data:     make(map[string]interface{}),
		dataFile: "data/analytics/analytics.json",
	}
}

// Initialize sets up the analytics engine
func (ae *AnalyticsEngine) Initialize() error {
	// Create the data directory if it doesn't exist
	dir := filepath.Dir(ae.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Check if data file exists
	if _, err := os.Stat(ae.dataFile); os.IsNotExist(err) {
		// Create empty data file
		return ae.Save()
	}
	
	// Load existing data
	data, err := os.ReadFile(ae.dataFile)
	if err != nil {
		return err
	}
	
	if len(data) == 0 {
		// File exists but is empty
		return nil
	}
	
	// Parse data
	if err := json.Unmarshal(data, &ae.data); err != nil {
		return err
	}
	
	return nil
}

// Save persists the analytics data to disk
func (ae *AnalyticsEngine) Save() error {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()
	
	// Marshal data to JSON
	data, err := json.MarshalIndent(ae.data, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to file atomically to prevent corruption
	tempFile := ae.dataFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	
	// Then rename to the actual file
	return os.Rename(tempFile, ae.dataFile)
}

// Logger returns a logger for the application
func Logger() *log.Logger {
	// Use a once to initialize the logger
	loggerOnce.Do(func() {
		if appLogger == nil {
			appLogger = log.New(os.Stdout, "SANKAREA: ", log.LstdFlags)
		}
	})
	
	return appLogger
}

// Global logger
var (
	appLogger  *log.Logger
	loggerOnce sync.Once
)

// FileMustExist ensures a file exists or creates it with default content
func FileMustExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Ensure directory exists
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		
		// Create empty file
		var defaultContent []byte
		
		// Set default content based on file extension
		switch filepath.Ext(path) {
		case ".json":
			defaultContent = []byte("{}")
		case ".yml", ".yaml":
			defaultContent = []byte("---\n")
		default:
			defaultContent = []byte("")
		}
		
		if err := os.WriteFile(path, defaultContent, 0644); err != nil {
			log.Fatalf("Failed to create file %s: %v", path, err)
		}
	}
}

// LoadEnv loads environment variables from .env file
func LoadEnv() {
	// Check if .env file exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		// No .env file, nothing to do
		return
	}
	
	// Read .env file
	data, err := os.ReadFile(".env")
	if err != nil {
		log.Printf("Warning: Could not read .env file: %v", err)
		return
	}
	
	// Parse .env file
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			// Skip empty lines and comments
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Invalid line format
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove surrounding quotes if present
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[0] == value[len(value)-1] {
			value = value[1 : len(value)-1]
		}
		
		// Set environment variable
		os.Setenv(key, value)
	}
}

// SetupLogging configures logging to file and console
func SetupLogging() error {
	// Ensure logs directory exists
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}
	
	// Open log file
	logFile := fmt.Sprintf("logs/sankarea_%s.log", time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Create multi-writer for console and file
	mw := io.MultiWriter(os.Stdout, f)
	
	// Set up logger
	loggerOnce.Do(func() {
		appLogger = log.New(mw, "SANKAREA: ", log.LstdFlags)
	})
	
	return nil
}

// InitDB initializes the database connection
func InitDB() error {
	// Implementation omitted for brevity
	// Should initialize the database connection and create tables if needed
	return nil
}

// StartHealthServer starts the health API server
func StartHealthServer(port int) {
	// Implementation omitted for brevity
	// Should start an HTTP server to expose health metrics
	go func() {
		defer RecoverFromPanic("health-server")
		
		Logger().Printf("Starting health API server on port %d", port)
		
		// Simple implementation would be:
		// http.HandleFunc("/health", handleHealth)
		// http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}()
}

// StartDashboard starts the web dashboard
func StartDashboard() error {
	// Implementation omitted for brevity
	// Should start the web dashboard for monitoring and configuration
	
	go func() {
		defer RecoverFromPanic("dashboard")
		
		Logger().Println("Starting dashboard...")
		
		// Dashboard implementation would go here
	}()
	
	return nil
}
