package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// State file path
const stateFilePath = "data/state.json"

var (
	state      = State{}
	stateMutex sync.Mutex
)

// LoadState loads the application state from disk
func LoadState() (*State, error) {
	// Create state file if it doesn't exist
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		// Create default state
		defaultState := &State{
			Version:     VERSION,
			StartupTime: time.Now(),
		}
		
		// Create directory if needed
		if err := os.MkdirAll("data", 0755); err != nil {
			return defaultState, err
		}
		
		// Save default state
		if err := SaveState(defaultState); err != nil {
			return defaultState, err
		}
		
		return defaultState, nil
	}
	
	// Read existing state file
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}
	
	// Parse state
	var loadedState State
	if err := json.Unmarshal(data, &loadedState); err != nil {
		return nil, err
	}
	
	// Update global state
	stateMutex.Lock()
	state = loadedState
	stateMutex.Unlock()
	
	return &loadedState, nil
}

// SaveState saves the application state to disk
func SaveState(s *State) error {
	// Update global state
	if s != nil {
		stateMutex.Lock()
		state = *s
		stateMutex.Unlock()
	}
	
	// Marshal state to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	// Save state
	return os.WriteFile(stateFilePath, data, 0644)
}

// GetState returns a copy of the current state (thread-safe)
func GetState() State {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	return state
}

// GetSystemStatus returns a map of key status metrics
func GetSystemStatus() map[string]interface{} {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	status := map[string]interface{}{
		"status":          getStatusText(state.Paused),
		"version":         VERSION,
		"uptime_seconds":  int64(time.Since(state.StartupTime).Seconds()),
		"feed_count":      state.FeedCount,
		"digest_count":    state.DigestCount,
		"error_count":     state.ErrorCount,
		"total_articles":  state.TotalArticles,
		"total_errors":    state.TotalErrors,
		"total_api_calls": state.TotalAPICalls,
	}
	
	return status
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

// UpdateTotalArticles updates the total articles counter
func UpdateTotalArticles(count int) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state.TotalArticles += count
}

// getStatusText returns a text representation of pause status
func getStatusText(paused bool) string {
	if paused {
		return "paused"
	}
	return "running"
}
