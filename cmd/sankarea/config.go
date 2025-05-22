package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
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
	Paused        bool   `json:"paused"`
	LastDigest    string `json:"lastDigest"`
	LastInterval  int    `json:"lastInterval"`
	LastError     string `json:"lastError"`
	NewsNextTime  string `json:"newsNextTime"`
	FeedCount     int    `json:"feedCount"`
	Lockdown      bool   `json:"lockdown"`
	LockdownSetBy string `json:"lockdownSetBy"`
	Version       string `json:"version"`
	StartupTime   string `json:"startupTime"`
	ErrorCount    int    `json:"errorCount"`
}

const (
	configFilePath  = "config/config.json"
	sourcesFilePath = "config/sources.yml"
	stateFilePath   = "data/state.json"
)

func loadEnv() {
	if _, err := os.Stat(".env"); err == nil {
		file, err := os.Open(".env")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if os.Getenv(key) == "" {
					os.Setenv(key, value)
				}
			}
		}
	}
}

func fileMustExist(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("ERROR: Required file not found: %s", path)
	}
}

func loadSources() ([]Source, error) {
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

func saveSources(sources []Source) error {
	b, err := yaml.Marshal(sources)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(sourcesFilePath, b, 0644)
}

func loadConfig() (Config, error) {
	b, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(b, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func saveConfig(config Config) error {
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFilePath, b, 0644)
}

func loadState() (State, error) {
	b, err := ioutil.ReadFile(stateFilePath)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(b, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func saveState(state State) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(stateFilePath, b, 0644)
}

func getEnvOrFail(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Missing required environment variable: %s", key)
	}
	return v
}

func getEnvOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}
