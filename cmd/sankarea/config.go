package main

import (
	"encoding/json"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	configFilePath  = "config/config.json"
	sourcesFilePath = "config/sources.yml"
	stateFilePath   = "data/state.json"
)

type Source struct {
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Paused       bool      `json:"paused"`
	LastDigest   time.Time `json:"lastDigest"`
	LastInterval int       `json:"lastInterval"`
	LastError    string    `json:"lastError"`
	NewsNextTime time.Time `json:"newsNextTime"`
	FeedCount    int       `json:"feedCount"`
	Lockdown     bool      `json:"lockdown"`
	LockdownSetBy string   `json:"lockdownSetBy"`
	Version      string    `json:"version"`
	StartupTime  time.Time `json:"startupTime"`
	ErrorCount   int       `json:"errorCount"`
}

type Config struct {
	BotToken          string `json:"bot_token"`
	AppID             string `json:"app_id"`
	GuildID           string `json:"guild_id"`
	News15MinCron     string `json:"news15MinCron"`
	AuditLogChannelID string `json:"auditLogChannelId"`
	NewsDigestCron    string `json:"newsDigestCron"`
	MaxPostsPerSource int    `json:"maxPostsPerSource"`
	Version           string `json:"version"`
}

type State struct {
	Paused       bool      `json:"paused"`
	LastDigest   time.Time `json:"lastDigest"`
	LastInterval int       `json:"lastInterval"`
	LastError    string    `json:"lastError"`
	NewsNextTime time.Time `json:"newsNextTime"`
	FeedCount    int       `json:"feedCount"`
	Lockdown     bool      `json:"lockdown"`
	LockdownSetBy string   `json:"lockdownSetBy"`
	Version      string    `json:"version"`
	StartupTime  time.Time `json:"startupTime"`
	ErrorCount   int       `json:"errorCount"`
}

func LoadConfig() (*Config, error) {
	if err := os.MkdirAll("config", 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// write a skeleton default
		defaultCfg := &Config{
			News15MinCron:     "*/15 * * * *",
			NewsDigestCron:    "0 8 * * *",
			MaxPostsPerSource: 5,
			Version:           "0.1.0",
		}
		if err := SaveConfig(defaultCfg); err != nil {
			return nil, err
		}
		return defaultCfg, nil
	}
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll("config", 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath, data, 0644)
}

func LoadSources() ([]Source, error) {
	if err := os.MkdirAll("config", 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(sourcesFilePath); os.IsNotExist(err) {
		// start empty
		if err := SaveSources([]Source{}); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(sourcesFilePath)
	if err != nil {
		return nil, err
	}
	var srcs []Source
	if err := yaml.Unmarshal(data, &srcs); err != nil {
		return nil, err
	}
	return srcs, nil
}

func SaveSources(srcs []Source) error {
	if err := os.MkdirAll("config", 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(srcs)
	if err != nil {
		return err
	}
	return os.WriteFile(sourcesFilePath, data, 0644)
}

func LoadState() (*State, error) {
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		defaultState := &State{
			StartupTime: time.Now(),
			Version:     "0.1.0",
		}
		if err := SaveState(defaultState); err != nil {
			return nil, err
		}
		return defaultState, nil
	}
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func SaveState(st *State) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath, data, 0644)
}
