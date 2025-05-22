package main

import (
    "encoding/json"
    "io/ioutil"
    "time"
    "gopkg.in/yaml.v2"
)

const (
    configFilePath  = "config/config.json"
    sourcesFilePath = "config/sources.yml"
    stateFilePath   = "data/state.json"
)

type Source struct {
    Name   string \`yaml:"name"\`
    URL    string \`yaml:"url"\`
    Bias   string \`yaml:"bias"\`
    Active bool   \`yaml:"active"\`
}

type Config struct {
    News15MinCron     string \`json:"news15MinCron"\`
    AuditLogChannelID string \`json:"auditLogChannelId"\`
    NewsDigestCron    string \`json:"newsDigestCron"\`
    MaxPostsPerSource int    \`json:"maxPostsPerSource"\`
    Version           string \`json:"version"\`
}

type State struct {
    Paused        bool      \`json:"paused"\`
    LastDigest    time.Time \`json:"lastDigest"\`
    LastInterval  int       \`json:"lastInterval"\`
    LastError     string    \`json:"lastError"\`
    NewsNextTime  time.Time \`json:"newsNextTime"\`
    FeedCount     int       \`json:"feedCount"\`
    Lockdown      bool      \`json:"lockdown"\`
    LockdownSetBy string    \`json:"lockdownSetBy"\`
    Version       string    \`json:"version"\`
    StartupTime   time.Time \`json:"startupTime"\`
    ErrorCount    int       \`json:"errorCount"\`
}

// TODO: Implement LoadSources, SaveSources, LoadConfig, SaveConfig, LoadState, SaveState
