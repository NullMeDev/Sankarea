// cmd/sankarea/models.go
package main

import (
    "encoding/json"
    "os"
    "sync"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Article represents parsed article content
type Article struct {
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    URL       string    `json:"url"`
    Source    string    `json:"source"`
    Timestamp time.Time `json:"timestamp"`
    Category  string    `json:"category"`
    Sentiment float64   `json:"sentiment"`
    FactScore float64   `json:"factScore"`
    Summary   string    `json:"summary"`
    Bias      string    `json:"bias"`
    Topics    []string  `json:"topics"`
    Keywords  []string  `json:"keywords"`
}

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
    Lockdown      bool      `json:"lockdown"`
    LockdownSetBy string    `json:"lockdownSetBy"`
}

// Metrics represents system metrics
type Metrics struct {
    MemoryUsageMB     float64 `json:"memoryUsageMb"`
    CPUUsagePercent   float64 `json:"cpuUsagePercent"`
    DiskUsagePercent  float64 `json:"diskUsagePercent"`
    UptimeSeconds     int64   `json:"uptimeSeconds"`
    ArticlesPerMinute float64 `json:"articlesPerMinute"`
    ErrorsPerHour     float64 `json:"errorsPerHour"`
    APICallsPerHour   float64 `json:"apiCallsPerHour"`
}

// ErrorEvent represents an error event
type ErrorEvent struct {
    Time      time.Time `json:"time"`
    Component string    `json:"component"`
    Message   string    `json:"message"`
    Severity  string    `json:"severity"`
}

// ErrorBuffer represents a circular buffer of error events
type ErrorBuffer struct {
    events []*ErrorEvent
    size   int
    mutex  sync.RWMutex
}

// Global state management
var (
    state *State
    mutex sync.RWMutex
)

// GetState returns a copy of the current state
func GetState() *State {
    mutex.RLock()
    defer mutex.RUnlock()
    stateCopy := *state
    return &stateCopy
}

// UpdateState safely updates the state
func UpdateState(updater func(*State)) error {
    mutex.Lock()
    defer mutex.Unlock()
    updater(state)
    return SaveState(state)
}

// LoadState loads the application state from disk
func LoadState() (*State, error) {
    data, err := os.ReadFile("data/state.json")
    if err != nil {
        if os.IsNotExist(err) {
            return &State{
                StartupTime: time.Now(),
                Version:    cfg.Version,
            }, nil
        }
        return nil, err
    }

    var newState State
    if err := json.Unmarshal(data, &newState); err != nil {
        return nil, err
    }

    mutex.Lock()
    state = &newState
    mutex.Unlock()

    return state, nil
}

// SaveState saves the application state to disk
func SaveState(state *State) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile("data/state.json", data, 0644)
}

// NewErrorBuffer creates a new error buffer with specified size
func NewErrorBuffer(size int) *ErrorBuffer {
    return &ErrorBuffer{
        events: make([]*ErrorEvent, 0, size),
        size:   size,
    }
}

// Add adds a new error event to the buffer
func (eb *ErrorBuffer) Add(event *ErrorEvent) {
    eb.mutex.Lock()
    defer eb.mutex.Unlock()

    if len(eb.events) >= eb.size {
        eb.events = eb.events[1:]
    }
    eb.events = append(eb.events, event)
}

// GetRecent returns the most recent error events
func (eb *ErrorBuffer) GetRecent(count int) []*ErrorEvent {
    eb.mutex.RLock()
    defer eb.mutex.RUnlock()

    if count > len(eb.events) {
        count = len(eb.events)
    }

    result := make([]*ErrorEvent, count)
    copy(result, eb.events[len(eb.events)-count:])
    return result
}
