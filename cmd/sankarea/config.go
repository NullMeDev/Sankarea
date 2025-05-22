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
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Bias   string `yaml:"bias"`
    Active bool   `yaml:"active"`
}

type Config struct {
    News15MinCron     string `json:"news15MinCron"`
    AuditLogChannelID string `json:"auditLogChannelId"`
    NewsDigestCron    string `json:"newsDigestCron"`
    MaxPostsPerSource int    `json:"maxPostsPerSource"`
    Version           string `json:"version"`
}

type State struct {
    Paused        bool      `json:"paused"`
    LastDigest    time.Time `json:"lastDigest"`
    LastInterval  int       `json:"lastInterval"`
    LastError     string    `json:"lastError"`
    NewsNextTime  time.Time `json:"newsNextTime"`
    FeedCount     int       `json:"feedCount"`
    Lockdown      bool      `json:"lockdown"`
    LockdownSetBy string    `json:"lockdownSetBy"`
    Version       string    `json:"version"`
    StartupTime   time.Time `json:"startupTime"`
    ErrorCount    int       `json:"errorCount"`
}

func LoadSources() ([]Source, error) {
    b, err := ioutil.ReadFile(sourcesFilePath)
    if err != nil {
        return nil, err
    }
    var sources []Source
    if err := yaml.Unmarshal(b, &sources); err != nil {
        return nil, err
    }
    return sources, nil
}

func SaveSources(sources []Source) error {
    b, err := yaml.Marshal(sources)
    if err != nil {
        return err
    }
    return ioutil.WriteFile(sourcesFilePath, b, 0644)
}

func LoadConfig() (Config, error) {
    b, err := ioutil.ReadFile(configFilePath)
    if err != nil {
        return Config{}, err
    }
    var cfg Config
    if err := json.Unmarshal(b, &cfg); err != nil {
        return Config{}, err
    }
    return cfg, nil
}

func SaveConfig(cfg Config) error {
    b, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(configFilePath, b, 0644)
}

func LoadState() (State, error) {
    b, err := ioutil.ReadFile(stateFilePath)
    if err != nil {
        return State{}, err
    }
    var st State
    if err := json.Unmarshal(b, &st); err != nil {
        return State{}, err
    }
    return st, nil
}

func SaveState(st State) error {
    b, err := json.MarshalIndent(st, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(stateFilePath, b, 0644)
}
