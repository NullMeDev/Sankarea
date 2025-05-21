package main

import (
    "encoding/json"
    "io/ioutil"
    "log"
    "os"

    "gopkg.in/yaml.v2"
)

const (
    ConfigFilePath  = "config/config.json"
    SourcesFilePath = "config/sources.yml"
    StateFilePath   = "data/state.json"
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
}

type State struct {
    Paused        bool   `json:"paused"`
    LastDigest    int64  `json:"lastDigest"`
    LastInterval  int    `json:"lastInterval"`
    LastError     string `json:"lastError"`
    NewsNextTime  int64  `json:"newsNextTime"`
    FeedCount     int    `json:"feedCount"`
    Lockdown      bool   `json:"lockdown"`
    LockdownSetBy string `json:"lockdownSetBy"`
}

// Load sources from YAML file
func LoadSources() ([]Source, error) {
    b, err := ioutil.ReadFile(SourcesFilePath)
    if err != nil {
        return nil, err
    }
    var sources []Source
    if err := yaml.Unmarshal(b, &sources); err != nil {
        return nil, err
    }
    return sources, nil
}

// Save sources to YAML file
func SaveSources(sources []Source) error {
    b, err := yaml.Marshal(sources)
    if err != nil {
        return err
    }
    return ioutil.WriteFile(SourcesFilePath, b, 0644)
}

// Load config from JSON file
func LoadConfig() (Config, error) {
    b, err := ioutil.ReadFile(ConfigFilePath)
    if err != nil {
        return Config{}, err
    }
    var config Config
    if err := json.Unmarshal(b, &config); err != nil {
        return Config{}, err
    }
    return config, nil
}

// Save config to JSON file
func SaveConfig(config Config) error {
    b, err := json.MarshalIndent(config, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(ConfigFilePath, b, 0644)
}

// Load state from JSON file
func LoadState() (State, error) {
    b, err := ioutil.ReadFile(StateFilePath)
    if err != nil {
        return State{}, err
    }
    var state State
    if err := json.Unmarshal(b, &state); err != nil {
        return State{}, err
    }
    return state, nil
}

// Save state to JSON file
func SaveState(state State) error {
    b, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(StateFilePath, b, 0644)
}

// Get environment variable or fail fatally
func GetEnvOrFail(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("Missing required environment variable: %s", key)
    }
    return v
}

// Ensure required files exist before starting
func EnsureRequiredFiles() {
    required := []string{ConfigFilePath, SourcesFilePath, StateFilePath}
    for _, f := range required {
        if _, err := os.Stat(f); os.IsNotExist(err) {
            log.Fatalf("Required file missing: %s", f)
        }
    }
}
