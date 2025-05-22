package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Global application state
var state = &State{
	Paused:      false,
	FeedCount:   0,
	DigestCount: 0,
	ErrorCount:  0,
	StartupTime: time.Now(),
	Version:     VERSION,
}

var stateMutex sync.Mutex

// State represents the application runtime state
type State struct {
	Paused          bool      `json:"paused"`
	FeedCount       int       `json:"feedCount"`
	DigestCount     int       `json:"digestCount"`
	ErrorCount      int       `json:"errorCount"`
	StartupTime     time.Time `json:"startupTime"`
	ShutdownTime    time.Time `json:"shutdownTime"`
	LastFetchTime   time.Time `json:"lastFetchTime"`
	NewsNextTime    time.Time `json:"newsNextTime"`
	DigestNextTime  time.Time `json:"digestNextTime"`
	Version         string    `json:"version"`
	LastInterval    int       `json:"lastInterval"`
	TotalArticles   int       `json:"totalArticles"`
	LastDigest      time.Time `json:"lastDigest"`
	APIRequestCount int       `json:"apiRequestCount"`
	TotalErrors     int       `json:"totalErrors"`
	LastError       string    `json:"lastError"`
	LastErrorTime   time.Time `json:"lastErrorTime"`
	UptimeSeconds   int64     `json:"uptimeSeconds"`
	SystemStatus    string    `json:"systemStatus"`
	ActiveLanguages []string  `json:"activeLanguages"`
}

// LoadState loads the application state from file
func LoadState() (*State, error) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// If file doesn't exist, create it with default state
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		if err := SaveState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	// Read and parse the state file
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		// File exists but is empty, use default state
		if err := SaveState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	// Parse the state
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}

	return state, nil
}

// SaveState saves the application state to file
func SaveState(s *State) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Copy the state to avoid data race
	state = s

	// Calculate uptime before saving
	state.UptimeSeconds = time.Since(state.StartupTime).Milliseconds() / 1000

	// Convert state to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Ensure the data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(stateFilePath, data, 0644)
}

// UpdateNewsNextTime updates the time for the next news fetch
func UpdateNewsNextTime(minutes int) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.NewsNextTime = time.Now().Add(time.Duration(minutes) * time.Minute)
	state.LastInterval = minutes
}

// UpdateDigestNextTime updates the time for the next digest
func UpdateDigestNextTime(nextTime time.Time) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.DigestNextTime = nextTime
}

// IncrementFeedCount increments the feed counter
func IncrementFeedCount() {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.FeedCount++
}

// IncrementDigestCount increments the digest counter
func IncrementDigestCount() {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.DigestCount++
}

// IncrementErrorCount increments the error counter
func IncrementErrorCount() {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.ErrorCount++
	state.TotalErrors++
}

// SetPaused sets the paused state
func SetPaused(paused bool) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.Paused = paused
	
	// Update system status
	if paused {
		state.SystemStatus = "paused"
	} else {
		state.SystemStatus = "running"
	}
}

// GetPaused gets the current paused state
func GetPaused() bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	return state.Paused
}

// IncrementAPIRequestCount increments the API request counter
func IncrementAPIRequestCount() {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	state.APIRequestCount++
}

// RecordError records an error in the state
func RecordError(errorMsg string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	state.LastError = errorMsg
	state.LastErrorTime = time.Now()
	state.ErrorCount++
	state.TotalErrors++
}

// UpdateTotalArticles updates the total articles count
func UpdateTotalArticles(count int) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	state.TotalArticles += count
}

// RecordDigestGeneration records that a digest was generated
func RecordDigestGeneration() {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	state.LastDigest = time.Now()
	state.DigestCount++
}

// AddActiveLanguage adds a language to the active languages list
func AddActiveLanguage(lang string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	// Check if language is already in the list
	for _, l := range state.ActiveLanguages {
		if l == lang {
			return
		}
	}
	
	state.ActiveLanguages = append(state.ActiveLanguages, lang)
}

// GetSystemStatus gets a detailed system status
func GetSystemStatus() map[string]interface{} {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	status := map[string]interface{}{
		"status":         state.SystemStatus,
		"version":        state.Version,
		"uptime_seconds": state.UptimeSeconds,
		"paused":         state.Paused,
		"feed_count":     state.FeedCount,
		"digest_count":   state.DigestCount,
		"error_count":    state.ErrorCount,
		"total_articles": state.TotalArticles,
		"startup_time":   state.StartupTime,
		"last_fetch":     state.LastFetchTime,
		"next_news":      state.NewsNextTime,
		"next_digest":    state.DigestNextTime,
	}
	
	return status
}
