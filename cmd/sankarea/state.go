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
	Paused        bool      `json:"paused"`
	FeedCount     int       `json:"feedCount"`
	DigestCount   int       `json:"digestCount"`
	ErrorCount    int       `json:"errorCount"`
	StartupTime   time.Time `json:"startupTime"`
	NewsNextTime  time.Time `json:"newsNextTime"`
	DigestNextTime time.Time `json:"digestNextTime"`
	Version       string    `json:"version"`
	LastInterval  int       `json:"lastInterval"`
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

	// Convert state to JSON
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Ensure the data directory exists
	if err := EnsureDataDir(); err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(stateFilePath, data, 0644)
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	return os.MkdirAll("data", 0755)
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
}

// SetPaused sets the paused state
func SetPaused(paused bool) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.Paused = paused
}

// IsPaused returns whether the system is paused
func IsPaused() bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	return state.Paused
}

// GetUptime returns the current uptime of the application
func GetUptime() time.Duration {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	return time.Since(state.StartupTime)
}
