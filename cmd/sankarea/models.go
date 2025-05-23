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
}

// ArticleDigest represents a summarized article for digests
type ArticleDigest struct {
    Title     string    `json:"title"`
    URL       string    `json:"url"`
    Source    string    `json:"source"`
    Published time.Time `json:"published"`
    Category  string    `json:"category"`
    Bias      string    `json:"bias"`
}

// SentimentAnalysis contains sentiment analysis results
type SentimentAnalysis struct {
    Sentiment     string         `json:"sentiment"`
    Score         float64        `json:"score"`
    Topics        []string       `json:"topics"`
    Keywords      []string       `json:"keywords"`
    EntityCount   map[string]int `json:"entity_count"`
    IsOpinionated bool          `json:"is_opinionated"`
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
    OpenAIAPIKey         string   `json:"openAiApiKey"`
    GoogleFactCheckAPIKey string  `json:"googleFactCheckApiKey"`
    ClaimBustersAPIKey   string  `json:"claimBustersApiKey"`
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

// ErrorEvent represents an error event
type ErrorEvent struct {
    Time      time.Time `json:"time"`
    Component string    `json:"component"`
    Message   string    `json:"message"`
    Severity  string    `json:"severity"`
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

// ErrorBuffer represents a circular buffer of error events
type ErrorBuffer struct {
    events []*ErrorEvent
    size   int
    mutex  sync.RWMutex
}

// Permission levels for command access
const (
    PermLevelEveryone = iota
    PermLevelAdmin
    PermLevelOwner
)

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

    var state State
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }

    return &state, nil
}

// SaveState saves the application state to disk
func SaveState(state *State) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile("data/state.json", data, 0644)
}
